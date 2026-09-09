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
	// OutlookDesktop is the Microsoft counterpart: a public (no secret)
	// desktop-type Entra app whose redirect is https://localhost, so the same
	// paste-the-address flow applies. Thunderbird's public client, for instance.
	OutlookDesktop *oauth2.Config
}

// OutlookDesktopRedirect is the loopback redirect_uri registered on
// Thunderbird's Microsoft client (Entra ignores the port on loopback). Nothing listens there; the user pastes it.
const OutlookDesktopRedirect = "http://127.0.0.1"

// GoogleDesktopRedirect is the loopback redirect_uri for GoogleDesktop. The
// port is arbitrary; nothing listens on it.
const GoogleDesktopRedirect = "http://localhost:17777/fullpilot/oauth"

// OAuth mailbox transports (the values of OAUTH_MAILBOX_TRANSPORT). They
// mirror models.MailTransport; config cannot import models.
const (
	// OAuthTransportAPI drives Gmail through the Gmail API and Outlook
	// through Microsoft Graph.
	OAuthTransportAPI = "api"
	// OAuthTransportSMTP drives both through the provider's IMAP and SMTP
	// submission endpoints with the same OAuth token (XOAUTH2), which is the
	// path that exercises the provider's real SMTP end to end.
	OAuthTransportSMTP = "smtp"
)

// OAuthMailboxTransport is how OAuth mailboxes are driven, from
// OAUTH_MAILBOX_TRANSPORT. The default is smtp for now, so provider SMTP is
// what campaigns and replies go through; set api to return to the Gmail API
// and Microsoft Graph. It decides two things at once: the scopes a new
// consent asks for (below), and the transport the control plane ships to the
// worker for each OAuth mailbox (email.buildAddWorkerEmail).
func OAuthMailboxTransport() string {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("OAUTH_MAILBOX_TRANSPORT"))) {
	case OAuthTransportAPI:
		return OAuthTransportAPI
	default:
		return OAuthTransportSMTP
	}
}

// Provider IMAP and SMTP submission endpoints for OAuth mailboxes on the smtp
// transport. Both providers accept XOAUTH2 on these; 587 is STARTTLS, 993 is
// implicit TLS. Gmail and Exchange Online both file a copy of every message
// submitted over SMTP in the sender's Sent folder, so the worker must not
// APPEND its own (it would double every sent message).
const (
	GmailSMTPHost   = "smtp.gmail.com"
	GmailSMTPPort   = 587
	GmailIMAPHost   = "imap.gmail.com"
	GmailIMAPPort   = 993
	OutlookSMTPHost = "smtp.office365.com"
	OutlookSMTPPort = 587
	OutlookIMAPHost = "outlook.office365.com"
	OutlookIMAPPort = 993
)

// Microsoft scopes. Entra issues one access token per resource, so a
// consent must not mix graph.microsoft.com and outlook.office.com scopes:
// the token would carry the first resource and the second would refuse it.
const (
	GraphUserReadScope      = "https://graph.microsoft.com/User.Read"
	GraphMailSendScope      = "https://graph.microsoft.com/Mail.Send"
	GraphMailReadWriteScope = "https://graph.microsoft.com/Mail.ReadWrite"
	OutlookIMAPScope        = "https://outlook.office.com/IMAP.AccessAsUser.All"
	OutlookSMTPScope        = "https://outlook.office.com/SMTP.Send"
)

// identityScopes let the connect flow read the account owner's name and
// address: Google's userinfo endpoint and Microsoft's id_token / OIDC
// userinfo both need them. They add no mailbox access.
var identityScopes = []string{"openid", "email", "profile"}

// redirectOverride lets a deployment present a redirect_uri that is already
// registered on an existing OAuth client (a host it controls that forwards to
// the real callback), instead of API base + callback path.
func redirectOverride(envKey, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(envKey)); v != "" {
		return v
	}
	return fallback
}

// googleScopes are the Gmail scopes for the web client: the granular gmail.*
// set the API transport needs, plus the full mail scope on the smtp
// transport, which is the only Gmail scope IMAP and SMTP accept.
func googleScopes() []string {
	scopes := append([]string{}, identityScopes...)
	scopes = append(scopes,
		gmail.GmailComposeScope,
		gmail.GmailMetadataScope,
		gmail.GmailModifyScope,
		gmail.GmailSendScope,
		gmail.GmailSettingsBasicScope,
		gmail.GmailReadonlyScope,
	)
	if OAuthMailboxTransport() == OAuthTransportSMTP {
		scopes = append(scopes, gmail.MailGoogleComScope)
	}
	return scopes
}

