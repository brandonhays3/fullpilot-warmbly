package email

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/warmbly/warmbly/internal/config"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/infrastructure/pubsub"
	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/observability/errs"
	"github.com/warmbly/warmbly/internal/pkg/crypt"
	"golang.org/x/oauth2"
)

// OAuthStart issues a fresh state nonce and returns the provider-specific authorization URL.
// The caller is expected to redirect the user to the URL and post back to OAuthFinish on return.
func (s *emailService) OAuthStart(ctx context.Context, userID string, orgID *uuid.UUID, provider models.InboxProvider, client string) (*models.EmailOnboardingStartResponse, *errx.Error) {
	client = s.resolveOAuthClient(provider, client)
	cfg, xerr := s.oauthConfigFor(provider, client)
	if xerr != nil {
		return nil, xerr
	}

	// Refuse early so we don't waste an OAuth round-trip on a request
	// that the inbox-limit guard would reject after callback.
	if _, xerr := s.guardInboxLimit(ctx, orgID); xerr != nil {
		return nil, xerr
	}

	state, err := crypt.Nonce()
	if err != nil {
		errs.CaptureException(err)
		return nil, errx.InternalError()
	}

	if xerr := s.saveOnboardingState(ctx, state, &models.EmailOnboardingState{
		UserID:         userID,
		OrganizationID: orgID,
		Provider:       string(provider),
		Nonce:          state,
		OAuthClient:    client,
	}); xerr != nil {
		return nil, xerr
	}

	url := cfg.AuthCodeURL(
		state,
		oauth2.AccessTypeOffline,
		oauth2.ApprovalForce, // force refresh_token issuance on reconnect
	)
	return &models.EmailOnboardingStartResponse{
		URL:            url,
		State:          state,
		ManualRedirect: manualRedirect(client, cfg),
	}, nil
}

// manualRedirect reports whether the consent window will land somewhere the
// callback page cannot run: the desktop-type client, or any client whose
// registered redirect_uri is a loopback address. The dashboard then asks the
// user to paste the landing address instead of waiting for postMessage.
func manualRedirect(client string, cfg *oauth2.Config) bool {
	if client == models.OAuthClientGoogleDesktop || client == models.OAuthClientOutlookDesktop {
		return true
	}
	if cfg == nil {
		return false
	}
	u := strings.ToLower(cfg.RedirectURL)
	return strings.HasPrefix(u, "http://localhost") || strings.HasPrefix(u, "https://localhost") || strings.HasPrefix(u, "http://127.0.0.1")
}

// guardInboxLimit refuses a connect that would take the workspace past its
// mailbox allowance (fair use for paid plans, FreeWorkspaceMailboxLimit for
// free ones, unlimited without billing) and returns the resolved allowance so
// the insert can enforce it again under the organization's lock. The
// allowance is counted per org, so no org means it cannot be applied and the
// connect is refused. Without an allowance source wired, the feature gate's
// free-or-paid split stands in and the insert is not re-checked.
func (s *emailService) guardInboxLimit(ctx context.Context, orgID *uuid.UUID) (*models.MailboxAllowance, *errx.Error) {
	if orgID == nil {
		return nil, errx.ErrNoOrganization
	}
	if s.allowance != nil {
		a, xerr := s.allowance.MailboxAllowance(ctx, *orgID)
		if xerr != nil {
			return nil, xerr
		}
		if a.CanAdd(1) {
			return a, nil
		}
		return nil, errx.MailboxAllowanceReached(a.Used, *a.Allowance, a.Paid)
	}
	if s.featureGate == nil {
		return nil, nil
	}
	count, xerr := s.emailRepository.CountForOrganization(ctx, *orgID)
	if xerr != nil {
		return nil, xerr
	}
	allowed, xerr := s.featureGate.CanAddInbox(ctx, *orgID, count)
	if xerr != nil {
		return nil, xerr
	}
	if allowed {
		return nil, nil
	}
	return nil, errx.MailboxAllowanceReached(count, models.FreeWorkspaceMailboxLimit, false)
}

