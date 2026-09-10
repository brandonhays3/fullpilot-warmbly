package wmail

import (
	"hash/fnv"

	"github.com/google/uuid"
	"github.com/warmbly/warmbly/internal/models"
)

// sendBucket places a task in 0..99 from a hash of its id, so the transport
// a send is assigned is a property of the task: a retry of the same task
// takes the same path, and the split holds across workers without any
// shared state.
func sendBucket(taskID uuid.UUID) int {
	h := fnv.New32a()
	_, _ = h.Write(taskID[:])
	return int(h.Sum32() % 100)
}

// chooseTransport is the transport a send with this task id is assigned when
// apiPercent of sends go through the provider API and the rest through SMTP.
func chooseTransport(taskID uuid.UUID, apiPercent int) models.MailTransport {
	if sendBucket(taskID) < apiPercent {
		return models.MailTransportAPI
	}
	return models.MailTransportSMTP
}

// canSendAPI and canSendSMTP report which clients this mailbox holds.
func (w *WMail) canSendAPI() bool {
	switch w.EmailType {
	case models.InboxProviderGoogle:
		return w.GoogleData != nil && w.GoogleData.Client != nil
	case models.InboxProviderOutlook:
		return w.GraphData != nil && w.GraphData.Client != nil
	}
	return false
}

func (w *WMail) canSendSMTP() bool {
	return w.SmtpImapData != nil && w.SmtpImapData.SmtpClient != nil
}

// sendTransport picks the path for one send. An smtp_imap mailbox has only
// SMTP. An OAuth mailbox takes the transport its task id is assigned
// (SendAPIPercent), unless it holds only one of the two clients (a brokered
// token, a consent without the other path's scopes), in which case it takes
// the one it has and reports the fallback. ok is false when it has neither.
func (w *WMail) sendTransport(taskID uuid.UUID) (transport models.MailTransport, fallback bool, ok bool) {
	if w.EmailType == models.InboxProviderSMTPIMAP {
		return models.MailTransportSMTP, false, w.canSendSMTP()
	}
	api, smtp := w.canSendAPI(), w.canSendSMTP()
	want := chooseTransport(taskID, w.SendAPIPercent)
	switch {
	case want == models.MailTransportAPI && api:
		return models.MailTransportAPI, false, true
	case want == models.MailTransportSMTP && smtp:
		return models.MailTransportSMTP, false, true
	case api:
		return models.MailTransportAPI, true, true
	case smtp:
		return models.MailTransportSMTP, true, true
	}
	return want, false, false
}
