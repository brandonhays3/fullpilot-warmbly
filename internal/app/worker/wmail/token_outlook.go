package wmail

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/warmbly/warmbly/internal/config"
	"golang.org/x/oauth2"
)

// outlookTokenURL is where refresh tokens are redeemed. A variable so tests
// can point it at a local server; empty means the config's own endpoint.
var outlookTokenURL = ""

// outlookTokenClient bounds one token request.
var outlookTokenClient = &http.Client{Timeout: 15 * time.Second}

// outlookTokens mints Microsoft access tokens per resource from one refresh
// token. Entra issues one access token per resource, so the Graph client and
// the IMAP/SMTP clients each need their own, and a token request must name
// the resource's scopes explicitly: the stored access token is only ever a
// Graph token (the code is redeemed for one) and cannot be assumed to be for
// either resource on load, so both are minted here and cached until expiry.
//
// Entra rotates the refresh token on use. The newest one is kept for every
// later mint, and persisted with the Graph token so the next load starts from
// a token that has not expired for want of use.
type outlookTokens struct {
	ctx     context.Context
	cfg     oauth2.Config
	persist func(*oauth2.Token) error

	mu      sync.Mutex
	refresh string
	graph   *oauth2.Token
	mail    *oauth2.Token
}

func newOutlookTokens(ctx context.Context, cfg oauth2.Config, refresh string, persist func(*oauth2.Token) error) *outlookTokens {
	return &outlookTokens{ctx: ctx, cfg: cfg, refresh: refresh, persist: persist}
}

// tokenSourceFunc adapts a func to oauth2.TokenSource.
type tokenSourceFunc func() (*oauth2.Token, error)

func (f tokenSourceFunc) Token() (*oauth2.Token, error) { return f() }

// GraphSource and MailSource are the per-resource token sources for the Graph
// client and for the IMAP and SMTP clients.
func (o *outlookTokens) GraphSource() oauth2.TokenSource { return tokenSourceFunc(o.Graph) }
func (o *outlookTokens) MailSource() oauth2.TokenSource  { return tokenSourceFunc(o.Mail) }

// Graph returns a Graph token, minting one when the cache is empty or within
// a minute of expiry.
func (o *outlookTokens) Graph() (*oauth2.Token, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if fresh(o.graph) {
		return o.graph, nil
	}
	if err := o.mintGraphLocked(); err != nil {
		return nil, err
	}
	return o.graph, nil
}

// Mail returns an outlook.office.com token the same way.
func (o *outlookTokens) Mail() (*oauth2.Token, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if fresh(o.mail) {
		return o.mail, nil
	}
	tok, rotated, err := o.mint(config.OutlookMailTokenScopes())
	if err != nil {
		return nil, err
	}
	o.mail = tok
	// A rotated refresh token is only persisted alongside a Graph token,
	// which is what the stored access token must always be; mint one now so
	// the rotation does not wait for the next Graph call.
	if rotated {
		if err := o.mintGraphLocked(); err != nil {
			log.Warn().Err(err).Msg("outlook: could not refresh the graph token after a rotation; the rotated refresh token is kept in memory")
		}
	}
	return o.mail, nil
}

func (o *outlookTokens) mintGraphLocked() error {
	tok, _, err := o.mint(config.OutlookGraphTokenScopes())
	if err != nil {
		return err
	}
	o.graph = tok
	if o.persist != nil {
		if perr := o.persist(&oauth2.Token{
			AccessToken:  tok.AccessToken,
			RefreshToken: o.refresh,
			Expiry:       tok.Expiry,
			TokenType:    "Bearer",
		}); perr != nil {
			log.Warn().Err(perr).Msg("outlook: could not persist the refreshed token")
		}
	}
	return nil
}

// fresh reports whether a cached token has more than a minute left.
func fresh(t *oauth2.Token) bool {
	return t != nil && t.AccessToken != "" && time.Until(t.Expiry) > time.Minute
}

// mint redeems the refresh token for one resource. Caller holds mu. Reports
// whether Entra rotated the refresh token.
func (o *outlookTokens) mint(scopes []string) (*oauth2.Token, bool, error) {
	if o.refresh == "" {
		return nil, false, errors.New("outlook: no refresh token")
	}
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {o.refresh},
		"client_id":     {o.cfg.ClientID},
		"scope":         {strings.Join(scopes, " ")},
	}
	if o.cfg.ClientSecret != "" {
		form.Set("client_secret", o.cfg.ClientSecret)
	}
	endpoint := outlookTokenURL
	if endpoint == "" {
		endpoint = o.cfg.Endpoint.TokenURL
	}
	req, err := http.NewRequestWithContext(o.ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := outlookTokenClient.Do(req)
	if err != nil {
		return nil, false, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return nil, false, &oauth2.RetrieveError{Response: resp, Body: body}
	}
	var out struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, false, fmt.Errorf("outlook: token response: %w", err)
	}
	if out.AccessToken == "" {
		return nil, false, errors.New("outlook: token response without access_token")
	}
	rotated := out.RefreshToken != "" && out.RefreshToken != o.refresh
	if rotated {
		o.refresh = out.RefreshToken
	}
	expiry := time.Now().Add(time.Hour)
	if out.ExpiresIn > 0 {
		expiry = time.Now().Add(time.Duration(out.ExpiresIn) * time.Second)
	}
	return &oauth2.Token{AccessToken: out.AccessToken, TokenType: "Bearer", Expiry: expiry}, rotated, nil
}
