package email

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/warmbly/warmbly/internal/config"
	"github.com/warmbly/warmbly/internal/models"
	"golang.org/x/oauth2"
)

// identityServer serves the provider identity endpoints from one mux and
// points the package's endpoint variables at it for the test's lifetime.
func identityServer(t *testing.T, routes map[string]any) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	for path, body := range routes {
		body := body
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") == "" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			if status, ok := body.(int); ok {
				w.WriteHeader(status)
				return
			}
			_ = json.NewEncoder(w).Encode(body)
		})
	}
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	prev := []string{googleUserinfoURL, gmailProfileURL, graphMeURL, microsoftUserinfoURL}
	googleUserinfoURL = srv.URL + "/google/userinfo"
	gmailProfileURL = srv.URL + "/gmail/profile"
	graphMeURL = srv.URL + "/graph/me"
	microsoftUserinfoURL = srv.URL + "/ms/userinfo"
	t.Cleanup(func() {
		googleUserinfoURL, gmailProfileURL, graphMeURL, microsoftUserinfoURL = prev[0], prev[1], prev[2], prev[3]
	})
	return srv
}

func TestFetchGmailOwner_UsesRealNameFromUserinfo(t *testing.T) {
	identityServer(t, map[string]any{
		"/google/userinfo": map[string]any{"email": "ana.silva@gmail.com", "given_name": "Ana", "family_name": "Silva", "name": "Ana Silva"},
	})
	owner, xerr := fetchInboxOwner(context.Background(), models.InboxProviderGoogle, nil, &oauth2.Token{AccessToken: "t"})
	if xerr != nil {
		t.Fatalf("fetchInboxOwner: %v", xerr)
	}
	if owner.Email != "ana.silva@gmail.com" || owner.Name != "Ana Silva" {
		t.Fatalf("owner = %+v, want Ana Silva <ana.silva@gmail.com>", owner)
	}
}

func TestFetchGmailOwner_FallsBackToProfileWithoutIdentityScopes(t *testing.T) {
	// A consent from before the identity scopes: userinfo refuses, the Gmail
	// profile still answers, and the name is left for the local-part fallback.
	identityServer(t, map[string]any{
		"/google/userinfo": http.StatusForbidden,
		"/gmail/profile":   map[string]any{"emailAddress": "ana.silva@gmail.com"},
	})
	owner, xerr := fetchInboxOwner(context.Background(), models.InboxProviderGoogle, nil, &oauth2.Token{AccessToken: "t"})
	if xerr != nil {
		t.Fatalf("fetchInboxOwner: %v", xerr)
	}
	if owner.Email != "ana.silva@gmail.com" || owner.Name != "" {
		t.Fatalf("owner = %+v, want address only", owner)
	}
	if got := deriveNameFromEmail(owner.Email); got != "Ana Silva" {
		t.Fatalf("deriveNameFromEmail = %q", got)
	}
}

func TestFetchOutlookOwner_GraphUsesGivenAndSurname(t *testing.T) {
	identityServer(t, map[string]any{
		"/graph/me": map[string]any{"mail": "ana@contoso.com", "givenName": "Ana", "surname": "Silva", "displayName": "Silva, Ana (Sales)"},
	})
	cfg := &oauth2.Config{Scopes: []string{"openid", config.GraphUserReadScope}}
	owner, xerr := fetchInboxOwner(context.Background(), models.InboxProviderOutlook, cfg, &oauth2.Token{AccessToken: "t"})
	if xerr != nil {
		t.Fatalf("fetchInboxOwner: %v", xerr)
	}
	if owner.Email != "ana@contoso.com" || owner.Name != "Ana Silva" {
		t.Fatalf("owner = %+v, want Ana Silva <ana@contoso.com>", owner)
	}
}

func idToken(t *testing.T, claims map[string]any) string {
	t.Helper()
	payload, _ := json.Marshal(claims)
	return "eyJhbGciOiJub25lIn0." + base64.RawURLEncoding.EncodeToString(payload) + "."
}

func TestFetchOutlookOwner_OIDCReadsIDTokenThenUserinfo(t *testing.T) {
	// The IMAP/SMTP consent carries no Graph scope. The id_token names the
	// account; the split name comes from the OIDC userinfo endpoint, reached
	// with a token the refresh token buys from the token endpoint.
	identityServer(t, map[string]any{
		"/ms/userinfo": map[string]any{"email": "ana@contoso.com", "given_name": "Ana", "family_name": "Silva"},
	})
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		if r.FormValue("grant_type") != "refresh_token" || r.FormValue("scope") != "openid email profile" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "userinfo-token", "token_type": "Bearer"})
	})
	tokenSrv := httptest.NewServer(mux)
	defer tokenSrv.Close()

	cfg := &oauth2.Config{
		ClientID: "public-client",
		Scopes:   []string{"openid", "email", "profile", "offline_access", config.OutlookIMAPScope, config.OutlookSMTPScope},
		Endpoint: oauth2.Endpoint{TokenURL: tokenSrv.URL + "/token"},
	}
	if config.OutlookGraphScoped(cfg) {
		t.Fatal("IMAP/SMTP scopes must not read as Graph")
	}
	tok := (&oauth2.Token{AccessToken: "outlook-token", RefreshToken: "r"}).WithExtra(map[string]any{
		"id_token": idToken(t, map[string]any{"preferred_username": "ana@contoso.com", "name": "Ana Silva"}),
	})
	owner, xerr := fetchInboxOwner(context.Background(), models.InboxProviderOutlook, cfg, tok)
	if xerr != nil {
		t.Fatalf("fetchInboxOwner: %v", xerr)
	}
	if owner.Email != "ana@contoso.com" || owner.Name != "Ana Silva" {
		t.Fatalf("owner = %+v, want Ana Silva <ana@contoso.com>", owner)
	}
}

func TestFetchOutlookOwner_OIDCFallsBackToIDTokenName(t *testing.T) {
	// No refresh token, so userinfo is out of reach: the id_token's display
	// name is still better than the local part.
	identityServer(t, map[string]any{})
	cfg := &oauth2.Config{Scopes: []string{"openid", config.OutlookIMAPScope}}
	tok := (&oauth2.Token{AccessToken: "outlook-token"}).WithExtra(map[string]any{
		"id_token": idToken(t, map[string]any{"email": "ana@contoso.com", "name": "Ana Silva"}),
	})
	owner, xerr := fetchInboxOwner(context.Background(), models.InboxProviderOutlook, cfg, tok)
	if xerr != nil {
		t.Fatalf("fetchInboxOwner: %v", xerr)
	}
	if owner.Email != "ana@contoso.com" || owner.Name != "Ana Silva" {
		t.Fatalf("owner = %+v", owner)
	}
}

func TestIsFallbackName(t *testing.T) {
	cases := []struct {
		name, email string
		want        bool
	}{
		{"", "ana.silva@gmail.com", true},
		{"Ana Silva", "ana.silva@gmail.com", true}, // exactly what deriveNameFromEmail yields
		{"ana silva", "ana.silva@gmail.com", true}, // case-insensitive
		{"ana.silva@gmail.com", "ana.silva@gmail.com", true},
		{"Ana Silva Ferreira", "ana.silva@gmail.com", false},
		{"Sales Team", "ana.silva@gmail.com", false},
	}
	for _, c := range cases {
		if got := isFallbackName(c.name, c.email); got != c.want {
			t.Errorf("isFallbackName(%q, %q) = %v, want %v", c.name, c.email, got, c.want)
		}
	}
}