// OAuthFinish validates the state, exchanges the code for tokens, fetches the
// inbox owner, and persists a new email account — or, when the state carries an
// account id (OAuthReauth), renews that mailbox's tokens in place instead.
func (s *emailService) OAuthFinish(ctx context.Context, userID, code, state string) (*models.Email, bool, *errx.Error) {
	if code = strings.TrimSpace(code); code == "" {
		return nil, false, errx.ErrEmailOnboardCode
	}
	if state = strings.TrimSpace(state); state == "" {
		return nil, false, errx.ErrEmailOnboardState
	}

	sess, xerr := s.takeOnboardingState(ctx, state)
	if xerr != nil {
		return nil, false, xerr
	}
	if sess.UserID != userID {
		return nil, false, errx.ErrEmailOnboardState
	}

	// A reauth adds no mailbox, so an org over its inbox cap can still fix one.
	var allowance *models.MailboxAllowance
	if sess.EmailAccountID == nil {
		a, xerr := s.guardInboxLimit(ctx, sess.OrganizationID)
		if xerr != nil {
			return nil, false, xerr
		}
		allowance = a
	}

	provider := models.InboxProvider(sess.Provider)
	cfg, xerr := s.oauthConfigFor(provider, sess.OAuthClient)
	if xerr != nil {
		return nil, false, xerr
	}

	tok, err := cfg.Exchange(ctx, code)
	if err != nil {
		return nil, false, errx.ErrEmailOnboardExchange
	}

	owner, xerr := fetchInboxOwner(ctx, provider, cfg, tok)
	if xerr != nil {
		return nil, false, xerr
	}

	if sess.EmailAccountID != nil {
		acc, xerr := s.finishReauth(ctx, sess, provider, tok, owner)
		return acc, true, xerr
	}

	if exists, xerr := s.emailRepository.ExistsForUser(ctx, userID, owner.Email); xerr != nil {
		return nil, false, xerr
	} else if exists {
		return nil, false, errx.ErrEmailOnboardAlreadyExists
	}

	name := strings.TrimSpace(owner.Name)
	if name == "" {
		name = deriveNameFromEmail(owner.Email)
	}

	acc, xerr := s.emailRepository.NewOauthAccount(ctx, userID, models.NewOauthAccount{
		OrganizationID: sess.OrganizationID,
		Allowance:      allowance,
		Provider:       provider,
		Name:           name,
		Email:          owner.Email,
		AccessToken:    tok.AccessToken,
		RefreshToken:   tok.RefreshToken,
		ExpiresAt:      tok.Expiry,
		OAuthClient:    s.resolveOAuthClient(provider, sess.OAuthClient),
	})
	if xerr == nil && acc != nil {
		s.syncWarmupPoolMembership(ctx, acc)
		s.publishAccountEvent(ctx, pubsub.EventAccountConnected, acc)
		s.dispatchAccountConnected(ctx, sess.OrganizationID, acc)
		// Assign a worker and load the mailbox so it starts sending/syncing
		// immediately; the reconciler is the fallback if this fails.
		s.loadAccountBestEffort(ctx, acc.ID)
	}
	return acc, false, xerr
}

// OnboardSMTPIMAP validates the supplied SMTP/IMAP credentials against a live worker, then
// persists the email account on success. Returns ErrEmailCredentials if the worker reports failure.
func (s *emailService) OnboardSMTPIMAP(ctx context.Context, userID string, orgID *uuid.UUID, data *models.NewSMTPIMAPAccount) (*models.Email, *errx.Error) {
	if xerr := validateSMTPIMAPInput(data); xerr != nil {
		return nil, xerr
	}

	allowance, xerr := s.guardInboxLimit(ctx, orgID)
	if xerr != nil {
		return nil, xerr
	}

	if exists, xerr := s.emailRepository.ExistsForUser(ctx, userID, data.Email); xerr != nil {
		return nil, xerr
	} else if exists {
		return nil, errx.ErrEmailOnboardAlreadyExists
	}

	if s.workerAssignment == nil {
		return nil, errx.ErrEmailOnboardNoWorker
	}

	// Pick any healthy worker for the one-shot validation handshake. Tier is
	// irrelevant here (nothing is placed yet, the worker just dials the
	// credentials once), so fall back to the other tier rather than failing:
	// asking only for free-tier workers made onboarding impossible on any
	// deployment whose workers all register as premium, which includes a stock
	// self-host install.
	w, werr := s.workerAssignment.SelectSharedWorker(ctx, false)
	if werr != nil || w == nil {
		w, werr = s.workerAssignment.SelectSharedWorker(ctx, true)
	}
	if werr != nil || w == nil {
		return nil, errx.ErrEmailOnboardNoWorker
	}

	creds := &models.SmtpImap{SMTP: data.SMTP, IMAP: data.IMAP}
	if xerr := s.ValidateCredentials(ctx, *orgID, w.ID.String(), creds); xerr != nil {
		return nil, xerr
	}

	data.OrganizationID = orgID
	data.Allowance = allowance

	acc, xerr := s.emailRepository.NewSMTPIMAPAccount(ctx, userID, *data)
	if xerr != nil {
		return nil, xerr
	}

	// Assign the long-term worker (free vs paid tier). Failure here is non-fatal:
	// the scheduler will pick the account up on its next pass.
	if orgID != nil {
		if _, err := s.workerAssignment.AssignWorkerToEmail(ctx, acc.ID, *orgID); err != nil {
			errs.CaptureException(err)
		}
	}

	s.syncWarmupPoolMembership(ctx, acc)
	s.publishAccountEvent(ctx, pubsub.EventAccountConnected, acc)
	s.dispatchAccountConnected(ctx, orgID, acc)
	// Load the mailbox onto its assigned worker so it starts sending/syncing
	// immediately; the reconciler is the fallback if this fails.
	s.loadAccountBestEffort(ctx, acc.ID)
	return acc, nil
}

