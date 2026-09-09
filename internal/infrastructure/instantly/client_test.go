package instantly

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewEmptyKeyIsDisabled(t *testing.T) {
	if c := New("  "); c != nil {
		t.Fatalf("expected nil client for an empty key")
	}
	var c *Client
	if c.Enabled() {
		t.Fatalf("nil client must report disabled")
	}
	if _, err := c.GetAccount(context.Background(), "a@b.c"); !errors.Is(err, ErrDisabled) {
		t.Fatalf("expected ErrDisabled, got %v", err)
	}
}

func TestGetAccountNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer k1" {
			t.Errorf("authorization header = %q", got)
		}
		if r.URL.Path != "/accounts/missing@example.com" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"statusCode":404,"error":"Not Found","message":"Resource not found"}`))
	}))
	defer srv.Close()

	c := New("k1", WithBaseURL(srv.URL))
	_, err := c.GetAccount(context.Background(), "missing@example.com")
	if !IsNotFound(err) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestEnableWarmupSendsEmailsBody(t *testing.T) {
	var got struct {
		Emails []string `json:"emails"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/accounts/warmup/enable" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("decode body: %v", err)
		}
		_, _ = w.Write([]byte(`{"id":"job-1","status":"pending"}`))
	}))
	defer srv.Close()

	c := New("k1", WithBaseURL(srv.URL))
	job, err := c.EnableWarmup(context.Background(), " user@example.com ")
	if err != nil {
		t.Fatalf("EnableWarmup: %v", err)
	}
	if job.ID != "job-1" {
		t.Fatalf("job id = %q", job.ID)
	}
	if len(got.Emails) != 1 || got.Emails[0] != "user@example.com" {
		t.Fatalf("emails body = %v", got.Emails)
	}
}

func TestUpdateWarmupSettingsAndAPIError(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch || r.URL.Path != "/accounts/user@example.com" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.WriteHeader(http.StatusPaymentRequired)
		_, _ = w.Write([]byte(`{"statusCode":402,"error":"Payment Required","message":"Workspace does not have an active paid plan"}`))
	}))
	defer srv.Close()

	c := New("k1", WithBaseURL(srv.URL))
	_, err := c.UpdateWarmupSettings(context.Background(), "user@example.com", WarmupDefaults("outlook"))
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 402 {
		t.Fatalf("expected 402 APIError, got %v", err)
	}
	warm, _ := body["warmup"].(map[string]any)
	if warm["limit"] != float64(16) || warm["increment"] != "2" {
		t.Fatalf("warmup body = %v", warm)
	}
	adv, _ := warm["advanced"].(map[string]any)
	if adv["spam_save_rate"] != float64(1) || adv["read_emulation"] != true || adv["weekday_only"] != false {
		t.Fatalf("advanced body = %v", adv)
	}
}

func TestGetWarmupStatusMergesAnalytics(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/accounts/user@example.com":
			_, _ = w.Write([]byte(`{"email":"user@example.com","status":1,"warmup_status":1,"provider_code":2,"setup_pending":false,"stat_warmup_score":88,"warmup":{"limit":30,"increment":"2","reply_rate":0.65}}`))
		case "/accounts/warmup-analytics":
			_, _ = w.Write([]byte(`{"aggregate_data":{"User@example.com":{"sent":15,"received":15,"landed_inbox":13,"landed_spam":2,"health_score":87,"health_score_label":"87%"}}}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	c := New("k1", WithBaseURL(srv.URL))
	st, err := c.GetWarmupStatus(context.Background(), "user@example.com")
	if err != nil {
		t.Fatalf("GetWarmupStatus: %v", err)
	}
	if !st.Active || st.WarmupLabel != "active" || st.AccountLabel != "active" {
		t.Fatalf("status = %+v", st)
	}
	if st.WarmupScore == nil || *st.WarmupScore != 88 {
		t.Fatalf("warmup score = %v", st.WarmupScore)
	}
	if st.Analytics == nil || st.Analytics.LandedSpam != 2 || st.Analytics.HealthScore != 87 {
		t.Fatalf("analytics = %+v", st.Analytics)
	}
	if st.Settings == nil || st.Settings.Limit != 30 {
		t.Fatalf("settings = %+v", st.Settings)
	}
}

func TestWarmupDefaults(t *testing.T) {
	for provider, limit := range map[string]int{"gmail": 30, "outlook": 16, "smtp_imap": 30} {
		d := WarmupDefaults(provider)
		if d.Limit != limit || d.Increment != "2" || d.ReplyRate != 0.65 {
			t.Fatalf("%s defaults = %+v", provider, d)
		}
		if d.Advanced == nil || d.Advanced.OpenRate != 0.63 || d.Advanced.ImportantRate != 0.34 || d.Advanced.SpamSaveRate != 1.0 || !d.Advanced.ReadEmulation || d.Advanced.WeekdayOnly {
			t.Fatalf("%s advanced = %+v", provider, d.Advanced)
		}
	}
	if ProviderCodeFor("gmail") != ProviderCodeGoogle || ProviderCodeFor("outlook") != ProviderCodeMicrosoft || ProviderCodeFor("smtp_imap") != ProviderCodeCustomIMAPSMTP {
		t.Fatalf("provider code mapping is wrong")
	}
}
