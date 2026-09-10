package wmail

import (
	"context"

	"github.com/google/uuid"
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
	// Transport is how an OAuth mailbox is SYNCED: over the provider API
	// (GoogleData / GraphData) or over its IMAP endpoint (SmtpImapData).
	// Always SMTP for smtp_imap. Sends do not follow it: an OAuth mailbox
	// holds both clients and each send picks one (sendTransport).
	Transport models.MailTransport
	// SendAPIPercent is the share of this mailbox's sends assigned to the
	// provider API rather than SMTP (SEND_TRANSPORT_API_PERCENT).
	SendAPIPercent int
	// SendOnly: this worker holds the mailbox for sending only (send-side
	// routing). No sync loop, no warmup actions, no IMAP session except the
	// one an smtp_imap mailbox files its Sent copy through.
	SendOnly bool

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
		ID:             data.ID,
		UserID:         data.UserID,
		OrgID:          data.OrganizationID,
		Email:          data.Email,
		FirstName:      data.FirstName,
		LastName:       data.LastName,
		EmailType:      data.Type,
		Transport:      data.MailTransport(),
		SendAPIPercent: config.SendTransportAPIPercent(),
		SendOnly:       data.SendOnly,
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

	switch data.Type {
	case models.InboxProviderGoogle:
		// Gmail API client plus SMTP (and IMAP when syncing over it) on one
		// token; see init_oauth.go.
		if err := mail.initGoogle(mailCtx, data); err != nil {
			return nil, err
		}
	case models.InboxProviderOutlook:
		// Graph client plus SMTP (and IMAP when syncing over it), each on
		// its own per-resource token; see init_oauth.go.
		if err := mail.initOutlook(mailCtx, data, policy); err != nil {
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

		// A send-only copy opens IMAP only to file the Sent copy; the owner
		// syncs. Everything else is the same client set.
		if data.ImapSync && (!data.SendOnly || mail.SaveToSent) {
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

// UsesSmtpImap reports whether the mailbox SYNCS over IMAP (and, for a plain
// smtp_imap mailbox, sends over SMTP). An OAuth mailbox's sends are decided
// per send by sendTransport, whatever this says.
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
