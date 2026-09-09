package templates

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// previewTime is a fixed timestamp so deletion previews/tests are
// deterministic instead of depending on the wall clock.
var previewTime = time.Date(2026, time.July, 1, 9, 0, 0, 0, time.UTC)

// ─── Base template (shared chrome) ───────────────────────────────

func TestBaseTemplate_Structure(t *testing.T) {
	html, err := GenerateLoginCodeHTML("000000")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	checks := []string{
		"<!DOCTYPE html>",
		"<html lang=\"en\">",
		// Inline SVG logo, dashboard slate fill.
		"<svg",
		"M222.805 644.772",
		"fill=\"#0f172a\"",
		// Brae chrome: cream wrapper + white card + hairline border.
		"#f5f6f8",
		"#ffffff",
		"#e2e8f0",
		"border-radius:8px",
	}

	for _, s := range checks {
		if !strings.Contains(html, s) {
			t.Errorf("base template: expected HTML to contain %q", s)
		}
	}
}

func TestBaseTemplate_BusinessDetails(t *testing.T) {
	html, err := GenerateLoginCodeHTML("000000")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	checks := []string{
		// Branding
		CompanyName,
		"warmbly.com",
		"Privacy",
		"Terms",
		TermsURL,
		PrivacyURL,
		// Companies Act 2006 required details
		LegalEntity,
		CompanyNumber,
		PlaceOfReg,
		RegisteredAddr,
	}

	for _, s := range checks {
		if !strings.Contains(html, s) {
			t.Errorf("footer: expected HTML to contain %q", s)
		}
	}
}

// ─── Login Code ──────────────────────────────────────────────────

func TestGenerateLoginCodeHTML(t *testing.T) {
	code := "123456"
	html, err := GenerateLoginCodeHTML(code)
	if err != nil {
		t.Fatalf("GenerateLoginCodeHTML returned error: %v", err)
	}

	if html == "" {
		t.Fatal("GenerateLoginCodeHTML returned empty string")
	}

	checks := []struct {
		name   string
		substr string
	}{
		{"contains the verification code", code},
		{"code monospace tracking", "letter-spacing:6px"},
		{"heading", "Your login code"},
		{"sign-in helper", "finish logging in"},
		{"expiry notice", "Expires in 15 minutes"},
		{"safety notice", "safely ignore"},
		{"title tag", "<title>Your Login Code</title>"},
	}

	for _, c := range checks {
		if !strings.Contains(html, c.substr) {
			t.Errorf("%s: expected HTML to contain %q", c.name, c.substr)
		}
	}
}

func TestGenerateLoginCodeHTML_DifferentCodes(t *testing.T) {
	codes := []string{"000000", "999999", "ABC123", "1"}
	for _, code := range codes {
		html, err := GenerateLoginCodeHTML(code)
		if err != nil {
			t.Fatalf("GenerateLoginCodeHTML(%q) returned error: %v", code, err)
		}
		if !strings.Contains(html, code) {
			t.Errorf("GenerateLoginCodeHTML(%q): code not found in output", code)
		}
	}
}

// ─── Registration Code ──────────────────────────────────────────

func TestGenerateRegistrationCodeHTML(t *testing.T) {
	code := "789012"
	html, err := GenerateRegistrationCodeHTML(code)
	if err != nil {
		t.Fatalf("GenerateRegistrationCodeHTML returned error: %v", err)
	}

	if html == "" {
		t.Fatal("GenerateRegistrationCodeHTML returned empty string")
	}

	checks := []struct {
		name   string
		substr string
	}{
		{"contains the verification code", code},
		{"welcome heading", "Welcome to Fullpilot"},
		{"registration prompt", "finish creating your account"},
		{"expiry notice", "Expires in 15 minutes"},
		{"title tag", "<title>Your Verification Code</title>"},
	}

	for _, c := range checks {
		if !strings.Contains(html, c.substr) {
			t.Errorf("%s: expected HTML to contain %q", c.name, c.substr)
		}
	}
}

