package models

import (
	"time"

	"github.com/google/uuid"
	"golang.org/x/oauth2"
)

// WorkerType represents the type of worker
type WorkerType string

const (
	WorkerTypeShared    WorkerType = "shared"
	WorkerTypeDedicated WorkerType = "dedicated"
)

// WorkerRiskPool buckets shared workers by acceptable mailbox risk level.
// Dedicated workers don't use it (one customer per worker — no
// cross-tenant contamination risk).
type WorkerRiskPool string

const (
	WorkerRiskPoolClean      WorkerRiskPool = "clean"
	WorkerRiskPoolRisky      WorkerRiskPool = "risky"
	WorkerRiskPoolQuarantine WorkerRiskPool = "quarantine"
)

// WorkerEgressKind describes how a worker is wired up to actually send mail.
// Different egress profiles ship with very different safe capacities, which
// is why the capacity view branches on this column to derive base_capacity.
type WorkerEgressKind string

const (
	WorkerEgressColdSMTP   WorkerEgressKind = "cold_smtp"
	WorkerEgressOAuthAPI   WorkerEgressKind = "oauth_api"
	WorkerEgressWarmupOnly WorkerEgressKind = "warmup_only"
)

// WorkerHealthState is the rolled-up health label maintained by the
// assignment loop. Authoritative for "can this worker accept new
// mailboxes" placement decisions. Mirrors the warmup health vocabulary
// but applies to whole workers, not per-mailbox warmup state.
type WorkerHealthState string

const (
	WorkerHealthHealthy     WorkerHealthState = "healthy"
	WorkerHealthWatch       WorkerHealthState = "watch"
	WorkerHealthThrottled   WorkerHealthState = "throttled"
	WorkerHealthQuarantined WorkerHealthState = "quarantined"
	WorkerHealthBlocked     WorkerHealthState = "blocked"
)

// WorkerDeployment is where a worker process runs, reported on every
// heartbeat from WORKER_DEPLOYMENT. It decides whether a worker is ephemeral:
// a Cloud Run job task lives a few minutes and gets a fresh egress IP per
// run, so SMTP/IMAP mailboxes are rotated across those; a VM worker is
// persistent and is the fallback when no ephemeral worker is live.
type WorkerDeployment string

const (
	WorkerDeploymentCloudRun WorkerDeployment = "cloud_run_worker"
	WorkerDeploymentCloudVM  WorkerDeployment = "cloud_vm"
	WorkerDeploymentCloudDev WorkerDeployment = "cloud_dev"
)

// IsEphemeral reports whether the worker is expected to exit within minutes.
// An unset deployment (an older build) is treated as persistent.
func (d WorkerDeployment) IsEphemeral() bool {
	return d == WorkerDeploymentCloudRun
}

