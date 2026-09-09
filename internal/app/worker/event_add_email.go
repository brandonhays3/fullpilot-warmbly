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

	if w.mailManager.Has(e.ID) {
		// Already loaded: keep the handler idempotent, but take a changed sync
		// budget so an operator's settings change reaches this mailbox.
		if mail := w.mailManager.Get(e.ID); mail != nil {
			mail.ApplySyncPolicy(e.Sync)
		}
		return nil
	}

	if err := w.mailManager.AddWMail(ctx, e); err != nil {
		log.Error().Err(err).Str("email_id", e.ID.String()).Msg("failed to add email account to worker")
		w.reportLoadAuthFailure(e, err)
		return err
	}

	// Start the periodic mail sync worker. Uses the WMail's own context which is
	// cancelled when the account is removed or terminates, so we don't leak goroutines.
	mail := w.mailManager.Get(e.ID)
	if mail != nil {
		// Gmail (history) and Outlook/Graph (delta) always sync. Only generic
		// SMTP/IMAP mailboxes are opt-in via ImapSync.
		if e.Type == models.InboxProviderSMTPIMAP && !e.ImapSync {
			log.Info().Str("email_id", e.ID.String()).Str("email", e.Email).Msg("email account added (no sync)")
			return nil
		}
		go mail.StartSyncWorker(mail.Ctx)
	}

	log.Info().Str("email_id", e.ID.String()).Str("email", e.Email).Str("transport", string(mail.Transport)).Msg("email account added to worker")
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
	}
	if perr := w.Produce(models.JobEventTypeEmailAuthError, e.ID.String(), event); perr != nil {
		log.Error().Err(perr).Str("email_id", e.ID.String()).Msg("failed to produce email auth error event")
	}
}
