package worker

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/warmbly/warmbly/internal/app/worker/mailmanager"
	"github.com/warmbly/warmbly/internal/app/worker/wmail"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/infrastructure/codec"
	"github.com/warmbly/warmbly/internal/infrastructure/eventbus"
	"github.com/warmbly/warmbly/internal/models"
)

// captureBus keeps every result the worker produces, decoded.
type captureBus struct {
	eventbus.EventBus

	mu      sync.Mutex
	results []models.JobEvent
}

func (b *captureBus) Publish(ctx context.Context, topic, key string, payload []byte) error {
	var ev models.JobEvent
	if err := codec.NewJSON().Deserialize(ctx, topic, payload, &ev); err != nil {
		return err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.results = append(b.results, ev)
	return nil
}

func (b *captureBus) failed() []models.SendEmailResult {
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []models.SendEmailResult
	for _, ev := range b.results {
		if ev.Type != models.JobEventTypeEmailFailed {
			continue
		}
		raw, _ := json.Marshal(ev.Body)
		var r models.SendEmailResult
		_ = json.Unmarshal(raw, &r)
		out = append(out, r)
	}
	return out
}

func newQueueWorker(t *testing.T) (*WorkerService, *captureBus) {
	t.Helper()
	bus := &captureBus{}
	w := &WorkerService{ID: "worker-1", Codec: codec.NewJSON(), Bus: bus}
	w.mailManager = mailmanager.NewMailManager(w.Produce, nil, nil, nil, nil, nil, nil)
	return w, bus
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// A send for a mailbox the worker does not hold is parked, not failed: the
// bus message is acked (nil) and nothing is produced until the wait runs out,
// at which point the reservation is resolved with EMAIL_FAILED naming this
// worker and the mailbox.
func TestSendForUnknownMailboxIsParkedThenFailed(t *testing.T) {
	old := sendLoadWait
	sendLoadWait = 150 * time.Millisecond
	defer func() { sendLoadWait = old }()

	w, bus := newQueueWorker(t)
	send := models.SendEmail{TaskID: uuid.New(), EmailID: uuid.New()}
	if err := w.HandleSendEmail(context.Background(), send); err != nil {
		t.Fatalf("HandleSendEmail: %v", err)
	}
	if got := bus.failed(); len(got) != 0 {
		t.Fatalf("failed the send at once: %+v", got)
	}
	if _, ok := w.sends.remove(send.EmailID, send.TaskID); !ok {
		t.Fatal("the send was not parked")
	}
	// Put it back so the timer finds it.
	w.sends.park(send)

	waitFor(t, "EMAIL_FAILED after the wait", func() bool { return len(bus.failed()) == 1 })
	got := bus.failed()[0]
	if got.TaskID != send.TaskID || got.EmailAccountID != send.EmailID.String() || got.WorkerID != "worker-1" {
		t.Fatalf("EMAIL_FAILED = %+v, want task %s, mailbox %s, worker worker-1", got, send.TaskID, send.EmailID)
	}
}

// A parked send released before the wait (the mailbox loaded) is not failed
// by the timer.
func TestReleasedParkedSendIsNotFailed(t *testing.T) {
	old := sendLoadWait
	sendLoadWait = 100 * time.Millisecond
	defer func() { sendLoadWait = old }()

	w, bus := newQueueWorker(t)
	send := models.SendEmail{TaskID: uuid.New(), EmailID: uuid.New()}
	w.parkSend(send)
	if taken := w.sends.take(send.EmailID); len(taken) != 1 || taken[0].TaskID != send.TaskID {
		t.Fatalf("take = %+v, want the parked send", taken)
	}
	time.Sleep(3 * sendLoadWait)
	if got := bus.failed(); len(got) != 0 {
		t.Fatalf("a released send was failed by the timer: %+v", got)
	}
}

// A send-only load the worker could not complete is remembered: sends parked
// for the mailbox and sends that arrive afterwards are answered at once with
// the load error's code, which is what makes the control plane route the
// mailbox elsewhere. The refused load itself is acked, not retried.
func TestRefusedSendOnlyLoadFailsParkedAndLaterSends(t *testing.T) {
	w, bus := newQueueWorker(t)
	mailbox := uuid.New()
	parked := models.SendEmail{TaskID: uuid.New(), EmailID: mailbox}
	w.sends.park(parked)

	// An smtp_imap payload without credentials is refused by NewWMail before
	// any network is touched, with an authentication code.
	err := w.HandleAddEmail(context.Background(), &models.AddWorkerEmail{
		ID:       mailbox,
		UserID:   uuid.New(),
		Email:    "sender@example.com",
		Type:     models.InboxProviderSMTPIMAP,
		SendOnly: true,
	})
	if err != nil {
		t.Fatalf("a refused send-only load must be acked, got %v", err)
	}
	if w.mailManager.Has(mailbox) {
		t.Fatal("a refused mailbox must not be loaded")
	}

	later := models.SendEmail{TaskID: uuid.New(), EmailID: mailbox}
	if err := w.HandleSendEmail(context.Background(), later); err != nil {
		t.Fatalf("HandleSendEmail: %v", err)
	}

	got := bus.failed()
	if len(got) != 2 {
		t.Fatalf("EMAIL_FAILED count = %d, want 2 (the parked send and the later one): %+v", len(got), got)
	}
	for _, r := range got {
		if r.Error == nil || r.Error.Code != string(errx.MailErrorCodeInvalidCredentials) {
			t.Fatalf("EMAIL_FAILED = %+v, want the load error's code %s", r, errx.MailErrorCodeInvalidCredentials)
		}
		if r.WorkerID != "worker-1" || r.EmailAccountID != mailbox.String() {
			t.Fatalf("EMAIL_FAILED = %+v, want worker-1 and mailbox %s stamped", r, mailbox)
		}
	}
	if _, ok := w.sends.remove(mailbox, parked.TaskID); ok {
		t.Fatal("the parked send was failed but left in the queue")
	}
}

// The owner's load of a mailbox the worker already holds send-only upgrades
// it: the send-only copy is dropped so the full load (with its sync loop) can
// take its place. Here the full load is refused too, which is enough to see
// the send-only copy go.
func TestOwnerLoadReplacesASendOnlyCopy(t *testing.T) {
	w, _ := newQueueWorker(t)
	mailbox := uuid.New()
	cancelled := false
	w.mailManager.Lock()
	w.mailManager.Emails[mailbox] = &wmail.WMail{ID: mailbox, SendOnly: true, Cancel: func() { cancelled = true }}
	w.mailManager.Unlock()

	_ = w.HandleAddEmail(context.Background(), &models.AddWorkerEmail{ID: mailbox, UserID: uuid.New(), Type: models.InboxProviderSMTPIMAP})
	if !cancelled {
		t.Fatal("the send-only copy's context was not cancelled")
	}
	if w.mailManager.Has(mailbox) {
		t.Fatal("the send-only copy is still loaded after the owner's load was refused")
	}
}
