package config

import (
	"os"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
)

type Oauth2Inbox struct {
	Google *oauth2.Config
	// GoogleDesktop is an optional second Gmail client of Google's "desktop"
	// type (Thunderbird's public client, for instance). Google only lets that
	// type redirect to a loopback address, so the consent popup lands on a dead
	// localhost page and the dashboard asks the user to paste its address.
	// Tokens it issues can only be refreshed with it, so a mailbox remembers
	// which client connected it (email_accounts_oauth.oauth_client).
	GoogleDesktop *oauth2.Config
	Outlook       *oauth2.Config
}

// GoogleDesktopRedirect is the loopback redirect_uri for GoogleDesktop. The
// port is arbitrary; nothing listens on it.
const GoogleDesktopRedirect = "http://localhost:17777/warmbly/oauth"

// redirectOverride lets a deployment present a redirect_uri that is already
// registered on an existing OAuth client (a host it controls that forwards to
// the real callback), instead of API base + callback path.
func redirectOverride(envKey, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(envKey)); v != "" {
		return v
	}
	return fallback
}

func GoogleOauth2Inbox(baseURL string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     os.Getenv("BOX_GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("BOX_GOOGLE_CLIENT_SECRET"),
		RedirectURL:  redirectOverride("BOX_GOOGLE_REDIRECT_URL", baseURL+"/addresses/google/callback"),
		Scopes: []string{
			gmail.GmailComposeScope,
			gmail.GmailMetadataScope,
			gmail.GmailModifyScope,
			gmail.GmailSendScope,
			gmail.GmailSettingsBasicScope,
			gmail.GmailReadonlyScope,
		},
		Endpoint: google.Endpoint,
	}
}

// GoogleDesktopOauth2Inbox is the desktop-type Gmail client, read from
// BOX_GOOGLE_DESKTOP_CLIENT_ID/SECRET. Desktop clients are verified for the
// full mail scope rather than the granular gmail.* ones; the Gmail API accepts
// it for every method Warmbly calls.
func GoogleDesktopOauth2Inbox() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     os.Getenv("BOX_GOOGLE_DESKTOP_CLIENT_ID"),
		ClientSecret: os.Getenv("BOX_GOOGLE_DESKTOP_CLIENT_SECRET"),
		RedirectURL:  GoogleDesktopRedirect,
		Scopes:       []string{gmail.MailGoogleComScope},
		Endpoint:     google.Endpoint,
	}
}

// OutlookOauth2Inbox configures delegated Microsoft Graph access for Outlook /
// Microsoft 365 mailboxes. Graph is the transport now (RAW MIME sendMail + delta
// sync), so we request Graph scopes rather than the legacy IMAP/SMTP scopes:
// Mail.Send (send), Mail.ReadWrite (delta sync + warmup move/mark/flag),
// User.Read (resolve the mailbox owner via /me), and offline_access (refresh
// token). None require tenant admin consent by default and all work on personal
// Outlook.com accounts.
func OutlookOauth2Inbox(baseURL string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     os.Getenv("BOX_OUTLOOK_CLIENT_ID"),
		ClientSecret: os.Getenv("BOX_OUTLOOK_CLIENT_SECRET"),
		RedirectURL:  redirectOverride("BOX_OUTLOOK_REDIRECT_URL", baseURL+"/addresses/outlook/callback"),
		Scopes: []string{
			"openid",
			"email",
			"profile",
			"offline_access",
			"https://graph.microsoft.com/User.Read",
			"https://graph.microsoft.com/Mail.Send",
			"https://graph.microsoft.com/Mail.ReadWrite",
		},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://login.microsoftonline.com/common/oauth2/v2.0/authorize",
			TokenURL: "https://login.microsoftonline.com/common/oauth2/v2.0/token",
		},
	}
}
