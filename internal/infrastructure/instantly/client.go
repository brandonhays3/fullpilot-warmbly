// Package instantly is a thin client for the Instantly.ai API v2 account and
// warmup endpoints (https://developer.instantly.ai/). It exists so a mailbox
// can be warmed by Instantly's network instead of Warmbly's own pool.
//
// Endpoints used, all under https://api.instantly.ai/api/v2 with
// `Authorization: Bearer <api key>`:
//
//   - GET  /accounts/{email}            one account, 404 when unknown
//   - POST /accounts                    create an IMAP/SMTP account
//   - PATCH /accounts/{email}           update warmup settings
//   - POST /accounts/warmup/enable      {emails: [...]} background job
//   - POST /accounts/warmup/disable     {emails: [...]} background job
//   - POST /accounts/warmup-analytics   {emails: [...]} per-account totals
package instantly

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	// DefaultBaseURL is the production API root.
	DefaultBaseURL = "https://api.instantly.ai/api/v2"
	// EnvAPIKey is the environment variable the backend reads the key from.
	// Empty means the integration is off and Warmbly's own warmup runs.
	EnvAPIKey = "INSTANTLY_API_KEY"
	// DashboardAccountsURL is where a user adds a Google or Microsoft
	// mailbox to their Instantly workspace, which the API cannot do alone.
	DashboardAccountsURL = "https://app.instantly.ai/app/accounts"

	defaultTimeout = 15 * time.Second
	maxErrorBody   = 4 << 10
)

// Provider codes Instantly assigns to an account's connection type.
const (
	ProviderCodeCustomIMAPSMTP = 1
	ProviderCodeGoogle         = 2
	ProviderCodeMicrosoft      = 3
)

// Account.Status values.
const (
	AccountStatusActive          = 1
	AccountStatusPaused          = 2
	AccountStatusMaintenance     = 3
	AccountStatusConnectionError = -1
	AccountStatusSoftBounce      = -2
	AccountStatusSendingError    = -3
)

// Account.WarmupStatus values.
const (
	WarmupStatusPaused         = 0
	WarmupStatusActive         = 1
	WarmupStatusBanned         = -1
	WarmupStatusSpamUnknown    = -2
	WarmupStatusPermanentlyOff = -3
)

// ErrNotFound is returned when Instantly has no account for the email.
var ErrNotFound = errors.New("instantly: account not found")

// APIError is any non-2xx answer other than a 404 on a lookup.
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("instantly: HTTP %d", e.StatusCode)
	}
	return fmt.Sprintf("instantly: HTTP %d: %s", e.StatusCode, e.Message)
}

// WarmupAdvanced mirrors the `warmup.advanced` object. Rates are fractions
// (0.63 is 63%), which is how the API documents its examples.
type WarmupAdvanced struct {
	WarmCTD       *bool   `json:"warm_ctd,omitempty"`
	OpenRate      float64 `json:"open_rate"`
	ImportantRate float64 `json:"important_rate"`
	ReadEmulation bool    `json:"read_emulation"`
	SpamSaveRate  float64 `json:"spam_save_rate"`
	WeekdayOnly   bool    `json:"weekday_only"`
}

// WarmupSettings mirrors the `warmup` object on an account. Increment is a
// string enum: "disabled" or "0".."4" emails added per day.
type WarmupSettings struct {
	Limit     int             `json:"limit"`
	Increment string          `json:"increment"`
	ReplyRate float64         `json:"reply_rate"`
	Advanced  *WarmupAdvanced `json:"advanced,omitempty"`
}

// Account is the subset of Instantly's Account schema Warmbly reads.
type Account struct {
	Email                string          `json:"email"`
	FirstName            string          `json:"first_name"`
	LastName             string          `json:"last_name"`
	Status               int             `json:"status"`
	WarmupStatus         int             `json:"warmup_status"`
	ProviderCode         int             `json:"provider_code"`
	SetupPending         bool            `json:"setup_pending"`
	Warmup               *WarmupSettings `json:"warmup,omitempty"`
	StatWarmupScore      *float64        `json:"stat_warmup_score"`
	TimestampWarmupStart *time.Time      `json:"timestamp_warmup_start"`
	DailyLimit           *int            `json:"daily_limit"`
}