// dispatchAccountConnected fires an email_account.connected webhook event
// to any subscribed endpoints. Failures here are best-effort and never
// block the onboarding flow.
func (s *emailService) dispatchAccountConnected(ctx context.Context, orgID *uuid.UUID, acc *models.Email) {
	if s.webhookService == nil || orgID == nil || acc == nil {
		return
	}
	payload := map[string]any{
		"email_account_id": acc.ID,
		"email":            acc.Email,
		"provider":         acc.Provider,
		"name":             acc.Name,
		"created_at":       acc.CreatedAt,
	}
	if _, err := s.webhookService.Dispatch(ctx, *orgID, models.WebhookEventEmailAccountConnected, payload); err != nil {
		errs.CaptureException(err)
	}
}

// oauthConfigured reports whether an OAuth client is actually usable, i.e. both
// halves of the credential are present.
func oauthConfigured(cfg *oauth2.Config) bool {
	return cfg != nil && cfg.ClientID != "" && cfg.ClientSecret != ""
}

// publicClientConfigured is oauthConfigured for a public (desktop-type)
// client, which has an id but may have no secret.
func publicClientConfigured(cfg *oauth2.Config) bool {
	return cfg != nil && cfg.ClientID != ""
}

// normalizeOAuthClient maps an empty or foreign client name to the default.
func normalizeOAuthClient(provider models.InboxProvider, client string) string {
	if provider == models.InboxProviderGoogle && client == models.OAuthClientGoogleDesktop {
		return client
	}
	if provider == models.InboxProviderOutlook && client == models.OAuthClientOutlookDesktop {
		return client
	}
	return models.OAuthClientDefault
}

// resolveOAuthClient is normalizeOAuthClient plus one deployment rule: when no
// web Gmail client is configured but a desktop-type one is, the desktop client
// IS the Gmail path, so "default" resolves to it and tokens are recorded as its.
func (s *emailService) resolveOAuthClient(provider models.InboxProvider, client string) string {
	client = normalizeOAuthClient(provider, client)
	if client != models.OAuthClientDefault || s.oauthInbox == nil {
		return client
	}
	switch provider {
	case models.InboxProviderGoogle:
		if !oauthConfigured(s.oauthInbox.Google) && oauthConfigured(s.oauthInbox.GoogleDesktop) {
			return models.OAuthClientGoogleDesktop
		}
	case models.InboxProviderOutlook:
		if !oauthConfigured(s.oauthInbox.Outlook) && publicClientConfigured(s.oauthInbox.OutlookDesktop) {
			return models.OAuthClientOutlookDesktop
		}
	}
	return client
}

