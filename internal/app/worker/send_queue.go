package worker

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/sendrouting"
)

// A SEND_EMAIL can reach a worker before the ADD_EMAIL that carries the
// mailbox's credentials has been processed: send-side routing publishes the
// two back to back, and the bus hands this worker one message at a time, so
// a load that is being retried lets the send through first. Failing the send
// at once would spend one of the lead's attempts for nothing. Instead the
// send is parked (and its bus message acked), released to the send path the
// moment the mailbox loads, and answered with EMAIL_FAILED if the mailbox is
// still missing after sendLoadWait, so the control plane's outcome loop
// always resolves the reservation.

// sendLoadWait is how long a send waits for its mailbox to be loaded. A
// variable so tests can shorten it.
var sendLoadWait = 15 * time.Second

// refusedLoad is a send-only load this worker could not complete, kept so a
// send for that mailbox is answered at once with the real error instead of
// waiting out sendLoadWait. Forgotten after sendrouting.AuthFailureTTL, when
// the control plane may hand the mailbox to this worker again.
type refusedLoad struct {
	err error
	at  time.Time
}

type sendQueue struct {
	mu      sync.Mutex
	parked  map[uuid.UUID]map[uuid.UUID]models.SendEmail
	refused map[uuid.UUID]refusedLoad
}

func (q *sendQueue) park(send models.SendEmail) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.parked == nil {
		q.parked = map[uuid.UUID]map[uuid.UUID]models.SendEmail{}
	}
	byTask := q.parked[send.EmailID]
	if byTask == nil {
		byTask = map[uuid.UUID]models.SendEmail{}
		q.parked[send.EmailID] = byTask
	}
	byTask[send.TaskID] = send
}

// remove takes one parked send back; false when it was already released.
func (q *sendQueue) remove(mailboxID, taskID uuid.UUID) (models.SendEmail, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	byTask := q.parked[mailboxID]
	send, ok := byTask[taskID]
	if ok {
		delete(byTask, taskID)
		if len(byTask) == 0 {
			delete(q.parked, mailboxID)
		}
	}
	return send, ok
}

// take releases every send parked for the mailbox.
func (q *sendQueue) take(mailboxID uuid.UUID) []models.SendEmail {
	q.mu.Lock()
	defer q.mu.Unlock()
	byTask := q.parked[mailboxID]
	delete(q.parked, mailboxID)
	out := make([]models.SendEmail, 0, len(byTask))
	for _, s := range byTask {
		out = append(out, s)
	}
	return out
}

func (q *sendQueue) refuse(mailboxID uuid.UUID, err error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.refused == nil {
		q.refused = map[uuid.UUID]refusedLoad{}
	}
	q.refused[mailboxID] = refusedLoad{err: err, at: time.Now()}
}

func (q *sendQueue) clearRefusal(mailboxID uuid.UUID) {
	q.mu.Lock()
	defer q.mu.Unlock()
	delete(q.refused, mailboxID)
}

// refusal is the still-fresh load failure for the mailbox, if any.
func (q *sendQueue) refusal(mailboxID uuid.UUID) (error, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	r, ok := q.refused[mailboxID]
	if !ok {
		return nil, false
	}
	if time.Since(r.at) > sendrouting.AuthFailureTTL {
		delete(q.refused, mailboxID)
		return nil, false
	}
	return r.err, true
}

// parkSend holds a send for a mailbox this worker does not have yet. The
// timer fails it if nothing has released it by then.
func (w *WorkerService) parkSend(send models.SendEmail) {
	w.sends.park(send)
	log.Info().
		Str("task_id", send.TaskID.String()).
		Str("email_id", send.EmailID.String()).
		Dur("wait", sendLoadWait).
		Msg("mailbox not loaded on this worker yet; send parked until it is")
	time.AfterFunc(sendLoadWait, func() {
		if _, ok := w.sends.remove(send.EmailID, send.TaskID); !ok {
			return
		}
		log.Warn().
			Str("task_id", send.TaskID.String()).
			Str("email_id", send.EmailID.String()).
			Msg("mailbox was not loaded on this worker in time; send failed")
		w.sendEmailFailure(send.TaskID, send.EmailID, nil, "email account "+send.EmailID.String()+" was not loaded on the worker in time")
	})
}

// releaseParkedSends hands every send parked for a just-loaded mailbox to
// the send path. A parked send's bus message is already acked, so a
// background context stands in for the delivery's.
func (w *WorkerService) releaseParkedSends(mailboxID uuid.UUID) {
	for _, send := range w.sends.take(mailboxID) {
		go func(send models.SendEmail) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := w.HandleSendEmail(ctx, send); err != nil {
				log.Error().Err(err).Str("task_id", send.TaskID.String()).Msg("parked send failed after the mailbox loaded")
			}
		}(send)
	}
}

// failParkedSends answers every send parked for a mailbox whose load was
// refused, with the load error so the control plane sees the real cause (an
// authentication code makes it stop routing the mailbox here).
func (w *WorkerService) failParkedSends(mailboxID uuid.UUID, loadErr error) {
	for _, send := range w.sends.take(mailboxID) {
		w.failSendWithError(send, loadErr)
	}
}

// failSendWithError reports a send that could not be attempted because the
// mailbox could not be loaded, carrying the load error's code when it has one.
func (w *WorkerService) failSendWithError(send models.SendEmail, loadErr error) {
	var mailErr *errx.MailError
	if errors.As(loadErr, &mailErr) {
		w.produceSendFailed(send.TaskID, send.EmailID, mailErr)
		return
	}
	w.sendEmailFailure(send.TaskID, send.EmailID, nil, "email account "+send.EmailID.String()+" could not be loaded on the worker: "+loadErr.Error())
}