type Worker struct {
	ID           uuid.UUID         `json:"id"`
	Name         string            `json:"name"`
	Notes        string            `json:"notes"`
	IPAddr       string            `json:"ip_addr"`
	Active       bool              `json:"active"`
	FreeTier     bool              `json:"free_tier"`
	WorkerType   WorkerType        `json:"worker_type"`
	AccountCount int               `json:"account_count"`
	RiskPool     WorkerRiskPool    `json:"risk_pool"`
	EgressKind   WorkerEgressKind  `json:"egress_kind"`
	HealthState  WorkerHealthState `json:"health_state"`
	LoadScore    float64           `json:"load_score"`
	// Deployment is the WORKER_DEPLOYMENT the worker reported; empty for a
	// build that predates it. StartedAt is when the current process booted.
	Deployment WorkerDeployment `json:"deployment"`
	StartedAt  *time.Time       `json:"started_at,omitempty"`

	// SSH management (none of these expose secret material — the encrypted
	// private key is fetched separately via GetWorkerSSHCredentials).
	SSHHost            string             `json:"ssh_host,omitempty"`
	SSHPort            int                `json:"ssh_port,omitempty"`
	SSHUser            string             `json:"ssh_user,omitempty"`
	SSHPublicKey       string             `json:"ssh_public_key,omitempty"`
	SSHHostFingerprint string             `json:"ssh_host_fingerprint,omitempty"`
	InstallState       WorkerInstallState `json:"install_state"`
	LastSeenAt         *time.Time         `json:"last_seen_at,omitempty"`
	LastError          string             `json:"last_error,omitempty"`

	// Profile assignment. Nil means "use backend env defaults".
	ProfileID       *uuid.UUID `json:"profile_id,omitempty"`
	ConfigAppliedAt *time.Time `json:"config_applied_at,omitempty"`

	// Image tag the worker is currently running, captured on every successful
	// Update. Used for the "v1.2.3 → v1.2.4" badge in the dashboard.
	ImageVersion string `json:"image_version,omitempty"`

	// Admin-applied free-form tags (eu-west, hetzner, warmup-only, ...).
	// Auto-derived "smart" labels (tier:free, pool:risky, state:error) are
	// computed client-side from the worker row and never stored here.
	Tags []string `json:"tags,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// WorkerInstallState mirrors the worker_install_state enum.
type WorkerInstallState string

const (
	WorkerInstallStatePending      WorkerInstallState = "pending"
	WorkerInstallStateProvisioning WorkerInstallState = "provisioning"
	WorkerInstallStateInstalled    WorkerInstallState = "installed"
	WorkerInstallStateError        WorkerInstallState = "error"
	WorkerInstallStateUninstalling WorkerInstallState = "uninstalling"
	WorkerInstallStateUninstalled  WorkerInstallState = "uninstalled"
)

// WorkerSSHCredentials carries the encrypted private key alongside the
// connection info. Only the orchestrator should ever fetch this; the field is
// never serialised to admin clients.
type WorkerSSHCredentials struct {
	WorkerID               uuid.UUID
	SSHHost                string
	SSHPort                int
	SSHUser                string
	SSHPublicKey           string
	SSHPrivateKeyEncrypted string
	SSHHostFingerprint     string
}

type UpdateWorker struct {
	IPAddr     *string     `json:"ip_addr"`
	Active     *bool       `json:"active"`
	WorkerType *WorkerType `json:"worker_type,omitempty"`
}

// DedicatedWorkerAssignment represents a dedicated worker assignment to a user
type DedicatedWorkerAssignment struct {
	ID             uuid.UUID  `json:"id"`
	WorkerID       uuid.UUID  `json:"worker_id"`
	OrganizationID uuid.UUID  `json:"organization_id"`
	SubscriptionID uuid.UUID  `json:"subscription_id"`
	AssignedAt     time.Time  `json:"assigned_at"`
	ReleasedAt     *time.Time `json:"released_at,omitempty"`
}

type WorkerStatus string

const (
	WorkerStatusOffline WorkerStatus = "offline"
	WorkerStatusLoading WorkerStatus = "loading"
	WorkerStatusOnline  WorkerStatus = "online"
)

type SendEmail struct {
	TaskID         uuid.UUID     `json:"task_id" avro:"task_id"`
	EmailID        uuid.UUID     `json:"email_id" avro:"email_id"`
	OrgID          uuid.UUID     `json:"org_id" avro:"org_id"`
	To             []string      `json:"to" avro:"to"`
	Cc             []string      `json:"cc" avro:"cc"`
	Bcc            []string      `json:"bcc" avro:"bcc"`
	Subject        string        `json:"subject" avro:"subject"`
	BodyS3Key      string        `json:"body_s3_key" avro:"body_s3_key"`
	MessageID      string        `json:"message_id" avro:"message_id"`
	InReplyTo      string        `json:"in_reply_to,omitempty" avro:"in_reply_to"`
	Parent         *EmailParent  `json:"parent,omitempty" avro:"parent"`
	IsWarmup       bool          `json:"is_warmup" avro:"is_warmup"`
	TrackingInfo   *TrackingInfo `json:"tracking_info,omitempty" avro:"tracking_info"`
	WarmupToken    string        `json:"warmup_token,omitempty" avro:"warmup_token"`
	UnsubscribeURL string        `json:"unsubscribe_url,omitempty" avro:"unsubscribe_url"`
}

// TrackingInfo contains tracking configuration for campaign emails
type TrackingInfo struct {
	OpenTracking   bool   `json:"open_tracking" avro:"open_tracking"`
	LinkTracking   bool   `json:"link_tracking" avro:"link_tracking"`
	TrackingDomain string `json:"tracking_domain" avro:"tracking_domain"`
}

// EmailSendError contains detailed error information for failed email sends
type EmailSendError struct {
	Code           string `json:"code" avro:"code"`
	Type           string `json:"type" avro:"type"`
	Message        string `json:"message" avro:"message"`
	ResolveMethod  string `json:"resolve_method" avro:"resolve_method"`
	UserVisible    bool   `json:"user_visible" avro:"user_visible"`
	UserTitle      string `json:"user_title,omitempty" avro:"user_title"`
	UserMessage    string `json:"user_message,omitempty" avro:"user_message"`
	ActionRequired string `json:"action_required,omitempty" avro:"action_required"`
}

// SendEmailResult is the result from worker after sending email
type SendEmailResult struct {
	TaskID         uuid.UUID       `json:"task_id" avro:"task_id"`
	Success        bool            `json:"success" avro:"success"`
	MessageID      string          `json:"message_id,omitempty" avro:"message_id"`
	ProviderMsgID  string          `json:"provider_msg_id,omitempty" avro:"provider_msg_id"`
	SentAt         time.Time       `json:"sent_at,omitempty" avro:"sent_at"`
	Error          *EmailSendError `json:"error,omitempty" avro:"error"`
	LegacyErrorMsg string          `json:"legacy_error,omitempty" avro:"legacy_error"` // Deprecated: use Error instead
	// SendMethod is how a successful send left the worker (SendMethodLabel):
	// transport, provider and deployment, so methods can be compared later.
	// Empty from a worker older than the field.
	SendMethod string `json:"send_method,omitempty" avro:"send_method"`
	// EmailAccountID and WorkerID name the mailbox and the worker that
	// answered, so the control plane can stop routing that mailbox's sends
	// to a worker that refused its credentials. Empty from an older worker.
	EmailAccountID string `json:"email_account_id,omitempty" avro:"email_account_id"`
	WorkerID       string `json:"worker_id,omitempty" avro:"worker_id"`
}

type AddWorkerEmailGoogleData struct {
	LastHistoryID uint64        `json:"last_history_id" avro:"last_history_id"`
	Token         *oauth2.Token `json:"token" avro:"token"`
}

type AddWorkerEmailSmtpImapData struct {
	Mailboxes   []Mailbox     `json:"mailboxes" avro:"mailboxes"`
	Token       *oauth2.Token `json:"token" avro:"token"`
	Credentials *SmtpImap     `json:"credentials" avro:"credentials"`
}

// AddWorkerEmailGraphData seeds a Microsoft Graph mailbox on the worker: the
// delegated OAuth token and the opaque per-folder delta cursors persisted by the
// control plane (empty on first connect, which primes the cursor without
// backfilling history).
type AddWorkerEmailGraphData struct {
	Token      *oauth2.Token     `json:"token" avro:"token"`
	DeltaLinks map[string]string `json:"delta_links" avro:"delta_links"`
}

// MailTransport is how the worker reaches an OAuth mailbox's provider.
// SMTP/IMAP mailboxes have only one transport and ignore it.
type MailTransport string

const (
	// MailTransportAPI is the provider's native API: Gmail API (history sync,
	// users.messages.send) and Microsoft Graph (delta sync, sendMail).
	MailTransportAPI MailTransport = "api"
	// MailTransportSMTP is the provider's IMAP and SMTP submission endpoints
	// authenticated with the same OAuth token over XOAUTH2. It is the path
	// that exercises the provider's real SMTP, which is what the recipient's
	// filters see.
	MailTransportSMTP MailTransport = "smtp"
)

// ParseMailTransport maps a configured or shipped value to a transport.
// Anything unrecognized, including the empty value an older publisher
// sends, is the API: that was the only transport before the switch existed.
func ParseMailTransport(v string) MailTransport {
	if MailTransport(v) == MailTransportSMTP {
		return MailTransportSMTP
	}
	return MailTransportAPI
}

type AddWorkerEmail struct {
	ID     uuid.UUID `json:"id" avro:"id"`
	UserID uuid.UUID `json:"user_id" avro:"user_id"`
	// OrganizationID scopes the organization-wide sync budget. Nil for a
	// legacy personal mailbox, which then only has per-mailbox budgets.
	OrganizationID *uuid.UUID `json:"organization_id" avro:"organization_id"`
	ImapSync       bool       `json:"imap_sync" avro:"imap_sync"`
	// SaveToSent is a pointer so an older control plane that does not send the
	// field is read as "unset" and takes the default (on) rather than as an
	// explicit false, which would silently stop filing sent mail.
	SaveToSent *bool                       `json:"save_to_sent,omitempty" avro:"save_to_sent"`
	Email      string                      `json:"email" avro:"email"`
	FirstName  string                      `json:"first_name" avro:"first_name"`
	LastName   string                      `json:"last_name" avro:"last_name"`
	Type       InboxProvider               `json:"type" avro:"type"`
	Google     *AddWorkerEmailGoogleData   `json:"google" avro:"google"`
	SmtpImap   *AddWorkerEmailSmtpImapData `json:"smtp_imap" avro:"smtp_imap"`
	Graph      *AddWorkerEmailGraphData    `json:"graph" avro:"graph"`
	// Sync is the fair-use budget and resume state. Nil only from a publisher
	// older than the sync policy; the worker then applies compiled defaults.
	Sync *AddWorkerEmailSyncData `json:"sync" avro:"sync"`
	// Brokered: no credential travels; the worker fetches access tokens from the backend.
	Brokered bool `json:"brokered" avro:"brokered"`
	// OAuthClient names the client that issued the provider tokens
	// (OAuthClient*), so the worker refreshes with the same one. Empty means default.
	OAuthClient string `json:"oauth_client,omitempty" avro:"oauth_client"`
	// Transport is how the worker drives a Gmail or Outlook mailbox
	// (MailTransport*). Shipped per mailbox so the control plane can switch
	// one mailbox at a time; empty means the API, which is what a publisher
	// older than the field always meant. Ignored for smtp_imap.
	Transport string `json:"transport,omitempty" avro:"transport"`
	// SendOnly loads the mailbox for sending on a worker that is not its
	// sync owner: the send clients are built, no sync loop runs, no sync or
	// warmup state is relayed. Unset from a publisher older than the field.
	SendOnly bool `json:"send_only,omitempty" avro:"send_only"`

	Cfg oauth2.Config `json:"-" avro:"-"`
	// TokenSource is set by the worker for brokered mailboxes.
	TokenSource oauth2.TokenSource `json:"-" avro:"-"`
}

// SavesSentCopy reports whether the worker should APPEND a copy of each sent
// message to the mailbox's Sent folder. Unset means yes: SMTP files nothing on
// its own, so the copy is the useful default.
func (a *AddWorkerEmail) SavesSentCopy() bool {
	return a.SaveToSent == nil || *a.SaveToSent
}

// MailTransport is the transport this payload asks for. An smtp_imap mailbox
// has only one and reports it as SMTP.
func (a *AddWorkerEmail) MailTransport() MailTransport {
	if a.Type == InboxProviderSMTPIMAP {
		return MailTransportSMTP
	}
	return ParseMailTransport(a.Transport)
}

// UsesSmtpImap reports whether the worker drives this mailbox over IMAP and
// SMTP, either because it is an smtp_imap mailbox or because an OAuth
// mailbox was shipped with the SMTP transport.
func (a *AddWorkerEmail) UsesSmtpImap() bool {
	return a.MailTransport() == MailTransportSMTP
}

type RemoveWorkerEmail struct {
	UserID  string `json:"user_id" avro:"user_id"`
	EmailID string `json:"email_id" avro:"email_id"`
}