// CreateAccountInput is the body of POST /accounts for a custom IMAP/SMTP
// mailbox. Every credential field is required by the API.
type CreateAccountInput struct {
	Email        string          `json:"email"`
	FirstName    string          `json:"first_name"`
	LastName     string          `json:"last_name"`
	ProviderCode int             `json:"provider_code"`
	IMAPUsername string          `json:"imap_username"`
	IMAPPassword string          `json:"imap_password"`
	IMAPHost     string          `json:"imap_host"`
	IMAPPort     int             `json:"imap_port"`
	SMTPUsername string          `json:"smtp_username"`
	SMTPPassword string          `json:"smtp_password"`
	SMTPHost     string          `json:"smtp_host"`
	SMTPPort     int             `json:"smtp_port"`
	Warmup       *WarmupSettings `json:"warmup,omitempty"`
	DailyLimit   *int            `json:"daily_limit,omitempty"`
}

// WarmupAnalytics is one account's row from POST /accounts/warmup-analytics.
type WarmupAnalytics struct {
	Sent             int     `json:"sent"`
	Received         int     `json:"received"`
	LandedInbox      int     `json:"landed_inbox"`
	LandedSpam       int     `json:"landed_spam"`
	HealthScore      float64 `json:"health_score"`
	HealthScoreLabel string  `json:"health_score_label"`
}

// WarmupStatus is what the dashboard shows for a mailbox Instantly warms.
type WarmupStatus struct {
	Email         string          `json:"email"`
	Active        bool            `json:"active"`
	WarmupStatus  int             `json:"warmup_status"`
	WarmupLabel   string          `json:"warmup_label"`
	AccountStatus int             `json:"account_status"`
	AccountLabel  string          `json:"account_label"`
	SetupPending  bool            `json:"setup_pending"`
	StartedAt     *time.Time      `json:"started_at,omitempty"`
	WarmupScore   *float64        `json:"warmup_score,omitempty"`
	Settings      *WarmupSettings `json:"settings,omitempty"`
	// Analytics is nil when the analytics call failed; the status above
	// still stands on its own.
	Analytics *WarmupAnalytics `json:"analytics,omitempty"`
}

// BackgroundJob is the reply to the bulk warmup enable/disable calls.
type BackgroundJob struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// Client talks to one Instantly workspace.
type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

// Option customizes a Client.
type Option func(*Client)

// WithBaseURL points the client at another API root (tests, proxies).
func WithBaseURL(u string) Option {
	return func(c *Client) { c.baseURL = strings.TrimRight(u, "/") }
}

// WithHTTPClient swaps the transport; the default has a 15s timeout.
func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) { c.http = h }
}

