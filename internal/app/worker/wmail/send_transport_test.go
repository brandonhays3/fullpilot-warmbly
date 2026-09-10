package wmail

import (
	"testing"

	"github.com/google/uuid"
	"github.com/warmbly/warmbly/internal/client/goog"
	"github.com/warmbly/warmbly/internal/client/msgraph"
	"github.com/warmbly/warmbly/internal/client/smtpimap/smtp"
	"github.com/warmbly/warmbly/internal/models"
)

// The split is a property of the task id: the same task always lands on the
// same side, the extremes send everything one way, and over many tasks the
// share is close to the configured one.
func TestChooseTransport(t *testing.T) {
	id := uuid.New()
	for i := 0; i < 5; i++ {
		if chooseTransport(id, 50) != chooseTransport(id, 50) {
			t.Fatal("same task id must always pick the same transport")
		}
	}
	for i := 0; i < 200; i++ {
		id := uuid.New()
		if chooseTransport(id, 100) != models.MailTransportAPI {
			t.Fatalf("100%% must always be api, got smtp for %s", id)
		}
		if chooseTransport(id, 0) != models.MailTransportSMTP {
			t.Fatalf("0%% must always be smtp, got api for %s", id)
		}
	}
	api := 0
	const n = 20000
	for i := 0; i < n; i++ {
		if chooseTransport(uuid.New(), 50) == models.MailTransportAPI {
			api++
		}
	}
	if share := float64(api) / n; share < 0.46 || share > 0.54 {
		t.Fatalf("api share at 50%% = %.3f, want about 0.5", share)
	}
}

// A mailbox that holds only one client sends through it whatever its task
// was assigned, and says so; one with neither cannot send at all.
func TestSendTransportFallsBackToTheClientTheMailboxHas(t *testing.T) {
	apiTask, smtpTask := uuid.New(), uuid.New()
	for chooseTransport(apiTask, 50) != models.MailTransportAPI {
		apiTask = uuid.New()
	}
	for chooseTransport(smtpTask, 50) != models.MailTransportSMTP {
		smtpTask = uuid.New()
	}

	both := &WMail{EmailType: models.InboxProviderGoogle, SendAPIPercent: 50,
		GoogleData:   &GoogleData{Client: &goog.Client{}},
		SmtpImapData: &SmtpImapData{SmtpClient: &smtp.Client{}}}
	if tr, fb, ok := both.sendTransport(apiTask); tr != models.MailTransportAPI || fb || !ok {
		t.Fatalf("both/api task = %v %v %v", tr, fb, ok)
	}
	if tr, fb, ok := both.sendTransport(smtpTask); tr != models.MailTransportSMTP || fb || !ok {
		t.Fatalf("both/smtp task = %v %v %v", tr, fb, ok)
	}

	apiOnly := &WMail{EmailType: models.InboxProviderOutlook, SendAPIPercent: 50, GraphData: &GraphData{Client: &msgraph.Client{}}}
	if tr, fb, ok := apiOnly.sendTransport(smtpTask); tr != models.MailTransportAPI || !fb || !ok {
		t.Fatalf("api only/smtp task = %v %v %v", tr, fb, ok)
	}
	smtpOnly := &WMail{EmailType: models.InboxProviderOutlook, SendAPIPercent: 50, SmtpImapData: &SmtpImapData{SmtpClient: &smtp.Client{}}}
	if tr, fb, ok := smtpOnly.sendTransport(apiTask); tr != models.MailTransportSMTP || !fb || !ok {
		t.Fatalf("smtp only/api task = %v %v %v", tr, fb, ok)
	}

	none := &WMail{EmailType: models.InboxProviderOutlook, SendAPIPercent: 50}
	if _, _, ok := none.sendTransport(apiTask); ok {
		t.Fatal("a mailbox with no client must not report a usable transport")
	}

	// A plain smtp_imap mailbox is SMTP regardless of the split.
	plain := &WMail{EmailType: models.InboxProviderSMTPIMAP, SendAPIPercent: 100, SmtpImapData: &SmtpImapData{SmtpClient: &smtp.Client{}}}
	if tr, fb, ok := plain.sendTransport(apiTask); tr != models.MailTransportSMTP || fb || !ok {
		t.Fatalf("smtp_imap = %v %v %v", tr, fb, ok)
	}
}

func TestSendMethodLabel(t *testing.T) {
	cases := []struct {
		transport  models.MailTransport
		provider   models.InboxProvider
		deployment string
		want       string
	}{
		{models.MailTransportSMTP, models.InboxProviderGoogle, "cloud_run_worker", "smtp_google_cloud_run_worker"},
		{models.MailTransportAPI, models.InboxProviderGoogle, "cloud_run_worker", "api_google_cloud_run_worker"},
		{models.MailTransportSMTP, models.InboxProviderGoogle, "cloud_vm", "smtp_google_cloud_vm"},
		{models.MailTransportAPI, models.InboxProviderOutlook, "cloud_run_worker", "api_microsoft_cloud_run_worker"},
		{models.MailTransportSMTP, models.InboxProviderSMTPIMAP, "cloud_vm", "smtp_smtpimap_cloud_vm"},
		{models.MailTransportSMTP, models.InboxProviderSMTPIMAP, "", "smtp_smtpimap_unknown"},
	}
	for _, c := range cases {
		if got := models.SendMethodLabel(c.transport, c.provider, c.deployment); got != c.want {
			t.Errorf("SendMethodLabel(%s, %s, %q) = %q, want %q", c.transport, c.provider, c.deployment, got, c.want)
		}
	}
}
