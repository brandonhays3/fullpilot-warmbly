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

func TestScopesOnSMTPTransport(t *testing.T) {
	t.Setenv("OAUTH_MAILBOX_TRANSPORT", "smtp")

	// Both Google clients read the owner's name and hold the full mail scope
	// IMAP and SMTP accept.
	for name, s := range map[string][]string{"web": GoogleOauth2Inbox("https://api").Scopes, "desktop": GoogleDesktopOauth2Inbox().Scopes} {
		if !hasAll(s, "openid", "email", "profile", gmail.MailGoogleComScope) {
			t.Errorf("google %s scopes = %v", name, s)
		}
	}

	// Microsoft: the outlook.office.com resource and nothing from Graph, since
	// Entra issues one token per resource.
	for name, cfg := range map[string]*oauth2.Config{"web": OutlookOauth2Inbox("https://api"), "desktop": OutlookDesktopOauth2Inbox()} {
		s := cfg.Scopes
		if !hasAll(s, "openid", "email", "profile", "offline_access", OutlookIMAPScope, OutlookSMTPScope) {
			t.Errorf("outlook %s scopes = %v", name, s)
		}
		if hasAny(s, GraphUserReadScope, GraphMailSendScope, GraphMailReadWriteScope) {
			t.Errorf("outlook %s scopes must not mix in Graph: %v", name, s)
		}
		if OutlookGraphScoped(cfg) {
			t.Errorf("outlook %s smtp consent must not read as Graph-scoped", name)
		}
	}
}

func TestScopesOnAPITransport(t *testing.T) {
	t.Setenv("OAUTH_MAILBOX_TRANSPORT", "api")

	if s := GoogleOauth2Inbox("https://api").Scopes; !hasAll(s, gmail.GmailSendScope, gmail.GmailReadonlyScope, "profile") || slices.Contains(s, gmail.MailGoogleComScope) {
		t.Errorf("google web scopes = %v", s)
	}
	for name, cfg := range map[string]*oauth2.Config{"web": OutlookOauth2Inbox("https://api"), "desktop": OutlookDesktopOauth2Inbox()} {
		s := cfg.Scopes
		if !hasAll(s, GraphUserReadScope, GraphMailSendScope, GraphMailReadWriteScope, "offline_access") || hasAny(s, OutlookIMAPScope, OutlookSMTPScope) {
			t.Errorf("outlook %s scopes = %v", name, s)
		}
		if !OutlookGraphScoped(cfg) {
			t.Errorf("outlook %s api consent must read as Graph-scoped", name)
		}
	}
	if OutlookGraphScoped(nil) {
		t.Error("nil config is not Graph-scoped")
	}
}
