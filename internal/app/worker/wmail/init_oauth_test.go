package wmail

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/oauth2"
	"google.golang.org/api/gmail/v1"
)

func TestGoogleTokenCanMail(t *testing.T) {
	var scope string
	var status int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("access_token") != "at" {
			t.Errorf("access_token = %q", r.URL.Query().Get("access_token"))
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"scope":"` + scope + `"}`))
	}))
	defer srv.Close()
	googleTokeninfoURL = srv.URL
	defer func() { googleTokeninfoURL = "https://oauth2.googleapis.com/tokeninfo" }()

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "at"})
	cases := []struct {
		name   string
		status int
		scope  string
		want   bool
	}{
		{"full mail scope", 200, "openid " + gmail.MailGoogleComScope, true},
		{"granular only", 200, "openid " + gmail.GmailSendScope + " " + gmail.GmailReadonlyScope, false},
		{"no answer", 500, "", true},
		{"empty scope", 200, "", true},
	}
	for _, c := range cases {
		status, scope = c.status, c.scope
		if got := googleTokenCanMail(context.Background(), ts); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
	if !googleTokenCanMail(context.Background(), oauth2.StaticTokenSource(&oauth2.Token{})) {
		t.Error("an empty token must not strip SMTP")
	}
}
