package config

import (
	"slices"
	"testing"

	"golang.org/x/oauth2"
	"google.golang.org/api/gmail/v1"
)

func TestOAuthMailboxTransportDefaultsToSMTP(t *testing.T) {
	t.Setenv("OAUTH_MAILBOX_TRANSPORT", "")
	if got := OAuthMailboxTransport(); got != OAuthTransportSMTP {
		t.Fatalf("default = %q, want smtp", got)
	}
	t.Setenv("OAUTH_MAILBOX_TRANSPORT", " API ")
	if got := OAuthMailboxTransport(); got != OAuthTransportAPI {
		t.Fatalf("api = %q", got)
	}
	t.Setenv("OAUTH_MAILBOX_TRANSPORT", "nonsense")
	if got := OAuthMailboxTransport(); got != OAuthTransportSMTP {
		t.Fatalf("unknown value = %q, want smtp", got)
	}
}

func hasAll(scopes []string, want ...string) bool {
	for _, w := range want {
		if !slices.Contains(scopes, w) {
			return false
		}
	}
	return true
}

func hasAny(scopes []string, any ...string) bool {
	for _, a := range any {
		if slices.Contains(scopes, a) {
			return true
		}
	}
	return false
}

// Every consent covers both send paths, whichever sync transport is set.
func TestScopesCoverBothTransports(t *testing.T) {
	for _, transport := range []string{"smtp", "api"} {
		t.Setenv("OAUTH_MAILBOX_TRANSPORT", transport)

		// Both Google clients read the owner's name and hold the full mail
		// scope IMAP and SMTP accept; the web client also has the granular
		// gmail.* scopes the API path was verified for.
		for name, s := range map[string][]string{"web": GoogleOauth2Inbox("https://api").Scopes, "desktop": GoogleDesktopOauth2Inbox().Scopes} {
			if !hasAll(s, "openid", "email", "profile", gmail.MailGoogleComScope) {
				t.Errorf("%s: google %s scopes = %v", transport, name, s)
			}
		}
		if s := GoogleOauth2Inbox("https://api").Scopes; !hasAll(s, gmail.GmailSendScope, gmail.GmailReadonlyScope) {
			t.Errorf("%s: google web scopes = %v", transport, s)
		}

		// Microsoft: both resource families on one consent, and the config
		// reads as Graph-scoped so the owner resolves through /me.
		for name, cfg := range map[string]*oauth2.Config{"web": OutlookOauth2Inbox("https://api"), "desktop": OutlookDesktopOauth2Inbox()} {
			s := cfg.Scopes
			if !hasAll(s, "openid", "email", "profile", "offline_access", OutlookIMAPScope, OutlookSMTPScope, GraphUserReadScope, GraphMailSendScope, GraphMailReadWriteScope) {
				t.Errorf("%s: outlook %s scopes = %v", transport, name, s)
			}
			if !OutlookGraphScoped(cfg) {
				t.Errorf("%s: outlook %s consent must read as Graph-scoped", transport, name)
			}
		}
	}
	if OutlookGraphScoped(nil) {
		t.Error("nil config is not Graph-scoped")
	}
}

// A token request names one resource: the Graph set carries no
// outlook.office.com scope and the mail set no Graph scope.
func TestOutlookTokenScopesAreSingleResource(t *testing.T) {
	if s := OutlookGraphTokenScopes(); !hasAll(s, "openid", "offline_access", GraphUserReadScope, GraphMailSendScope, GraphMailReadWriteScope) || hasAny(s, OutlookIMAPScope, OutlookSMTPScope) {
		t.Errorf("graph token scopes = %v", s)
	}
	if s := OutlookMailTokenScopes(); !hasAll(s, "offline_access", OutlookIMAPScope, OutlookSMTPScope) || hasAny(s, GraphUserReadScope, GraphMailSendScope, GraphMailReadWriteScope) {
		t.Errorf("mail token scopes = %v", s)
	}
}
