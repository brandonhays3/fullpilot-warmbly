package wmail

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/warmbly/warmbly/internal/client/goog"
	"github.com/warmbly/warmbly/internal/client/msgraph"
	"github.com/warmbly/warmbly/internal/client/smtpimap/imap"
	"github.com/warmbly/warmbly/internal/client/smtpimap/smtp"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/pkg/stoken"
	"golang.org/x/oauth2"
	"google.golang.org/api/gmail/v1"
)

// An OAuth mailbox holds both an API client and an SMTP client, so every
// send can take either path (sendTransport). Which one SYNCS the mailbox is
// w.Transport, from OAUTH_MAILBOX_TRANSPORT: on smtp an IMAP session is
// opened too, on api it is not. A brokered (cloud-managed) token is minted
// for the provider API and cannot authenticate IMAP or SMTP, so a brokered
// mailbox stays on the API for both.

func missingOAuthCredentials(what string) *errx.MailError {
	return errx.MError(
		errx.MailErrorCritical,
		errx.MailErrorCodeAuthenticationFailed,
		"missing "+what+" credentials in add-email payload",
		errx.MailErrorResolveMethodReload,
	)
}

// forceAPISync moves a mailbox whose token cannot reach IMAP onto API sync.
func (w *WMail) forceAPISync(reason string) {
	if w.Transport == models.MailTransportSMTP {
		log.Warn().Str("email_id", w.ID.String()).Msg(reason + "; syncing over the provider API")
		w.Transport = models.MailTransportAPI
	}
}

func (w *WMail) initGoogle(ctx context.Context, data *models.AddWorkerEmail) *errx.MailError {
	if data.Google == nil {
		return missingOAuthCredentials("Google")
	}
	client := &goog.Client{
		Email:     data.Email,
		FirstName: data.FirstName,
		LastName:  data.LastName,

		Cache:           w.Cache,
		OnMessageAdded:  w.onGoogleMessageAdded,
		OnMessageRemove: w.onGoogleMessageRemove,
		OnLabelAdd:      w.onGoogleMessageLabelsAdded,
		OnLabelRemove:   w.onGoogleMessageLabelsRemoved,
	}
	w.GoogleData = &GoogleData{Client: client, LastHistoryID: data.Google.LastHistoryID}

	if data.TokenSource != nil {
		w.forceAPISync("brokered mailbox cannot use the smtp transport")
		return client.InitWithSource(ctx, data.TokenSource)
	}
	if data.Google.Token == nil {
		return missingOAuthCredentials("Google")
	}

	// One Gmail token serves the API and XOAUTH2 alike, so one refreshing
	// source feeds every client. Without the relay a refreshed token is never
	// persisted and the mailbox stops roughly an hour after connect.
	var ts oauth2.TokenSource = oauth2.ReuseTokenSource(data.Google.Token, data.Cfg.TokenSource(ctx, data.Google.Token))
	ts = stoken.New(ts, w.onTokenUpdate)
	if err := client.InitWithSource(ctx, ts); err != nil {
		return err
	}

	// A web-client consent from before the full mail scope was always asked
	// for cannot authenticate IMAP or SMTP; such a mailbox sends through the
	// API only until it is reconnected.
	if !googleTokenCanMail(ctx, ts) {
		log.Warn().Str("email_id", w.ID.String()).Msg("gmail consent lacks the full mail scope; sends go through the Gmail API only until the mailbox is reconnected")
		w.forceAPISync("gmail consent lacks the full mail scope")
		return nil
	}
	return w.initOAuthSmtp(ctx, data, ts)
}