func TestGenerateRegistrationCodeHTML_DifferentCodes(t *testing.T) {
	codes := []string{"000000", "999999", "ABC123", "1"}
	for _, code := range codes {
		html, err := GenerateRegistrationCodeHTML(code)
		if err != nil {
			t.Fatalf("GenerateRegistrationCodeHTML(%q) returned error: %v", code, err)
		}
		if !strings.Contains(html, code) {
			t.Errorf("GenerateRegistrationCodeHTML(%q): code not found in output", code)
		}
	}
}

// ─── Reset Password ─────────────────────────────────────────────

func TestGenerateResetPasswordHTML(t *testing.T) {
	url := "https://app.warmbly.com/reset?token=abc123def456"
	html, err := GenerateResetPasswordHTML("", url)
	if err != nil {
		t.Fatalf("GenerateResetPasswordHTML returned error: %v", err)
	}

	if html == "" {
		t.Fatal("GenerateResetPasswordHTML returned empty string")
	}

	checks := []struct {
		name   string
		substr string
	}{
		{"reset URL in button href", url},
		{"button text", "Reset password</a>"},
		{"reset prompt", "Reset your password"},
		{"ignore notice", "safely ignore this email"},
		{"expiry notice", "expires in 4 hours"},
		{"title tag", "<title>Reset Your Password</title>"},
	}

	for _, c := range checks {
		if !strings.Contains(html, c.substr) {
			t.Errorf("%s: expected HTML to contain %q", c.name, c.substr)
		}
	}

	// The reset URL should appear at least twice (button href + plaintext link)
	count := strings.Count(html, url)
	if count < 2 {
		t.Errorf("expected reset URL to appear at least 2 times, got %d", count)
	}
}

func TestGenerateResetPasswordHTML_URLEncoding(t *testing.T) {
	url := "https://app.warmbly.com/reset?token=abc&user=test@example.com"
	html, err := GenerateResetPasswordHTML("", url)
	if err != nil {
		t.Fatalf("GenerateResetPasswordHTML returned error: %v", err)
	}
	if !strings.Contains(html, "app.warmbly.com/reset") {
		t.Error("expected HTML to contain the reset URL domain")
	}
}

// ─── Cross-template checks ──────────────────────────────────────

func TestTemplatesProduceDistinctOutput(t *testing.T) {
	loginHTML, err := GenerateLoginCodeHTML("111111")
	if err != nil {
		t.Fatal(err)
	}

	regHTML, err := GenerateRegistrationCodeHTML("111111")
	if err != nil {
		t.Fatal(err)
	}

	resetHTML, err := GenerateResetPasswordHTML("", "https://example.com/reset")
	if err != nil {
		t.Fatal(err)
	}

	if loginHTML == regHTML {
		t.Error("login and registration templates should produce different output")
	}
	if loginHTML == resetHTML {
		t.Error("login and reset templates should produce different output")
	}
	if regHTML == resetHTML {
		t.Error("registration and reset templates should produce different output")
	}
}

func TestLoginCodeDoesNotContainResetContent(t *testing.T) {
	html, _ := GenerateLoginCodeHTML("123456")
	if strings.Contains(html, "Reset Password</a>") {
		t.Error("login code template should not contain reset password button")
	}
	if strings.Contains(html, "reset your password") {
		t.Error("login code template should not contain reset password text")
	}
}

func TestResetPasswordDoesNotContainCodeBlock(t *testing.T) {
	html, _ := GenerateResetPasswordHTML("", "https://example.com/reset")
	if strings.Contains(html, "letter-spacing:8px") {
		t.Error("reset password template should not contain a code display block")
	}
	if strings.Contains(html, "verify your login") {
		t.Error("reset password template should not contain login verify text")
	}
}

// ─── Constants ──────────────────────────────────────────────────

// ─── Preview — writes HTML files to a temp dir and prints paths ──

