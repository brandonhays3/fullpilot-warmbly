package email

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/infrastructure/instantly"
	"github.com/warmbly/warmbly/internal/models"
)

// InstantlyWarmup is the answer to GET /emails/:id/warmup/instantly: whether
// the integration is on, whether this mailbox is warmed there, and what
// Instantly reports for it.
type InstantlyWarmup struct {
	Configured   bool                    `json:"configured"`
	Enrolled     bool                    `json:"enrolled"`
	Found        bool                    `json:"found"`
	DashboardURL string                  `json:"dashboard_url"`
	Status       *instantly.WarmupStatus `json:"status,omitempty"`
}

// WireInstantly attaches the Instantly client. A nil client keeps Warmbly's
// own warmup in charge, so callers can pass whatever NewFromEnv returned.
func (s *emailService) WireInstantly(c *instantly.Client) {
	s.instantly = c
}

func (s *emailService) instantlyEnabled() bool {
	return s.instantly.Enabled()
}

// InstantlyWarmupStatus reads the Instantly side of a mailbox's warmup.
func (s *emailService) InstantlyWarmupStatus(ctx context.Context, orgID, emailAccountID string) (*InstantlyWarmup, *errx.Error) {
	account, xerr := s.emailRepository.Get(ctx, orgID, emailAccountID)
	if xerr != nil {
		return nil, xerr
	}
	out := &InstantlyWarmup{
		Configured:   s.instantlyEnabled(),
		Enrolled:     account.WarmsViaInstantly(),
		DashboardURL: instantly.DashboardAccountsURL,
	}
	if !out.Configured {
		return out, nil
	}
	st, err := s.instantly.GetWarmupStatus(ctx, account.Email)
	switch {
	case err == nil:
		out.Found = true
		out.Status = st
	case instantly.IsNotFound(err):
	default:
		return nil, instantlyError(account, "could not read the mailbox's warmup status from Instantly", err)
	}
	return out, nil
}

// ownedAccount loads a mailbox and checks the caller owns it, the same
// predicate the lifecycle UPDATE runs under, so nothing reaches Instantly
// for a mailbox the caller could not change anyway.
func (s *emailService) ownedAccount(ctx context.Context, userID, emailAccountID string) (*models.Email, *errx.Error) {
	id, err := uuid.Parse(emailAccountID)
	if err != nil {
		return nil, errx.ErrUuid
	}
	account, xerr := s.emailRepository.GetByID(ctx, id)
	if xerr != nil {
		return nil, xerr
	}
	if account.UserID != userID {
		return nil, errx.ErrNotFound
	}
	return account, nil
}

// instantlyTransition runs the Instantly half of a warmup lifecycle change
// before the local row moves, and answers which warmup_provider the row
// should record afterwards ("" leaves it as it is).
//
// With the integration on, start and resume enroll the mailbox in Instantly
// and hand warmup to it; pause and stop pause it there. With the integration
// off, a start or resume takes the mailbox back to Warmbly's own warmup so a
// removed key never leaves a mailbox that nothing warms.
func (s *emailService) instantlyTransition(ctx context.Context, userID, emailAccountID, action string) (string, *errx.Error) {
	starting := action == "start" || action == "resume"
	if !s.instantlyEnabled() {
		if starting {
			return models.WarmupProviderInternal, nil
		}
		return "", nil
	}
	current, xerr := s.ownedAccount(ctx, userID, emailAccountID)
	if xerr != nil {
		return "", xerr
	}
	switch action {
	case "start", "resume":
		if xerr := s.instantlyEnroll(ctx, current); xerr != nil {
			return "", xerr
		}
		return models.WarmupProviderInstantly, nil
	case "pause":
		if current.WarmsViaInstantly() {
			return "", s.instantlyPause(ctx, current)
		}
		return "", nil
	case "stop", "disable":
		if current.WarmsViaInstantly() {
			if xerr := s.instantlyPause(ctx, current); xerr != nil {
				return "", xerr
			}
		}
		return models.WarmupProviderInternal, nil
	}
	return "", nil
}

// recordWarmupProvider persists the provider a transition decided on and
// keeps the returned row in step with it.
func (s *emailService) recordWarmupProvider(ctx context.Context, account *models.Email, provider string) *errx.Error {
	if account == nil || provider == "" || account.WarmupProvider == provider {
		return nil
	}
	if xerr := s.emailRepository.SetWarmupProvider(ctx, account.ID, provider); xerr != nil {
		return xerr
	}
	account.WarmupProvider = provider
	return nil
}

