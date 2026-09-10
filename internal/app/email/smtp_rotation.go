package email

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/warmbly/warmbly/internal/config"
	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/repository"
	"github.com/warmbly/warmbly/internal/sendrouting"
)

// pendingDrain is a mailbox that has moved and whose old worker still holds
// it. The REMOVE_EMAIL goes out once the drain window has passed, so a send
// dispatched to the old worker right before the switch is answered first.
type pendingDrain struct {
	oldWorkerID uuid.UUID
	userID      string
	movedAt     time.Time
}

// StartSmtpRotationReconciler keeps every active SMTP/IMAP mailbox on a live
// ephemeral worker and rotates it between them. Each pass:
//
//  1. sends the deferred REMOVE_EMAIL for moves older than the drain window
//  2. re-places a mailbox whose worker is gone right away, instead of waiting
//     for the worker reconciler's republish window
//  3. asks the assignment policy whether each remaining mailbox should move
//     (off a persistent worker while an ephemeral one is live, or off an
//     ephemeral one after SMTP_ROTATE_AFTER_SECONDS) and moves it unless a
//     send tick for it is running at that moment
//
// A move loads the mailbox on the new worker before the assignment switches,
// so a send routed to the new worker finds it, and the old worker keeps it
// through the drain so a send already routed there completes and answers.
// Nothing here touches gmail or outlook mailboxes: the candidate query only
// returns smtp_imap rows.
func (s *emailService) StartSmtpRotationReconciler(ctx context.Context, interval time.Duration) {
	if s.workerAssignment == nil || s.publisher == nil {
		return
	}
	drains := map[uuid.UUID]pendingDrain{}
	s.rotateSmtpMailboxes(ctx, drains)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.rotateSmtpMailboxes(ctx, drains)
		}
	}
}

func (s *emailService) rotateSmtpMailboxes(ctx context.Context, drains map[uuid.UUID]pendingDrain) {
	s.flushSmtpDrains(ctx, drains)

	cands, err := s.workerAssignment.ListSmtpRotationCandidates(ctx)
	if err != nil {
		log.Warn().Err(err).Msg("smtp rotation: list candidates failed")
		return
	}
	now := time.Now()
	rotateAfter := time.Duration(config.SmtpRotateAfterSeconds()) * time.Second
	moved, replaced := 0, 0
	for _, c := range cands {
		if ctx.Err() != nil {
			return
		}
		if _, draining := drains[c.AccountID]; draining {
			// The last move has not drained yet; let it settle first.
			continue
		}
		if c.WorkerID == nil || !c.WorkerLive {
			// Stranded: the load path releases the dead worker and places the
			// mailbox again, preferring an ephemeral worker.
			if err := s.LoadAccountOntoWorker(ctx, c.AccountID); err != nil {
				log.Warn().Err(err).Str("email_id", c.AccountID.String()).Msg("smtp rotation: re-placing a stranded mailbox failed")
				continue
			}
			replaced++
			continue
		}
		// Only SMTP/IMAP mailboxes rotate; anything else was listed solely
		// because its worker was gone, which is handled above.
		if c.Provider != "smtp_imap" {
			continue
		}
		target, err := s.workerAssignment.PlanSmtpRotation(ctx, c, now, rotateAfter)
		if err != nil {
			log.Warn().Err(err).Str("email_id", c.AccountID.String()).Msg("smtp rotation: planning failed")
			continue
		}
		if target == nil || target.ID == *c.WorkerID {
			continue
		}
		busy, err := s.workerAssignment.HasInFlightSend(ctx, c.AccountID)
		if err != nil || busy {
			// A send may be on its way to the current worker; try next pass.
			continue
		}
		if err := s.moveSmtpMailbox(ctx, c, target.ID); err != nil {
			log.Warn().Err(err).
				Str("email_id", c.AccountID.String()).
				Str("from_worker", c.WorkerID.String()).
				Str("to_worker", target.ID.String()).
				Msg("smtp rotation: move failed")
			continue
		}
		drains[c.AccountID] = pendingDrain{oldWorkerID: *c.WorkerID, userID: c.UserID.String(), movedAt: now}
		moved++
	}
	if moved > 0 || replaced > 0 {
		log.Info().Int("moved", moved).Int("replaced", replaced).Int("candidates", len(cands)).Msg("smtp rotation: pass complete")
	}
}

// moveSmtpMailbox ships the mailbox to the new worker, then switches the
// assignment. The old worker is not told yet; see flushSmtpDrains.
func (s *emailService) moveSmtpMailbox(ctx context.Context, c repository.SmtpRotationCandidate, newWorkerID uuid.UUID) error {
	acc, xerr := s.emailRepository.GetByID(ctx, c.AccountID)
	if xerr != nil {
		return xerr
	}
	if acc == nil || acc.Status != "active" {
		return nil
	}
	payload, err := s.buildAddWorkerEmail(ctx, acc)
	if err != nil {
		return err
	}
	if payload == nil {
		return nil
	}
	if err := s.publisher.PublishAddEmail(ctx, newWorkerID, payload); err != nil {
		return err
	}
	if err := s.workerAssignment.MoveEmailToWorker(ctx, c.AccountID, *c.WorkerID, newWorkerID); err != nil {
		return err
	}
	log.Info().
		Str("email_id", c.AccountID.String()).
		Str("from_worker", c.WorkerID.String()).
		Str("from_deployment", string(c.WorkerDeployment)).
		Str("to_worker", newWorkerID.String()).
		Msg("smtp rotation: mailbox moved to a live ephemeral worker")
	return nil
}

// flushSmtpDrains tells old workers to drop mailboxes whose move is older
// than the drain window. A worker that is no longer live is skipped: there
// is nothing to tell, and an ephemeral one is gone with its memory anyway.
func (s *emailService) flushSmtpDrains(ctx context.Context, drains map[uuid.UUID]pendingDrain) {
	drain := time.Duration(config.SmtpMoveDrainSeconds) * time.Second
	for accountID, d := range drains {
		if time.Since(d.movedAt) < drain {
			continue
		}
		delete(drains, accountID)
		// The old worker is about to drop the mailbox, so send-side routing
		// must hand it the credentials again before routing a send there.
		if s.r != nil {
			_ = s.r.Del(ctx, sendrouting.LoadedOnKey(accountID, d.oldWorkerID)).Err()
		}
		live, err := s.workerAssignment.IsWorkerLive(ctx, d.oldWorkerID)
		if err != nil || !live {
			continue
		}
		if err := s.publisher.PublishRemoveEmail(ctx, d.oldWorkerID, &models.RemoveWorkerEmail{
			UserID:  d.userID,
			EmailID: accountID.String(),
		}); err != nil {
			log.Warn().Err(err).
				Str("email_id", accountID.String()).
				Str("worker_id", d.oldWorkerID.String()).
				Msg("smtp rotation: could not tell the old worker to drop the mailbox")
		}
	}
}
