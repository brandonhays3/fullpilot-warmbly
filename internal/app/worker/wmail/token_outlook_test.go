package wmail

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/warmbly/warmbly/internal/config"
	"golang.org/x/oauth2"
)

// A fake Entra token endpoint: one token per resource, refresh tokens
// rotated on every use, and a consent that may lack one resource family.
type fakeEntra struct {
	mu       sync.Mutex
	requests []string // the scope of each request, in order
	refuse   string   // a scope prefix the consent does not cover
	seq      int
}

func (f *fakeEntra) handler(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		scope := r.Form.Get("scope")
		f.mu.Lock()
		f.requests = append(f.requests, scope)
		f.seq++
		seq := f.seq
		f.mu.Unlock()
		if r.Form.Get("grant_type") != "refresh_token" || !strings.HasPrefix(r.Form.Get("refresh_token"), "rt-") {
			http.Error(w, `{"error":"invalid_grant"}`, http.StatusBadRequest)
			return
		}
		if f.refuse != "" && strings.Contains(scope, f.refuse) {
			http.Error(w, `{"error":"invalid_grant","error_description":"AADSTS65001 consent required"}`, http.StatusBadRequest)
			return
		}
		resource := "graph"
		if strings.Contains(scope, "outlook.office.com") {
			resource = "mail"
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  resource + "-token-" + itoa(seq),
			"refresh_token": "rt-" + itoa(seq),
			"expires_in":    3600,
			"token_type":    "Bearer",
		})
	}
}

func itoa(n int) string { return strconv.Itoa(n) }

func TestOutlookTokensMintPerResourceAndPersistRotation(t *testing.T) {
	entra := &fakeEntra{}
	srv := httptest.NewServer(entra.handler(t))
	defer srv.Close()
	outlookTokenURL = srv.URL
	defer func() { outlookTokenURL = "" }()

	var persisted []*oauth2.Token
	tokens := newOutlookTokens(context.Background(), oauth2.Config{ClientID: "c"}, "rt-0", func(tok *oauth2.Token) error {
		persisted = append(persisted, tok)
		return nil
	})

	graph, err := tokens.Graph()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(graph.AccessToken, "graph-token-") {
		t.Fatalf("graph token = %q", graph.AccessToken)
	}
	// Cached: a second read mints nothing.
	if again, _ := tokens.Graph(); again.AccessToken != graph.AccessToken {
		t.Fatal("graph token was re-minted while fresh")
	}
	mail, err := tokens.Mail()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(mail.AccessToken, "mail-token-") {
		t.Fatalf("mail token = %q", mail.AccessToken)
	}
	if again, _ := tokens.Mail(); again.AccessToken != mail.AccessToken {
		t.Fatal("mail token was re-minted while fresh")
	}

	// Each request named one resource; the mail mint rotated the refresh
	// token, so a Graph token was re-minted to persist that rotation, and
	// that one is now the cached Graph token.
	if len(entra.requests) != 3 {
		t.Fatalf("token requests = %v, want graph, mail, graph", entra.requests)
	}
	if again, _ := tokens.Graph(); again.AccessToken != "graph-token-3" || len(entra.requests) != 3 {
		t.Fatalf("graph token after rotation = %q (%d requests), want graph-token-3 from cache", again.AccessToken, len(entra.requests))
	}
	if !strings.Contains(entra.requests[0], config.GraphMailSendScope) || strings.Contains(entra.requests[0], "outlook.office.com") {
		t.Fatalf("first request scope = %q", entra.requests[0])
	}
	if !strings.Contains(entra.requests[1], config.OutlookSMTPScope) || strings.Contains(entra.requests[1], "graph.microsoft.com") {
		t.Fatalf("second request scope = %q", entra.requests[1])
	}
	if len(persisted) != 2 {
		t.Fatalf("persisted %d tokens, want 2", len(persisted))
	}
	last := persisted[len(persisted)-1]
	if last.RefreshToken != "rt-3" || !strings.HasPrefix(last.AccessToken, "graph-token-") {
		t.Fatalf("persisted token = %+v, want the newest refresh token with a Graph access token", last)
	}
}

// A consent that predates the Graph scopes mints only mail tokens; the
// caller reads the Graph failure as "no Graph" and keeps the SMTP path.
func TestOutlookTokensReportTheMissingResource(t *testing.T) {
	entra := &fakeEntra{refuse: "graph.microsoft.com"}
	srv := httptest.NewServer(entra.handler(t))
	defer srv.Close()
	outlookTokenURL = srv.URL
	defer func() { outlookTokenURL = "" }()

	tokens := newOutlookTokens(context.Background(), oauth2.Config{ClientID: "c"}, "rt-0", nil)
	if _, err := tokens.Graph(); err == nil {
		t.Fatal("graph mint must fail without the Graph scopes")
	}
	if _, err := tokens.Mail(); err != nil {
		t.Fatalf("mail mint: %v", err)
	}
}
