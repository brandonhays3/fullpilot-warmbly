package wmail

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/warmbly/warmbly/internal/app/cipher"
	"github.com/warmbly/warmbly/internal/client/goog"
	"github.com/warmbly/warmbly/internal/client/msgraph"
	"github.com/warmbly/warmbly/internal/client/smtpimap/imap"
	"github.com/warmbly/warmbly/internal/client/smtpimap/smtp"
	"github.com/warmbly/warmbly/internal/config"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/infrastructure/cache"
	"github.com/warmbly/warmbly/internal/infrastructure/storage"
	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/pkg/stoken"
	"github.com/warmbly/warmbly/internal/repository"
	"golang.org/x/oauth2"
)

type GoogleService struct {
	Token    *oauth2.Token
	svc      *goog.Client
	OnUpdate func(token *oauth2.Token)
}

type OutlookService struct {
	Token    *oauth2.Token
	OnUpdate func(token *oauth2.Token)
}

type GoogleData struct {
	Client        *goog.Client
	LastHistoryID uint64
}

type GraphData struct {
	Client *msgraph.Client
}

type SmtpImapData struct {
	ImapClient ImapConn
	SmtpClient *smtp.Client
	Mailboxes  []*models.Mailbox
	// mailbox is the UIDVALIDITY of the folder currently being walked: the
	// generation the UIDs stamped on stored messages belong to, not the
	// folder's identity.
	mailbox uint32
	// folderPath is that folder's name, which IS its identity, and folder the
	// canonical folder it maps to. Both are set alongside mailbox and stamped
	// on every stored or updated message.
	folderPath string
	folder     string
	// overflowReported keeps the "more folders than we follow" warning to
	// one per worker session; the condition is static until the user
	// reorganizes their mail.
	overflowReported bool
}

type WMail struct {
	UserID uuid.UUID
	ID     uuid.UUID
	// OrgID scopes the organization-wide sync budget; nil for a legacy
	// personal mailbox.
	OrgID *uuid.UUID

	Email          string
	FirstName      string
	LastName       string
	SignaturePlain string
	SignatureHTML  string

	// SaveToSent files a copy of every outbound message in the mailbox's Sent
	// folder. SMTP/IMAP only: Gmail and Graph file their own copy, so acting on
	// it there would duplicate every sent message.
	SaveToSent bool

	EmailType models.InboxProvider
	// Transport is how an OAuth mailbox is driven: the provider API
	// (GoogleData / GraphData) or the provider's IMAP and SMTP endpoints
	// with the same token (SmtpImapData). Always SMTP for smtp_imap.
	Transport models.MailTransport

	GoogleData   *GoogleData
	GraphData    *GraphData
	SmtpImapData *SmtpImapData

	Cache                     *cache.Cache
	Storage                   storage.Store
	EmailMessageMapRepository repository.EmailMessageMapRepository
	SyncContext               repository.SyncContextRepository
	CipherService             cipher.CipherService

	// Sync fair use: the budget engine and the relayed state (governor.go,
	// sync_state.go). Built in NewWMail from the ADD_EMAIL payload.
	gov     syncBudget
	tracker *syncTracker
	// laneCache remembers deferred messages' lanes across passes; googleTick
	// and graphTick carry the running pass's stats into provider callbacks.
	laneCache  laneCache
	googleTick *tickStats
	graphTick  *tickStats
	// flagScan is the previous flag snapshot per folder name, used only on
	// IMAP servers without CONDSTORE, which cannot say what changed.
	flagScan map[string]*folderFlagScan
	// transportFailures counts consecutive passes that could not reach the
	// mail server, which paces the retry and keeps one outage to one warning.
	transportFailures int

	Ctx           context.Context
	Cancel        context.CancelFunc
	TerminateFunc func()

	onEvent func(jobType models.JobEventType, body any) error
}