func (s *emailService) oauthConfigFor(provider models.InboxProvider, client string) (*oauth2.Config, *errx.Error) {
	// LoadOauth2Inbox always returns a config, populated with empty strings when
	// the variables are unset, so the credentials themselves are what decides
	// whether the provider is actually available here.
	switch provider {
	case models.InboxProviderGoogle:
		if client == models.OAuthClientGoogleDesktop {
			if s.oauthInbox == nil || !oauthConfigured(s.oauthInbox.GoogleDesktop) {
				return nil, errx.ErrEmailOnboardGoogleNotConfigured
			}
			return s.oauthInbox.GoogleDesktop, nil
		}
		if s.oauthInbox == nil || !oauthConfigured(s.oauthInbox.Google) {
			return nil, errx.ErrEmailOnboardGoogleNotConfigured
		}
		return s.oauthInbox.Google, nil
	case models.InboxProviderOutlook:
		if client == models.OAuthClientOutlookDesktop {
			if s.oauthInbox == nil || !publicClientConfigured(s.oauthInbox.OutlookDesktop) {
				return nil, errx.ErrEmailOnboardOutlookNotConfigured
			}
			return s.oauthInbox.OutlookDesktop, nil
		}
		if s.oauthInbox == nil || !oauthConfigured(s.oauthInbox.Outlook) {
			return nil, errx.ErrEmailOnboardOutlookNotConfigured
		}
		return s.oauthInbox.Outlook, nil
	default:
		return nil, errx.ErrEmailOnboardProvider
	}
}

func validateSMTPIMAPInput(data *models.NewSMTPIMAPAccount) *errx.Error {
	if data == nil || data.SMTP == nil || data.IMAP == nil {
		return errx.ErrEmailCredentialsRequired
	}
	data.Email = strings.TrimSpace(data.Email)
	if _, err := mail.ParseAddress(data.Email); err != nil {
		return errx.ErrEmail
	}
	if !validNameLen(&data.Name) {
		return errx.ErrEmailName
	}
	if strings.TrimSpace(data.SMTP.Host) == "" {
		return errx.ErrEmailSMTPHost
	}
	if !validPort(data.SMTP.Port) {
		return errx.ErrEmailSMTPPort
	}
	if strings.TrimSpace(data.IMAP.Host) == "" {
		return errx.ErrEmailIMAPHost
	}
	if !validPort(data.IMAP.Port) {
		return errx.ErrEmailIMAPPort
	}
	return validateMailSecurity(data.SMTP, data.IMAP)
}

// validPort accepts any routable TCP port. Mail submission is conventionally
// 465/587 and IMAP 993/143, but plenty of providers and self-hosted servers
// use 2525, 25, or something else entirely, and the security mode (not the
// port) is what decides how we connect.
func validPort(port int) bool {
	return port > 0 && port <= 65535
}

// validateMailSecurity rejects an unknown security mode. Empty is allowed and
// means "infer from the port", which is how existing clients behave.
//
// "none" carries two extra conditions, because it is the one mode that puts a
// password on an unencrypted socket. It is legal only against a loopback host,
// where the socket never reaches a wire, and only on a self-hosted instance,
// where the worker runs on the operator's own machine. On the hosted product
// the worker is never the customer's machine, so a loopback address there is
// the WORKER's loopback: the mode could not reach the relay it was meant for
// and would only be a way to speak plaintext to whatever answers on that port.
func validateMailSecurity(smtp, imap *models.Service) *errx.Error {
	if smtp.Security != "" && !models.ValidMailSecurity(smtp.Security) {
		return errx.ErrEmailSMTPSecurity
	}
	if imap.Security != "" && !models.ValidMailSecurity(imap.Security) {
		return errx.ErrEmailIMAPSecurity
	}
	if err := validateCleartextHost(smtp.Security, smtp.Host, errx.ErrEmailSMTPSecurityNotLocal, errx.ErrEmailSMTPSecurityHosted); err != nil {
		return err
	}
	return validateCleartextHost(imap.Security, imap.Host, errx.ErrEmailIMAPSecurityNotLocal, errx.ErrEmailIMAPSecurityHosted)
}

// validateCleartextHost is the "none" gate for one leg.
func validateCleartextHost(security, host string, notLocal, hosted *errx.Error) *errx.Error {
	if security != models.MailSecurityNone {
		return nil
	}
	if !config.SelfHosted() {
		return hosted
	}
	if !models.LoopbackMailHost(host) {
		return notLocal
	}
	return nil
}

func validNameLen(name *string) bool {
	*name = strings.TrimSpace(*name)
	if *name == "" {
		return false
	}
	r := []rune(*name)
	return len(r) >= 2 && len(r) <= 100
}

