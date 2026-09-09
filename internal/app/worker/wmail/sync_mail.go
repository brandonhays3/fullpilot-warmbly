package wmail

import (
	"context"

	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
)

func (w *WMail) SyncMail(ctx context.Context) *errx.MailError {
	// The transport, not the provider, picks the path: an OAuth mailbox on
	// the smtp transport syncs over the provider's IMAP.
	switch {
	case w.UsesSmtpImap():
		return w.Sync(ctx)
	case w.EmailType == models.InboxProviderGoogle:
		return w.SyncGoogle(ctx)
	case w.EmailType == models.InboxProviderOutlook:
		return w.SyncGraph(ctx)
	default:
		return nil
	}
}