func NewWMail(
	data *models.AddWorkerEmail,
	OnEvent func(eventType models.JobEventType, key string, body any) error,
	terminate func(),
	cache *cache.Cache, storage storage.Store,
	emailMessageMapRepository repository.EmailMessageMapRepository,
	syncContext repository.SyncContextRepository,
	cipherService cipher.CipherService,
) (*WMail, *errx.MailError) {
	// Use background context so the WMail outlives the AddEmail request handler.
	mailCtx, cancel := context.WithCancel(context.Background())

	mail := &WMail{
		ID:        data.ID,
		UserID:    data.UserID,
		OrgID:     data.OrganizationID,
		Email:     data.Email,
		FirstName: data.FirstName,
		LastName:  data.LastName,
		EmailType: data.Type,
		Transport: data.MailTransport(),
		// Unset in the payload means yes; see AddWorkerEmail.SavesSentCopy.
		// Only a plain smtp_imap mailbox needs the copy: Gmail and Exchange
		// Online file their own for SMTP submissions too, so an OAuth mailbox
		// on the smtp transport must not APPEND a second one.
		SaveToSent: data.Type == models.InboxProviderSMTPIMAP && data.SavesSentCopy(),
		onEvent: func(jobType models.JobEventType, body any) error {
			return OnEvent(jobType, data.ID.String(), body)
		},

		Ctx:           mailCtx,
		Cancel:        cancel,
		TerminateFunc: terminate,

		Cache:                     cache,
		Storage:                   storage,
		EmailMessageMapRepository: emailMessageMapRepository,
		SyncContext:               syncContext,
		CipherService:             cipherService,
	}

	// A publisher older than the sync policy sends no Sync block; compiled
	// defaults then apply and the mailbox starts its backfill from scratch.
	var seed *models.SyncState
	var policy models.SyncPolicy
	if data.Sync != nil {
		policy = data.Sync.Policy
		seed = data.Sync.State
	}
	// A nil *cache.Cache must not become a non-nil interface holding nil.
	var gcache redisCmdable
	if cache != nil {
		gcache = cache
	}
	mail.gov = newGovernor(data.ID, data.OrganizationID, gcache, policy)
	mail.tracker = newSyncTracker(seed, func(st models.SyncState) error {
		return mail.onEvent(models.JobEventTypeSyncState, &models.JobEventSyncState{
			UserID:  mail.UserID,
			EmailID: mail.ID,
			State:   st,
		})
	})

	// An OAuth mailbox on the smtp transport is driven like an smtp_imap one,
	// with the provider's endpoints and the OAuth token in place of a
	// password. A brokered (cloud-managed) token is minted for the provider
	// API and cannot authenticate IMAP, so those stay on the API.
	if data.Type != models.InboxProviderSMTPIMAP && data.UsesSmtpImap() {
		if data.TokenSource != nil {
			log.Warn().Str("email_id", data.ID.String()).Msg("brokered mailbox cannot use the smtp transport; using the provider API")
			mail.Transport = models.MailTransportAPI
		} else {
			if err := mail.initOAuthSmtpImap(mailCtx, data); err != nil {
				return nil, err
			}
			return mail, nil
		}
	}

	switch data.Type {
	case models.InboxProviderGoogle:
		if data.Google == nil {
			return nil, errx.MError(
				errx.MailErrorCritical,
				errx.MailErrorCodeAuthenticationFailed,
				"missing Google credentials in add-email payload",
				errx.MailErrorResolveMethodReload,
			)
		}
		mail.GoogleData = &GoogleData{
			Client: &goog.Client{
				Email:     data.Email,
				FirstName: data.FirstName,
				LastName:  data.LastName,

				Cache:           mail.Cache,
				OnMessageAdded:  mail.onGoogleMessageAdded,
				OnMessageRemove: mail.onGoogleMessageRemove,
				OnLabelAdd:      mail.onGoogleMessageLabelsAdded,
				OnLabelRemove:   mail.onGoogleMessageLabelsRemoved,
				// Without this a refreshed Gmail token is never persisted, so
				// the mailbox stops working roughly an hour after connect.
				OnTokenRefresh: func(_ context.Context, t *oauth2.Token) error {
					return mail.onTokenUpdate(t)
				},
			},
			LastHistoryID: data.Google.LastHistoryID,
		}

		if data.TokenSource != nil {
			// Brokered: the cloud refreshes; there is nothing to persist here.
			mail.GoogleData.Client.OnTokenRefresh = nil
			if err := mail.GoogleData.Client.InitWithSource(mailCtx, data.TokenSource); err != nil {
				return nil, err
			}
		} else if err := mail.GoogleData.Client.Init(mailCtx, data.Google.Token, data.Cfg); err != nil {
			return nil, err
		}
	case models.InboxProviderOutlook:
		// Microsoft/Outlook mailboxes run entirely on Microsoft Graph: RAW MIME
		// sendMail plus delta-based inbound sync. There is no IMAP/SMTP path.
		if data.Graph == nil {
			return nil, errx.MError(
				errx.MailErrorCritical,
				errx.MailErrorCodeAuthenticationFailed,
				"missing Microsoft Graph credentials in add-email payload",
				errx.MailErrorResolveMethodReload,
			)
		}
		token := data.Graph.Token
		deltaLinks := data.Graph.DeltaLinks
		var syncSince time.Time
		if policy.SyncSince != nil {
			syncSince = *policy.SyncSince
		}

		mail.GraphData = &GraphData{
			Client: &msgraph.Client{
				Email:     data.Email,
				FirstName: data.FirstName,
				LastName:  data.LastName,

				Cache:      mail.Cache,
				DeltaLinks: cloneStringMap(deltaLinks),
				SyncSince:  syncSince,

				OnMessageSeen:   mail.onGraphMessageSeen,
				OnMessageRemove: mail.onGraphMessageRemove,
				OnDelta:         mail.onGraphDelta,
				OnTokenRefresh: func(_ context.Context, t *oauth2.Token) error {
					return mail.onTokenUpdate(t)
				},
			},
		}

		if data.TokenSource != nil {
			mail.GraphData.Client.OnTokenRefresh = nil
			if err := mail.GraphData.Client.InitWithSource(mailCtx, data.TokenSource); err != nil {
				return nil, err
			}
		} else if err := mail.GraphData.Client.Init(mailCtx, token, data.Cfg); err != nil {
			return nil, err
		}
	case models.InboxProviderSMTPIMAP:
		// Generic SMTP/IMAP with plain-auth credentials (arbitrary providers).
		if data.SmtpImap == nil || data.SmtpImap.Credentials == nil {
			return nil, errx.MError(
				errx.MailErrorCritical,
				errx.MailErrorCodeInvalidCredentials,
				"missing SMTP/IMAP credentials in add-email payload",
				errx.MailErrorResolveMethodReload,
			)
		}
		mail.SmtpImapData = &SmtpImapData{}

		if data.ImapSync {
			conn := &imap.Client{
				Email:       data.Email,
				AuthType:    models.AuthPlain,
				Credentials: data.SmtpImap.Credentials.IMAP,
			}
			if err := conn.Connect(); err != nil {
				return nil, err
			}
			mail.SmtpImapData.ImapClient = conn
			// Saved folder cursors: live sync resumes from each folder's stored
			// HIGHESTMODSEQ instead of re-baselining (and, before this, instead
			// of re-walking every folder on every worker restart).
			for i := range data.SmtpImap.Mailboxes {
				box := data.SmtpImap.Mailboxes[i]
				mail.SmtpImapData.Mailboxes = append(mail.SmtpImapData.Mailboxes, &box)
			}
		}

		mail.SmtpImapData.SmtpClient = &smtp.Client{
			FirstName:   data.FirstName,
			LastName:    data.LastName,
			Email:       data.Email,
			AuthType:    models.AuthPlain,
			Credentials: data.SmtpImap.Credentials.SMTP,
		}
	default:
		return nil, errx.MError(
			errx.MailErrorCritical,
			errx.MailErrorCodeUnsupported,
			"Unsupported email provider",
			errx.MailErrorResolveMethodReload,
		)
	}

	return mail, nil
}