func TestPreview(t *testing.T) {
	dir := filepath.Join(os.TempDir(), "warmbly-email-preview")
	os.MkdirAll(dir, 0755)

	templates := []struct {
		name string
		gen  func() (string, error)
	}{
		{"login-code.html", func() (string, error) { return GenerateLoginCodeHTML("123456") }},
		{"registration-code.html", func() (string, error) { return GenerateRegistrationCodeHTML("789012") }},
		{"reset-password.html", func() (string, error) {
			return GenerateResetPasswordHTML("", "https://app.warmbly.com/reset?token=abc123def456")
		}},
		{"trial-expired.html", func() (string, error) { return GenerateTrialExpiredHTML() }},
		{"invitation.html", func() (string, error) {
			return GenerateInvitationHTML("Jane Doe", "Acme Inc", AppURL+"/invite?token=abc123")
		}},
		{"notification.html", func() (string, error) {
			return GenerateNotificationHTML("New sign-in to your account", "Signed in from Chrome on macOS (London, GB).", AppURL+"/app/settings/security", "")
		}},
		{"deletion-org-scheduled.html", func() (string, error) {
			return GenerateOrgDeletionScheduledHTML("Acme Inc", previewTime, 30, AppURL+"/organization/settings/danger-zone")
		}},
		{"deletion-user-scheduled.html", func() (string, error) {
			return GenerateUserDeletionScheduledHTML("Jane", previewTime, 30, AppURL+"/account/danger-zone")
		}},
		{"deletion-reminder.html", func() (string, error) {
			return GenerateDeletionReminderHTML("Acme Inc", previewTime, AppURL+"/organization/settings/danger-zone")
		}},
		{"deletion-completed.html", func() (string, error) {
			return GenerateDeletionCompletedHTML(previewTime, previewTime)
		}},
	}

	for _, tmpl := range templates {
		html, err := tmpl.gen()
		if err != nil {
			t.Fatalf("%s: %v", tmpl.name, err)
		}
		path := filepath.Join(dir, tmpl.name)
		if err := os.WriteFile(path, []byte(html), 0644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
		t.Logf("Preview: %s", path)
	}

	t.Logf("Open all: open %s/*.html", dir)
}

// ─── Trial expired ──────────────────────────────────────────────

func TestGenerateTrialExpiredHTML(t *testing.T) {
	html, err := GenerateTrialExpiredHTML()
	if err != nil {
		t.Fatalf("GenerateTrialExpiredHTML returned error: %v", err)
	}
	checks := []string{
		"<!DOCTYPE html>", // shared base shell
		"#f5f6f8",         // branded cream wrapper
		"Your free trial has ended",
		"Choose a plan</a>",
		AppURL + "/settings/billing", // billing CTA href
		"<title>Your Fullpilot trial has ended</title>",
	}
	for _, s := range checks {
		if !strings.Contains(html, s) {
			t.Errorf("trial expired: expected HTML to contain %q", s)
		}
	}
}

// ─── Invitation ─────────────────────────────────────────────────

func TestGenerateInvitationHTML(t *testing.T) {
	url := AppURL + "/invite?token=abc123"
	html, err := GenerateInvitationHTML("Jane Doe", "Acme Inc", url)
	if err != nil {
		t.Fatalf("GenerateInvitationHTML returned error: %v", err)
	}
	checks := []string{
		"#f5f6f8",
		"Jane Doe",
		"Acme Inc",
		"Accept invitation</a>",
		url,
		"expires in 7 days",
	}
	for _, s := range checks {
		if !strings.Contains(html, s) {
			t.Errorf("invitation: expected HTML to contain %q", s)
		}
	}
}

func TestGenerateInvitationHTML_EscapesNames(t *testing.T) {
	html, err := GenerateInvitationHTML("<script>evil()</script>", "Acme & Co", AppURL+"/invite?token=x")
	if err != nil {
		t.Fatalf("GenerateInvitationHTML returned error: %v", err)
	}
	if strings.Contains(html, "<script>evil()</script>") {
		t.Error("invitation: inviter name must be HTML-escaped")
	}
	if !strings.Contains(html, "Acme &amp; Co") {
		t.Error("invitation: org name ampersand must be HTML-escaped")
	}
}

// ─── Notification ───────────────────────────────────────────────

func TestGenerateNotificationHTML_WithCTA(t *testing.T) {
	html, err := GenerateNotificationHTML("New sign-in", "Signed in from Chrome.", AppURL+"/app/settings/security", "")
	if err != nil {
		t.Fatalf("GenerateNotificationHTML returned error: %v", err)
	}
	checks := []string{
		"#f5f6f8",
		"New sign-in",
		"Signed in from Chrome.",
		"Open in Fullpilot</a>", // default CTA label
		AppURL + "/app/settings/security",
		"<title>New sign-in</title>",
	}
	for _, s := range checks {
		if !strings.Contains(html, s) {
			t.Errorf("notification: expected HTML to contain %q", s)
		}
	}
}

func TestGenerateNotificationHTML_NoCTA(t *testing.T) {
	html, err := GenerateNotificationHTML("Heads up", "Something happened.", "", "")
	if err != nil {
		t.Fatalf("GenerateNotificationHTML returned error: %v", err)
	}
	if strings.Contains(html, "Open in Fullpilot") {
		t.Error("notification: no button should render when ctaURL is empty")
	}
}

// ─── Deletion (danger zone) ─────────────────────────────────────

func TestGenerateDeletionEmails(t *testing.T) {
	cancel := AppURL + "/account/danger-zone"
	cases := []struct {
		name    string
		gen     func() (string, error)
		wants   []string
		notWant string
	}{
		{
			name:  "org scheduled",
			gen:   func() (string, error) { return GenerateOrgDeletionScheduledHTML("Acme Inc", previewTime, 30, cancel) },
			wants: []string{"Organization scheduled for deletion", "Acme Inc", "Cancel deletion</a>", "Will be deleted on", "01 July 2026", cancel},
		},
		{
			name:  "user scheduled",
			gen:   func() (string, error) { return GenerateUserDeletionScheduledHTML("Jane", previewTime, 30, cancel) },
			wants: []string{"Account scheduled for deletion", "Hi Jane", "Cancel deletion</a>", "30 days"},
		},
		{
			name:  "org cancelled",
			gen:   func() (string, error) { return GenerateOrgDeletionCancelledHTML("Acme Inc", previewTime) },
			wants: []string{"Deletion cancelled", "scheduled deletion for", "Acme Inc", "Originally scheduled for:"},
		},
		{
			name:  "user cancelled",
			gen:   func() (string, error) { return GenerateUserDeletionCancelledHTML("Jane", previewTime) },
			wants: []string{"Account deletion cancelled", "Hi Jane", "back to normal"},
		},
		{
			name:  "reminder",
			gen:   func() (string, error) { return GenerateDeletionReminderHTML("Acme Inc", previewTime, cancel) },
			wants: []string{"Deletion in", "Acme Inc", "Cancel deletion</a>"},
		},
		{
			name:  "completed",
			gen:   func() (string, error) { return GenerateDeletionCompletedHTML(previewTime, previewTime) },
			wants: []string{"Deletion completed", "Executed at:"},
		},
	}
	for _, c := range cases {
		html, err := c.gen()
		if err != nil {
			t.Fatalf("%s: returned error: %v", c.name, err)
		}
		if !strings.Contains(html, "<!DOCTYPE html>") {
			t.Errorf("%s: expected shared base shell", c.name)
		}
		for _, w := range c.wants {
			if !strings.Contains(html, w) {
				t.Errorf("%s: expected HTML to contain %q", c.name, w)
			}
		}
	}
}

// ─── Constants ──────────────────────────────────────────────────

func TestBusinessConstants(t *testing.T) {
	if CompanyName == "" {
		t.Error("CompanyName should not be empty")
	}
	if LegalEntity == "" {
		t.Error("LegalEntity should not be empty")
	}
	if CompanyNumber == "" {
		t.Error("CompanyNumber should not be empty")
	}
	if PlaceOfReg == "" {
		t.Error("PlaceOfReg should not be empty")
	}
	if RegisteredAddr == "" {
		t.Error("RegisteredAddr should not be empty")
	}
	if WebsiteURL == "" {
		t.Error("WebsiteURL should not be empty")
	}
	if !strings.HasPrefix(WebsiteURL, "https://") {
		t.Error("WebsiteURL should start with https://")
	}
	if !strings.HasPrefix(TermsURL, "https://") {
		t.Error("TermsURL should start with https://")
	}
	if !strings.HasPrefix(PrivacyURL, "https://") {
		t.Error("PrivacyURL should start with https://")
	}
}