// instantlyEnroll makes sure the mailbox exists in the Instantly workspace,
// applies the per-provider warmup profile and turns warmup on. An OAuth
// mailbox that is not there yet cannot be added over the API (Instantly's
// create endpoint needs IMAP and SMTP credentials), so that case comes back
// as ErrInstantlyAccountMissing for the dashboard to explain.
func (s *emailService) instantlyEnroll(ctx context.Context, account *models.Email) *errx.Error {
	client := s.instantly
	_, err := client.GetAccount(ctx, account.Email)
	switch {
	case err == nil:
	case instantly.IsNotFound(err):
		if account.Provider != string(models.InboxProviderSMTPIMAP) {
			return errx.ErrInstantlyAccountMissing
		}
		if xerr := s.instantlyCreate(ctx, account); xerr != nil {
			return xerr
		}
	default:
		return instantlyError(account, "could not look the mailbox up in Instantly", err)
	}

	settings := instantly.WarmupDefaults(account.Provider)
	if _, err := client.UpdateWarmupSettings(ctx, account.Email, settings); err != nil {
		return instantlyError(account, "could not apply the warmup settings in Instantly", err)
	}
	if _, err := client.EnableWarmup(ctx, account.Email); err != nil {
		return instantlyError(account, "could not start warmup in Instantly", err)
	}
	log.Info().Str("email_account_id", account.ID.String()).Str("provider", account.Provider).Msg("warmup handed to Instantly")
	return nil
}

// instantlyCreate adds an SMTP/IMAP mailbox to the workspace with its stored
// credentials. The decrypt goes through the repository, which is the only
// place that holds the credential key.
func (s *emailService) instantlyCreate(ctx context.Context, account *models.Email) *errx.Error {
	creds, xerr := s.emailRepository.GetSMTPCredentials(ctx, account.ID)
	if xerr != nil {
		return xerr
	}
	first, last := splitName(account.Name)
	if first == "" {
		first = strings.SplitN(account.Email, "@", 2)[0]
	}
	if last == "" {
		// Instantly requires both names; a lone first name gets its
		// domain as the surname rather than an empty string it rejects.
		last = strings.SplitN(account.Email, "@", 2)[0]
		if at := strings.Index(account.Email, "@"); at >= 0 {
			last = account.Email[at+1:]
		}
	}
	settings := instantly.WarmupDefaults(account.Provider)
	_, err := s.instantly.CreateAccount(ctx, instantly.CreateAccountInput{
		Email:        account.Email,
		FirstName:    first,
		LastName:     last,
		ProviderCode: instantly.ProviderCodeCustomIMAPSMTP,
		IMAPUsername: creds.IMAPUser,
		IMAPPassword: creds.IMAPPassword,
		IMAPHost:     creds.IMAPHost,
		IMAPPort:     creds.IMAPPort,
		SMTPUsername: creds.SMTPUser,
		SMTPPassword: creds.SMTPPassword,
		SMTPHost:     creds.SMTPHost,
		SMTPPort:     creds.SMTPPort,
		Warmup:       &settings,
	})
	if err != nil {
		return instantlyError(account, "could not add the mailbox to Instantly", err)
	}
	log.Info().Str("email_account_id", account.ID.String()).Msg("mailbox created in Instantly from its stored SMTP/IMAP credentials")
	return nil
}

// instantlyPause stops warmup in Instantly. A mailbox Instantly no longer
// knows has nothing running there, so that counts as paused.
func (s *emailService) instantlyPause(ctx context.Context, account *models.Email) *errx.Error {
	if _, err := s.instantly.PauseWarmup(ctx, account.Email); err != nil && !instantly.IsNotFound(err) {
		return instantlyError(account, "could not pause warmup in Instantly", err)
	}
	return nil
}

// instantlyError logs the upstream failure and answers a 503 that names it,
// so the dashboard can show the reason instead of a generic "couldn't update".
func instantlyError(account *models.Email, what string, err error) *errx.Error {
	var apiErr *instantly.APIError
	msg := what + "."
	if errors.As(err, &apiErr) && apiErr.Message != "" {
		msg = what + ": " + apiErr.Message
	}
	log.Warn().Err(err).Str("email_account_id", account.ID.String()).Msg(what)
	return errx.NewWithIdentifier(errx.ServiceUnavailable, "instantly_unavailable", msg)
}