// UsesSmtpImap reports whether sends and syncs go through SmtpImapData.
func (w *WMail) UsesSmtpImap() bool {
	return w.EmailType == models.InboxProviderSMTPIMAP || w.Transport == models.MailTransportSMTP
}

// providerSmtpImap is the IMAP and SMTP submission endpoint pair for an OAuth
// provider.
func providerSmtpImap(t models.InboxProvider) (smtpHost string, smtpPort int, imapHost string, imapPort int, ok bool) {
	switch t {
	case models.InboxProviderGoogle:
		return config.GmailSMTPHost, config.GmailSMTPPort, config.GmailIMAPHost, config.GmailIMAPPort, true
	case models.InboxProviderOutlook:
		return config.OutlookSMTPHost, config.OutlookSMTPPort, config.OutlookIMAPHost, config.OutlookIMAPPort, true
	}
	return "", 0, "", 0, false
}

// initOAuthSmtpImap builds the IMAP and SMTP clients for an OAuth mailbox on
// the smtp transport. One refreshing token source serves both, and every
// refresh is relayed to the control plane exactly as the API clients do, so
// the stored credential stays current whichever transport the mailbox is on.
func (w *WMail) initOAuthSmtpImap(ctx context.Context, data *models.AddWorkerEmail) *errx.MailError {
	var token *oauth2.Token
	switch data.Type {
	case models.InboxProviderGoogle:
		if data.Google != nil {
			token = data.Google.Token
		}
	case models.InboxProviderOutlook:
		if data.Graph != nil {
			token = data.Graph.Token
		}
	}
	smtpHost, smtpPort, imapHost, imapPort, ok := providerSmtpImap(data.Type)
	if token == nil || !ok {
		return errx.MError(
			errx.MailErrorCritical,
			errx.MailErrorCodeAuthenticationFailed,
			"missing OAuth credentials in add-email payload",
			errx.MailErrorResolveMethodReload,
		)
	}

	var ts oauth2.TokenSource = oauth2.ReuseTokenSource(token, data.Cfg.TokenSource(ctx, token))
	ts = stoken.New(ts, func(t *oauth2.Token) error {
		return w.onTokenUpdate(t)
	})

	w.SmtpImapData = &SmtpImapData{}
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

	w.SmtpImapData.SmtpClient = &smtp.Client{
		FirstName: data.FirstName,
		LastName:  data.LastName,
		Email:     data.Email,
		AuthType:  models.AuthOAuth2,
		Oauth2:    &models.Oauth2Service{Host: smtpHost, Port: smtpPort, Token: ts},
	}
	return nil
}

// ApplySyncPolicy takes the budget from a republished ADD_EMAIL for a mailbox
// that is already loaded, so an operator's settings change lands within the
// reconciler's republish interval instead of at the next worker restart. The
// backfill window already fixed by a running import is deliberately not moved.
func (w *WMail) ApplySyncPolicy(data *models.AddWorkerEmailSyncData) {
	if data == nil || w.gov == nil {
		return
	}
	w.gov.SetPolicy(data.Policy)
}