func (w *WMail) initOutlook(ctx context.Context, data *models.AddWorkerEmail, policy models.SyncPolicy) *errx.MailError {
	if data.Graph == nil {
		return missingOAuthCredentials("Microsoft Graph")
	}
	var syncSince time.Time
	if policy.SyncSince != nil {
		syncSince = *policy.SyncSince
	}
	client := &msgraph.Client{
		Email:     data.Email,
		FirstName: data.FirstName,
		LastName:  data.LastName,

		Cache:      w.Cache,
		DeltaLinks: cloneStringMap(data.Graph.DeltaLinks),
		SyncSince:  syncSince,

		OnMessageSeen:   w.onGraphMessageSeen,
		OnMessageRemove: w.onGraphMessageRemove,
		OnDelta:         w.onGraphDelta,
	}
	w.GraphData = &GraphData{Client: client}

	if data.TokenSource != nil {
		w.forceAPISync("brokered mailbox cannot use the smtp transport")
		return client.InitWithSource(ctx, data.TokenSource)
	}
	if data.Graph.Token == nil || data.Graph.Token.RefreshToken == "" {
		return missingOAuthCredentials("Microsoft Graph")
	}

	// Entra issues one token per resource, so Graph and IMAP/SMTP each get
	// their own from the refresh token. Minting each once here is also the
	// only way to learn which resources the consent covers: a mailbox
	// connected before both were asked for has one of the two.
	tokens := newOutlookTokens(ctx, data.Cfg, data.Graph.Token.RefreshToken, w.onTokenUpdate)
	_, graphErr := tokens.Graph()
	_, mailErr := tokens.Mail()
	if graphErr != nil && mailErr != nil {
		log.Warn().Err(graphErr).AnErr("mail_error", mailErr).Str("email_id", w.ID.String()).Msg("outlook refresh token mints no usable access token")
		return errx.MError(
			errx.MailErrorCritical,
			errx.MailErrorCodeAuthenticationFailed,
			"Microsoft token refresh failed: "+graphErr.Error(),
			errx.MailErrorResolveMethodReload,
		)
	}

	if graphErr != nil {
		log.Warn().Err(graphErr).Str("email_id", w.ID.String()).Msg("outlook consent lacks the Graph scopes; sends go through SMTP only until the mailbox is reconnected")
		w.GraphData = nil
		if w.Transport != models.MailTransportSMTP {
			log.Warn().Str("email_id", w.ID.String()).Msg("outlook consent lacks the Graph scopes; syncing over IMAP")
			w.Transport = models.MailTransportSMTP
		}
	} else if err := client.InitWithSource(ctx, tokens.GraphSource()); err != nil {
		return err
	}

	if mailErr != nil {
		log.Warn().Err(mailErr).Str("email_id", w.ID.String()).Msg("outlook consent lacks the IMAP/SMTP scopes; sends go through Graph only until the mailbox is reconnected")
		w.forceAPISync("outlook consent lacks the IMAP/SMTP scopes")
		return nil
	}
	return w.initOAuthSmtp(ctx, data, tokens.MailSource())
}

// initOAuthSmtp builds the SMTP client for an OAuth mailbox on the provider's
// submission endpoint and, when the mailbox syncs over IMAP, the IMAP session
// with its saved folder cursors. ts must yield a token the provider's IMAP
// and SMTP accept over XOAUTH2.
func (w *WMail) initOAuthSmtp(ctx context.Context, data *models.AddWorkerEmail, ts oauth2.TokenSource) *errx.MailError {
	smtpHost, smtpPort, imapHost, imapPort, ok := providerSmtpImap(data.Type)
	if !ok {
		return missingOAuthCredentials("OAuth")
	}
	w.SmtpImapData = &SmtpImapData{
		SmtpClient: &smtp.Client{
			FirstName: data.FirstName,
			LastName:  data.LastName,
			Email:     data.Email,
			AuthType:  models.AuthOAuth2,
			Oauth2:    &models.Oauth2Service{Host: smtpHost, Port: smtpPort, Token: ts},
		},
	}
	// A send-only copy never syncs, and the provider files the Sent copy of an
	// SMTP submission itself, so it needs no IMAP session at all.
	if w.Transport != models.MailTransportSMTP || w.SendOnly {
		return nil
	}

	conn := &imap.Client{
		Email:    data.Email,
		AuthType: models.AuthOAuth2,
		Oauth2:   &models.Oauth2Service{Host: imapHost, Port: imapPort, Token: ts},
	}
	if err := conn.Connect(); err != nil {
		return err
	}
	w.SmtpImapData.ImapClient = conn
	// Saved folder cursors, same as an smtp_imap mailbox: live sync resumes
	// from each folder's stored HIGHESTMODSEQ (or UIDNEXT) instead of
	// re-baselining on every worker restart.
	if data.SmtpImap != nil {
		for i := range data.SmtpImap.Mailboxes {
			box := data.SmtpImap.Mailboxes[i]
			w.SmtpImapData.Mailboxes = append(w.SmtpImapData.Mailboxes, &box)
		}
	}
	return nil
}

// googleTokeninfoURL reports a Google access token's scopes. A variable so
// tests can point it at a local server.
var googleTokeninfoURL = "https://oauth2.googleapis.com/tokeninfo"

var googleTokeninfoClient = &http.Client{Timeout: 10 * time.Second}

// googleTokenCanMail reports whether the token carries the full mail scope,
// the only Gmail scope IMAP and SMTP accept. Anything short of a definite
// answer (no token, a network error, an unexpected body) is read as yes, so
// an outage of the introspection endpoint never strips a mailbox of SMTP.
func googleTokenCanMail(ctx context.Context, ts oauth2.TokenSource) bool {
	tok, err := ts.Token()
	if err != nil || tok == nil || tok.AccessToken == "" {
		return true
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, googleTokeninfoURL+"?access_token="+url.QueryEscape(tok.AccessToken), nil)
	if err != nil {
		return true
	}
	resp, err := googleTokeninfoClient.Do(req)
	if err != nil {
		return true
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return true
	}
	var info struct {
		Scope string `json:"scope"`
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if err := json.Unmarshal(body, &info); err != nil || strings.TrimSpace(info.Scope) == "" {
		return true
	}
	for _, s := range strings.Fields(info.Scope) {
		if s == gmail.MailGoogleComScope {
			return true
		}
	}
	return false
}
