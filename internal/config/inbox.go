package config

import (
	"os"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
)

type Oauth2Inbox struct {
	Google  *oauth2.Config
	Outlook *oauth2.Config
}

// GoogleLocalhostRedirect is the redirect_uri used when BOX_GOOGLE_REDIRECT_MODE
// is "localhost". Google only lets desktop-type OAuth clients (such as
// Thunderbird's public client) redirect to a loopback address, so the consent
// popup lands on a dead localhost page and the dashboard asks the user to paste
// that page's address; the code and state are read out of it. The port is
// arbitrary and nothing listens on it.
const GoogleLocalhostRedirect = "http://localhost:17777/warmbly/oauth"

// GoogleManualRedirect reports whether the Gmail OAuth flow runs against a
// desktop-type client and therefore needs the paste-the-URL step in the
// dashboard instead of the API callback page.
func GoogleManualRedirect() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("BOX_GOOGLE_REDIRECT_MODE")), "localhost")
}

func GoogleOauth2Inbox(baseURL string) *oauth2.Config {
	if GoogleManualRedirect() {
		// Desktop-type clients are verified for the full mail scope, not the
		// granular gmail.* ones; the Gmail API accepts it for every method
		// Warmbly calls.
		return &oauth2.Config{
			ClientID:     os.Getenv("BOX_GOOGLE_CLIENT_ID"),
			ClientSecret: os.Getenv("BOX_GOOGLE_CLIENT_SECRET"),
			RedirectURL:  GoogleLocalhostRedirect,
			Scopes:       []string{gmail.MailGoogleComScope},
			Endpoint:     google.Endpoint,
		}
	}
	return &oauth2.Config{
		ClientID:     os.Getenv("BOX_GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("BOX_GOOGLE_CLIENT_SECRET"),
		RedirectURL:  baseURL + "/addresses/google/callback",
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
		RedirectURL:  baseURL + "/addresses/outlook/callback",
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