func deriveNameFromEmail(email string) string {
	at := strings.IndexByte(email, '@')
	if at <= 0 {
		return email
	}
	local := email[:at]
	if local == "" {
		return email
	}
	local = strings.ReplaceAll(local, ".", " ")
	local = strings.ReplaceAll(local, "_", " ")
	return strings.Title(local)
}

// inboxOwner is the per-provider user info shape we normalize on. Name is
// the account's real name ("First Last") when the provider gives one, and
// empty otherwise; the callers fall back to the address's local part only
// then.
type inboxOwner struct {
	Email string
	Name  string
}

// fullName joins the provider's given and family names, falling back to the
// display name when only that is set. Trimmed so a provider that leaves one
// half empty yields "First" rather than "First ".
func fullName(given, family, display string) string {
	name := strings.TrimSpace(strings.TrimSpace(given) + " " + strings.TrimSpace(family))
	if name == "" {
		name = strings.TrimSpace(display)
	}
	return name
}

// isFallbackName reports whether a stored mailbox name is one the connect
// flow derived from the address rather than one the provider or the user
// gave, so a reauth may replace it with the real name.
func isFallbackName(name, email string) bool {
	name = strings.TrimSpace(name)
	return name == "" || strings.EqualFold(name, email) || strings.EqualFold(name, deriveNameFromEmail(email))
}

// fetchInboxOwner resolves the address and real name behind a fresh consent.
// cfg is the client the consent ran against: for Microsoft it decides whether
// the token can reach Graph at all.
func fetchInboxOwner(ctx context.Context, provider models.InboxProvider, cfg *oauth2.Config, tok *oauth2.Token) (*inboxOwner, *errx.Error) {
	if tok == nil {
		return nil, errx.ErrEmailOnboardUserInfo
	}
	switch provider {
	case models.InboxProviderGoogle:
		return fetchGmailOwner(ctx, tok.AccessToken)
	case models.InboxProviderOutlook:
		if config.OutlookGraphScoped(cfg) {
			return fetchOutlookOwner(ctx, tok.AccessToken)
		}
		return fetchOutlookOIDCOwner(ctx, cfg, tok)
	default:
		return nil, errx.ErrEmailOnboardProvider
	}
}

var httpClient = &http.Client{Timeout: 10 * time.Second}

// Provider identity endpoints. Variables so tests can point them at a local
// server.
var (
	googleUserinfoURL    = "https://www.googleapis.com/oauth2/v3/userinfo"
	gmailProfileURL      = "https://gmail.googleapis.com/gmail/v1/users/me/profile"
	graphMeURL           = "https://graph.microsoft.com/v1.0/me"
	microsoftUserinfoURL = "https://graph.microsoft.com/oidc/userinfo"
)

// getJSON performs a bearer-authenticated GET and decodes the body into out.
// Any transport error, non-200 status or undecodable body is reported as the
// one onboarding error the caller can show.
func getJSON(ctx context.Context, url, token string, out any) *errx.Error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return errx.ErrEmailOnboardUserInfo
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return errx.ErrEmailOnboardUserInfo
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return errx.ErrEmailOnboardUserInfo
	}
	if err := json.Unmarshal(body, out); err != nil {
		return errx.ErrEmailOnboardUserInfo
	}
	return nil
}

// fetchGmailOwner reads the account's address and real name from Google's
// OpenID userinfo endpoint (openid email profile), which is where the given
// and family names live; the Gmail profile endpoint has only the address.
// The profile endpoint stays as the fallback for a consent that predates the
// identity scopes, so an older client can still connect, just without a name.
func fetchGmailOwner(ctx context.Context, token string) (*inboxOwner, *errx.Error) {
	var info struct {
		Email      string `json:"email"`
		GivenName  string `json:"given_name"`
		FamilyName string `json:"family_name"`
		Name       string `json:"name"`
	}
	if xerr := getJSON(ctx, googleUserinfoURL, token, &info); xerr == nil && info.Email != "" {
		return &inboxOwner{Email: info.Email, Name: fullName(info.GivenName, info.FamilyName, info.Name)}, nil
	}

	var profile struct {
		EmailAddress string `json:"emailAddress"`
	}
	if xerr := getJSON(ctx, gmailProfileURL, token, &profile); xerr != nil {
		return nil, xerr
	}
	if profile.EmailAddress == "" {
		return nil, errx.ErrEmailOnboardUserInfo
	}
	return &inboxOwner{Email: profile.EmailAddress}, nil
}