// New builds a client. An empty key yields nil, which every method treats
// as "integration off", so callers can wire it unconditionally.
func New(apiKey string, opts ...Option) *Client {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil
	}
	c := &Client{
		baseURL: DefaultBaseURL,
		apiKey:  apiKey,
		http:    &http.Client{Timeout: defaultTimeout},
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// NewFromEnv reads INSTANTLY_API_KEY; nil when it is unset.
func NewFromEnv(opts ...Option) *Client {
	return New(os.Getenv(EnvAPIKey), opts...)
}

// Enabled reports whether the integration is configured. Safe on nil.
func (c *Client) Enabled() bool { return c != nil && c.apiKey != "" }

// GetAccount looks an account up by email. ErrNotFound when absent.
func (c *Client) GetAccount(ctx context.Context, email string) (*Account, error) {
	var out Account
	if err := c.do(ctx, http.MethodGet, "/accounts/"+url.PathEscape(strings.TrimSpace(email)), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateAccount adds a custom IMAP/SMTP mailbox to the workspace.
func (c *Client) CreateAccount(ctx context.Context, in CreateAccountInput) (*Account, error) {
	if in.ProviderCode == 0 {
		in.ProviderCode = ProviderCodeCustomIMAPSMTP
	}
	var out Account
	if err := c.do(ctx, http.MethodPost, "/accounts", in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateWarmupSettings patches the account's warmup object.
func (c *Client) UpdateWarmupSettings(ctx context.Context, email string, settings WarmupSettings) (*Account, error) {
	body := struct {
		Warmup WarmupSettings `json:"warmup"`
	}{Warmup: settings}
	var out Account
	if err := c.do(ctx, http.MethodPatch, "/accounts/"+url.PathEscape(strings.TrimSpace(email)), body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// EnableWarmup starts warmup for one account. Instantly runs it as a
// background job; the job handle is returned for callers that want to poll.
func (c *Client) EnableWarmup(ctx context.Context, email string) (*BackgroundJob, error) {
	return c.warmupBulk(ctx, "/accounts/warmup/enable", email)
}

// PauseWarmup stops warmup for one account (Instantly calls it disable; the
// account and its settings stay).
func (c *Client) PauseWarmup(ctx context.Context, email string) (*BackgroundJob, error) {
	return c.warmupBulk(ctx, "/accounts/warmup/disable", email)
}

func (c *Client) warmupBulk(ctx context.Context, path, email string) (*BackgroundJob, error) {
	body := struct {
		Emails []string `json:"emails"`
	}{Emails: []string{strings.TrimSpace(email)}}
	var out BackgroundJob
	if err := c.do(ctx, http.MethodPost, path, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetWarmupAnalytics returns the aggregate warmup numbers for one account.
// A missing row (no warmup activity yet) comes back as an empty struct.
func (c *Client) GetWarmupAnalytics(ctx context.Context, email string) (*WarmupAnalytics, error) {
	email = strings.TrimSpace(email)
	body := struct {
		Emails []string `json:"emails"`
	}{Emails: []string{email}}
	var out struct {
		Aggregate map[string]WarmupAnalytics `json:"aggregate_data"`
	}
	if err := c.do(ctx, http.MethodPost, "/accounts/warmup-analytics", body, &out); err != nil {
		return nil, err
	}
	for k, v := range out.Aggregate {
		if strings.EqualFold(k, email) {
			row := v
			return &row, nil
		}
	}
	return &WarmupAnalytics{}, nil
}

// GetWarmupStatus combines the account record with its warmup analytics.
// ErrNotFound when the account is not in the workspace.
func (c *Client) GetWarmupStatus(ctx context.Context, email string) (*WarmupStatus, error) {
	acc, err := c.GetAccount(ctx, email)
	if err != nil {
		return nil, err
	}
	st := &WarmupStatus{
		Email:         acc.Email,
		Active:        acc.WarmupStatus == WarmupStatusActive,
		WarmupStatus:  acc.WarmupStatus,
		WarmupLabel:   WarmupStatusLabel(acc.WarmupStatus),
		AccountStatus: acc.Status,
		AccountLabel:  AccountStatusLabel(acc.Status),
		SetupPending:  acc.SetupPending,
		StartedAt:     acc.TimestampWarmupStart,
		WarmupScore:   acc.StatWarmupScore,
		Settings:      acc.Warmup,
	}
	// Analytics failing must not hide the status itself.
	if a, aerr := c.GetWarmupAnalytics(ctx, email); aerr == nil {
		st.Analytics = a
	}
	return st, nil
}

// WarmupStatusLabel is the human name of an Account.WarmupStatus value.
func WarmupStatusLabel(v int) string {
	switch v {
	case WarmupStatusActive:
		return "active"
	case WarmupStatusPaused:
		return "paused"
	case WarmupStatusBanned:
		return "banned"
	case WarmupStatusSpamUnknown:
		return "spam folder unknown"
	case WarmupStatusPermanentlyOff:
		return "permanently suspended"
	}
	return fmt.Sprintf("unknown (%d)", v)
}

// AccountStatusLabel is the human name of an Account.Status value.
func AccountStatusLabel(v int) string {
	switch v {
	case AccountStatusActive:
		return "active"
	case AccountStatusPaused:
		return "paused"
	case AccountStatusMaintenance:
		return "maintenance"
	case AccountStatusConnectionError:
		return "connection error"
	case AccountStatusSoftBounce:
		return "soft bounce error"
	case AccountStatusSendingError:
		return "sending error"
	}
	return fmt.Sprintf("unknown (%d)", v)
}

// IsNotFound reports whether err is the account-missing case.
func IsNotFound(err error) bool { return errors.Is(err, ErrNotFound) }

// ErrDisabled is returned by every method on a nil client.
var ErrDisabled = errors.New("instantly: integration not configured")

func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	if !c.Enabled() {
		return ErrDisabled
	}
	var rdr io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("instantly: encode %s %s: %w", method, path, err)
		}
		rdr = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, rdr)
	if err != nil {
		return fmt.Errorf("instantly: build %s %s: %w", method, path, err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("instantly: %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		_, _ = io.Copy(io.Discard, resp.Body)
		return ErrNotFound
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))
		return &APIError{StatusCode: resp.StatusCode, Message: errorMessage(raw)}
	}
	if out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(out); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("instantly: decode %s %s: %w", method, path, err)
	}
	return nil
}

// errorMessage pulls Instantly's {statusCode, error, message} body apart,
// falling back to the raw text.
func errorMessage(raw []byte) string {
	var body struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	if json.Unmarshal(raw, &body) == nil && (body.Message != "" || body.Error != "") {
		if body.Message == "" {
			return body.Error
		}
		if body.Error == "" {
			return body.Message
		}
		return body.Error + ": " + body.Message
	}
	return strings.TrimSpace(string(raw))
}