func GoogleOauth2Inbox(baseURL string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     os.Getenv("BOX_GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("BOX_GOOGLE_CLIENT_SECRET"),
		RedirectURL:  redirectOverride("BOX_GOOGLE_REDIRECT_URL", baseURL+"/addresses/google/callback"),
		Scopes:       googleScopes(),
		Endpoint:     google.Endpoint,
	}
}

// GoogleDesktopOauth2Inbox is the desktop-type Gmail client, read from
// BOX_GOOGLE_DESKTOP_CLIENT_ID/SECRET. Desktop clients are verified for the
// full mail scope rather than the granular gmail.* ones; the Gmail API accepts
// it for every method Fullpilot calls, and it is also the scope Gmail's IMAP
// and SMTP endpoints take, so the same token serves both transports.
func GoogleDesktopOauth2Inbox() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     os.Getenv("BOX_GOOGLE_DESKTOP_CLIENT_ID"),
		ClientSecret: os.Getenv("BOX_GOOGLE_DESKTOP_CLIENT_SECRET"),
		RedirectURL:  GoogleDesktopRedirect,
		Scopes:       append(append([]string{}, identityScopes...), gmail.MailGoogleComScope),
		Endpoint:     google.Endpoint,
	}
}

// outlookScopes are the Microsoft scopes for the configured transport.
//
// api: Graph is the transport (RAW MIME sendMail + delta sync), so Mail.Send
// (send), Mail.ReadWrite (delta sync + warmup move/mark/flag) and User.Read
// (resolve the mailbox owner via /me). None need tenant admin consent by
// default and all work on personal Outlook.com accounts.
//
// smtp: the legacy IMAP/SMTP resource instead, IMAP.AccessAsUser.All and
// SMTP.Send, which is what outlook.office365.com and smtp.office365.com accept
// over XOAUTH2. No Graph scope rides along (one token per resource), so the
// owner's name and address come from the id_token and the OIDC userinfo
// endpoint rather than from Graph /me. A mailbox consented under one set
// must reconsent to move to the other.
//
// Both carry offline_access (refresh token) and the identity scopes.
func outlookScopes() []string {
	scopes := append([]string{}, identityScopes...)
	scopes = append(scopes, "offline_access")
	if OAuthMailboxTransport() == OAuthTransportSMTP {
		return append(scopes, OutlookIMAPScope, OutlookSMTPScope)
	}
	return append(scopes, GraphUserReadScope, GraphMailSendScope, GraphMailReadWriteScope)
}

// OutlookGraphScoped reports whether a Microsoft client's consent carries a
// Graph scope, in which case its access token is a Graph token and /me can
// resolve the owner. Without one the token is for outlook.office.com.
func OutlookGraphScoped(cfg *oauth2.Config) bool {
	if cfg == nil {
		return false
	}
	for _, s := range cfg.Scopes {
		if strings.HasPrefix(s, "https://graph.microsoft.com/") {
			return true
		}
	}
	return false
}

var microsoftEndpoint = oauth2.Endpoint{
	AuthURL:  "https://login.microsoftonline.com/common/oauth2/v2.0/authorize",
	TokenURL: "https://login.microsoftonline.com/common/oauth2/v2.0/token",
}

// OutlookOauth2Inbox configures delegated access for Outlook / Microsoft 365
// mailboxes through the web client; see outlookScopes for what it asks for.
func OutlookOauth2Inbox(baseURL string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     os.Getenv("BOX_OUTLOOK_CLIENT_ID"),
		ClientSecret: os.Getenv("BOX_OUTLOOK_CLIENT_SECRET"),
		RedirectURL:  redirectOverride("BOX_OUTLOOK_REDIRECT_URL", baseURL+"/addresses/outlook/callback"),
		Scopes:       outlookScopes(),
		Endpoint:     microsoftEndpoint,
	}
}

// OutlookDesktopOauth2Inbox is the desktop-type Microsoft client, read from
// BOX_OUTLOOK_DESKTOP_CLIENT_ID (public client: the secret is optional). Same
// scopes as the web client for the configured transport; Entra consents to
// them dynamically.
func OutlookDesktopOauth2Inbox() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     os.Getenv("BOX_OUTLOOK_DESKTOP_CLIENT_ID"),
		ClientSecret: os.Getenv("BOX_OUTLOOK_DESKTOP_CLIENT_SECRET"),
		RedirectURL:  redirectOverride("BOX_OUTLOOK_DESKTOP_REDIRECT_URL", OutlookDesktopRedirect),
		Scopes:       outlookScopes(),
		Endpoint:     microsoftEndpoint,
	}
}