// fetchOutlookOwner resolves the owner through Graph /me, for a consent that
// carries a Graph scope. givenName and surname are the real name; displayName
// is the fallback, since a tenant can leave the split fields empty.
func fetchOutlookOwner(ctx context.Context, token string) (*inboxOwner, *errx.Error) {
	var out struct {
		Mail              string `json:"mail"`
		UserPrincipalName string `json:"userPrincipalName"`
		DisplayName       string `json:"displayName"`
		GivenName         string `json:"givenName"`
		Surname           string `json:"surname"`
	}
	if xerr := getJSON(ctx, graphMeURL, token, &out); xerr != nil {
		return nil, xerr
	}
	addr := out.Mail
	if addr == "" {
		addr = out.UserPrincipalName
	}
	if addr == "" {
		return nil, errx.ErrEmailOnboardUserInfo
	}
	return &inboxOwner{Email: addr, Name: fullName(out.GivenName, out.Surname, out.DisplayName)}, nil
}

// oidcClaims are the identity claims both the id_token and the OIDC userinfo
// endpoint can carry.
type oidcClaims struct {
	Email             string `json:"email"`
	PreferredUsername string `json:"preferred_username"`
	GivenName         string `json:"given_name"`
	FamilyName        string `json:"family_name"`
	Name              string `json:"name"`
}

func (c oidcClaims) address() string {
	if c.Email != "" {
		return c.Email
	}
	// Entra's preferred_username is the UPN, which is the address on every
	// mailbox account (a guest's would carry #EXT#, and cannot be a mailbox).
	if strings.Contains(c.PreferredUsername, "@") && !strings.Contains(c.PreferredUsername, "#EXT#") {
		return c.PreferredUsername
	}
	return ""
}

// fetchOutlookOIDCOwner resolves the owner for a consent with no Graph scope
// (the IMAP/SMTP resource): the access token cannot reach /me, so the id_token
// the token endpoint returned alongside it is read first, and the OIDC
// userinfo endpoint fills in the split name Entra leaves out of the id_token
// by default. That endpoint wants a token for itself, which the refresh token
// buys with the identity scopes alone, one request, no new consent.
func fetchOutlookOIDCOwner(ctx context.Context, cfg *oauth2.Config, tok *oauth2.Token) (*inboxOwner, *errx.Error) {
	claims := idTokenClaims(tok)
	owner := &inboxOwner{Email: claims.address(), Name: fullName(claims.GivenName, claims.FamilyName, "")}

	if (owner.Email == "" || owner.Name == "") && cfg != nil && tok.RefreshToken != "" {
		if infoTok := userinfoToken(ctx, cfg, tok.RefreshToken); infoTok != "" {
			var info oidcClaims
			if xerr := getJSON(ctx, microsoftUserinfoURL, infoTok, &info); xerr == nil {
				if owner.Email == "" {
					owner.Email = info.address()
				}
				if owner.Name == "" {
					owner.Name = fullName(info.GivenName, info.FamilyName, info.Name)
				}
			}
		}
	}
	if owner.Name == "" {
		owner.Name = strings.TrimSpace(claims.Name)
	}
	if owner.Email == "" {
		return nil, errx.ErrEmailOnboardUserInfo
	}
	return owner, nil
}

// idTokenClaims decodes the id_token's payload. The token arrived over TLS
// straight from the issuer's token endpoint in the same response as the
// access token, so its signature adds nothing here and is not checked.
func idTokenClaims(tok *oauth2.Token) oidcClaims {
	var claims oidcClaims
	raw, _ := tok.Extra("id_token").(string)
	parts := strings.Split(raw, ".")
	if len(parts) < 2 {
		return claims
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(parts[1], "="))
	if err != nil {
		return claims
	}
	_ = json.Unmarshal(payload, &claims)
	return claims
}

// userinfoToken redeems the refresh token for an access token scoped to the
// identity claims only, which Entra issues for its OIDC userinfo endpoint.
// Empty on any failure; the caller has the id_token to fall back on.
func userinfoToken(ctx context.Context, cfg *oauth2.Config, refreshToken string) string {
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {cfg.ClientID},
		"scope":         {"openid email profile"},
	}
	if cfg.ClientSecret != "" {
		form.Set("client_secret", cfg.ClientSecret)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.Endpoint.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return ""
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := httpClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	var out struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return ""
	}
	return out.AccessToken
}
