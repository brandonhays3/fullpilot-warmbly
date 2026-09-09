package models

import "testing"

func TestAddWorkerEmailMailTransport(t *testing.T) {
	cases := []struct {
		provider  InboxProvider
		transport string
		want      MailTransport
	}{
		// A publisher older than the field, or one that says api, drives the
		// provider API.
		{InboxProviderGoogle, "", MailTransportAPI},
		{InboxProviderGoogle, "api", MailTransportAPI},
		{InboxProviderGoogle, "bogus", MailTransportAPI},
		{InboxProviderGoogle, "smtp", MailTransportSMTP},
		{InboxProviderOutlook, "smtp", MailTransportSMTP},
		// An smtp_imap mailbox has only one transport whatever is stamped.
		{InboxProviderSMTPIMAP, "", MailTransportSMTP},
		{InboxProviderSMTPIMAP, "api", MailTransportSMTP},
	}
	for _, c := range cases {
		a := &AddWorkerEmail{Type: c.provider, Transport: c.transport}
		if got := a.MailTransport(); got != c.want {
			t.Errorf("%s/%q: MailTransport() = %q, want %q", c.provider, c.transport, got, c.want)
		}
		if got := a.UsesSmtpImap(); got != (c.want == MailTransportSMTP) {
			t.Errorf("%s/%q: UsesSmtpImap() = %v", c.provider, c.transport, got)
		}
	}
}
