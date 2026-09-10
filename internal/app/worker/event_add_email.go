package worker

import (
	"context"
	"errors"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/warmbly/warmbly/internal/app/worker/wmail"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
)

func (w *WorkerService) HandleAddEmail(ctx context.Context, e *models.AddWorkerEmail) error {
	if e == nil {
		return nil
	}

	if existing := w.mailManager.Get(e.ID); existing != nil {
		if existing.SendOnly && !e.SendOnly {
			// This worker becomes the sync owner of a mailbox it only sent
			// from: rebuild it as a full load so the sync loop starts.
			if existing.Cancel != nil {
				existing.Cancel()
			}
			w.mailManager.Terminate(e.ID)
		} else {
			// Already loaded: keep the handler idempotent, but take a changed
			// sync budget so an operator's settings change reaches this mailbox.
			existing.ApplySyncPolicy(e.Sync)
			return nil
		}
	}

	if err := w.mailManager.AddWMail(ctx, e); err != nil {
		if e.SendOnly {
			// Not this worker's mailbox to judge: the owner reports credential
			// problems. Remember the refusal so sends routed here are answered
			// at once, and answer the ones already waiting.
			log.Warn().Err(err).Str("email_id", e.ID.String()).Msg("send-only load refused; sends routed here are failed back to the control plane")
			w.sends.refuse(e.ID, err)
			w.failParkedSends(e.ID, err)
			return nil
		}
		log.Error().Err(err).Str("email_id", e.ID.String()).Msg("failed to add email account to worker")
		w.reportLoadAuthFailure(e, err)
		return err
	}
	w.sends.clearRefusal(e.ID)

	mail := w.mailManager.Get(e.ID)
	if mail == nil {
		return nil
	}
	if e.SendOnly {
		log.Info().Str("email_id", e.ID.String()).Str("email", e.Email).Msg("email account loaded on worker for sending only")
		w.releaseParkedSends(e.ID)
		return nil
	}

	// Start the periodic mail sync worker. Uses the WMail's own context which is
	// cancelled when the account is removed or terminates, so we don't leak goroutines.
	// Gmail (history) and Outlook/Graph (delta) always sync. Only generic
	// SMTP/IMAP mailboxes are opt-in via ImapSync.
	if e.Type == models.InboxProviderSMTPIMAP && !e.ImapSync {
		log.Info().Str("email_id", e.ID.String()).Str("email", e.Email).Msg("email account added (no sync)")
	} else {
		go mail.StartSyncWorker(mail.Ctx)
		log.Info().Str("email_id", e.ID.String()).Str("email", e.Email).Str("transport", string(mail.Transport)).Msg("email account added to worker")
	}
	w.releaseParkedSends(e.ID)
	return nil
}

// reportLoadAuthFailure raises EMAIL_AUTH_ERROR when a mailbox is refused at
// load for its credentials, so the control plane deactivates it and the owner
// is asked to reconnect. Without it the reconciler republishes the same
// refused mailbox every few minutes and nobody is told. The case that
// produces it: an OAuth mailbox moved to the smtp transport whose consent
// carries no IMAP/SMTP scope. Anything else (a dead server, a missing
// payload) is left to the next republish.
func (w *WorkerService) reportLoadAuthFailure(e *models.AddWorkerEmail, err error) {
	var mailErr *errx.MailError
	if !errors.As(err, &mailErr) || wmail.DetermineErrorEventType(mailErr) != models.JobEventTypeEmailAuthError {
		return
	}
	userInfo := mailErr.GetUserErrorInfo()
	event := models.EmailErrorEvent{
		EmailAccountID: e.ID.String(),
		UserID:         e.UserID.String(),
		ErrorCode:      string(mailErr.Code),
		ErrorType:      string(mailErr.Type),
		ResolveMethod:  string(mailErr.ResolveMethod),
		Message:        mailErr.Message,
		UserVisible:    mailErr.IsUserVisible(),
		UserTitle:      userInfo.Title,
		UserMessage:    userInfo.Message,
		ActionRequired: userInfo.ActionRequired,
		Timestamp:      time.Now().Unix(),
		WorkerID:       w.ID,
	}
	if perr := w.Produce(models.JobEventTypeEmailAuthError, e.ID.String(), event); perr != nil {
		log.Error().Err(perr).Str("email_id", e.ID.String()).Msg("failed to produce email auth error event")
	}
}
