// Package aisettings holds a workspace's own AI provider key and default model.
// Every user-facing generation path (AI blocks in campaign copy, the writing
// assistant, reply and compose drafts) resolves the workspace key here instead
// of the platform's AI_* environment, so the workspace pays OpenRouter directly
// and the platform key is left to reply classification.
package aisettings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/warmbly/warmbly/internal/app/cipher"
	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/pkg/generation"
	"github.com/warmbly/warmbly/internal/repository"
)

// DefaultModel is the model an AI block runs on when neither the block nor the
// workspace names one.
const DefaultModel = "openai/gpt-4o-mini"

var (
	// ErrKeyMissing means the workspace has not saved an OpenRouter key.
	ErrKeyMissing = errors.New("the workspace has no OpenRouter key")
	// ErrKeyInvalid means OpenRouter refused the key when it was saved.
	ErrKeyInvalid = errors.New("OpenRouter rejected the API key")
	// ErrModelInvalid means the model id is not something OpenRouter can route.
	ErrModelInvalid = errors.New("invalid model id")
)

const (
	maxKeyLen   = 512
	maxModelLen = 200
	// modelsCacheTTL bounds how stale the model catalog can be; the list is
	// the same for every key so one cache serves every workspace.
	modelsCacheTTL = 10 * time.Minute
	// providerCacheTTL bounds how long a built provider is reused before the
	// stored row is re-read, so a key replaced by another process is picked
	// up within a minute even without an invalidation.
	providerCacheTTL  = time.Minute
	openRouterTimeout = 15 * time.Second
)

// Resolved is a workspace's provider, ready to call, plus the model it
// defaults to.
type Resolved struct {
	Provider generation.Provider
	// Writing is the same provider through the writing-assistant contract.
	Writing generation.WritingGenerator
	Model   string
}

// Service is the read/write surface for a workspace's AI settings and the
// resolver every generation path goes through.
type Service interface {
	Get(ctx context.Context, orgID uuid.UUID) (*models.AISettingsView, error)
	// SetKey validates the key against OpenRouter, seals it with the
	// organization DEK and stores it.
	SetKey(ctx context.Context, orgID uuid.UUID, key string) (*models.AISettingsView, error)
	// SetModel stores the workspace default model; "" restores DefaultModel.
	SetModel(ctx context.Context, orgID uuid.UUID, model string) (*models.AISettingsView, error)
	// ClearKey removes the key. The model choice is kept.
	ClearKey(ctx context.Context, orgID uuid.UUID) error
	HasKey(ctx context.Context, orgID uuid.UUID) (bool, error)
	// Resolve returns the workspace's provider or ErrKeyMissing.
	Resolve(ctx context.Context, orgID uuid.UUID) (*Resolved, error)
	// ListModels proxies OpenRouter's model catalog with the workspace key.
	// Returns ErrKeyMissing without one.
	ListModels(ctx context.Context, orgID uuid.UUID) ([]models.AIModel, error)
}

type service struct {
	repo   repository.OrganizationAISettingsRepository
	cipher cipher.CipherService
	search generation.SearchClient
	http   *http.Client
	// baseURL is OpenRouter's API root; overridable for tests.
	baseURL string

	modelsMu      sync.Mutex
	models        []models.AIModel
	modelsFetched time.Time

	providersMu sync.Mutex
	providers   map[uuid.UUID]providerEntry
}

type providerEntry struct {
	ciphertext string
	model      string
	resolved   *Resolved
	expires    time.Time
}

// New builds the service. search backs the provider's search_web tool and may
// be nil.
func New(repo repository.OrganizationAISettingsRepository, c cipher.CipherService, search generation.SearchClient) Service {
	return &service{
		repo:      repo,
		cipher:    c,
		search:    search,
		http:      &http.Client{Timeout: openRouterTimeout},
		baseURL:   generation.OpenRouterBaseURL,
		providers: map[uuid.UUID]providerEntry{},
	}
}

func view(row *models.OrganizationAISettings) *models.AISettingsView {
	v := &models.AISettingsView{Provider: models.AIProviderOpenRouter, Model: DefaultModel}
	if row == nil {
		return v
	}
	v.HasKey = row.APIKeyCiphertext != nil && *row.APIKeyCiphertext != ""
	v.KeyLast4 = row.APIKeyLast4
	if strings.TrimSpace(row.Model) != "" {
		v.Model = row.Model
	}
	t := row.UpdatedAt
	v.UpdatedAt = &t
	return v
}

func (s *service) Get(ctx context.Context, orgID uuid.UUID) (*models.AISettingsView, error) {
	row, err := s.repo.Get(ctx, orgID)
	if err != nil {
		return nil, err
	}
	return view(row), nil
}

func (s *service) HasKey(ctx context.Context, orgID uuid.UUID) (bool, error) {
	row, err := s.repo.Get(ctx, orgID)
	if err != nil {
		return false, err
	}
	return row != nil && row.APIKeyCiphertext != nil && *row.APIKeyCiphertext != "", nil
}

func (s *service) SetKey(ctx context.Context, orgID uuid.UUID, key string) (*models.AISettingsView, error) {
	key = strings.TrimSpace(key)
	if key == "" || len(key) > maxKeyLen || strings.ContainsAny(key, " \t\r\n") {
		return nil, ErrKeyInvalid
	}
	if err := s.verifyKey(ctx, key); err != nil {
		return nil, err
	}
	c, err := s.cipher.Cipher(ctx, orgID)
	if err != nil {
		return nil, err
	}
	sealed, err := c.Encrypt(ctx, key)
	if err != nil {
		return nil, err
	}
	last4 := key
	if len(last4) > 4 {
		last4 = last4[len(last4)-4:]
	}
	if err := s.repo.SetKey(ctx, orgID, sealed, last4); err != nil {
		return nil, err
	}
	s.forget(orgID)
	return s.Get(ctx, orgID)
}

func (s *service) SetModel(ctx context.Context, orgID uuid.UUID, model string) (*models.AISettingsView, error) {
	model = strings.TrimSpace(model)
	if len(model) > maxModelLen || strings.ContainsAny(model, " \t\r\n") {
		return nil, ErrModelInvalid
	}
	if err := s.repo.SetModel(ctx, orgID, model); err != nil {
		return nil, err
	}
	s.forget(orgID)
	return s.Get(ctx, orgID)
}

func (s *service) ClearKey(ctx context.Context, orgID uuid.UUID) error {
	if err := s.repo.ClearKey(ctx, orgID); err != nil {
		return err
	}
	s.forget(orgID)
	return nil
}

func (s *service) forget(orgID uuid.UUID) {
	s.providersMu.Lock()
	delete(s.providers, orgID)
	s.providersMu.Unlock()
}

// Resolve reads the stored row, opens the key and builds the provider. The
// built provider is reused while the row is unchanged so the provider's own
// parameter-compatibility state survives across calls.
func (s *service) Resolve(ctx context.Context, orgID uuid.UUID) (*Resolved, error) {
	row, err := s.repo.Get(ctx, orgID)
	if err != nil {
		return nil, err
	}
	if row == nil || row.APIKeyCiphertext == nil || *row.APIKeyCiphertext == "" {
		return nil, ErrKeyMissing
	}
	model := strings.TrimSpace(row.Model)
	if model == "" {
		model = DefaultModel
	}

	s.providersMu.Lock()
	entry, ok := s.providers[orgID]
	s.providersMu.Unlock()
	if ok && entry.ciphertext == *row.APIKeyCiphertext && entry.model == model && time.Now().Before(entry.expires) {
		return entry.resolved, nil
	}

	c, err := s.cipher.Cipher(ctx, orgID)
	if err != nil {
		return nil, err
	}
	key, err := c.Decrypt(ctx, *row.APIKeyCiphertext)
	if err != nil {
		return nil, err
	}
	// Local marks the provider un-metered: the workspace pays OpenRouter
	// directly, so no credit is charged for anything it generates.
	provider, err := generation.NewProvider(generation.ProviderConfig{
		OpenAIAPIKey:     key,
		OpenAIBaseURL:    s.baseURL,
		OpenAIModelTrial: model,
		OpenAIModelPaid:  model,
		Local:            true,
		Search:           s.search,
	})
	if err != nil {
		return nil, err
	}
	writing, _ := provider.(generation.WritingGenerator)
	resolved := &Resolved{Provider: provider, Writing: writing, Model: model}

	s.providersMu.Lock()
	s.providers[orgID] = providerEntry{ciphertext: *row.APIKeyCiphertext, model: model, resolved: resolved, expires: time.Now().Add(providerCacheTTL)}
	s.providersMu.Unlock()
	return resolved, nil
}

// verifyKey asks OpenRouter whether the key is accepted. A refusal is the
// caller's typo to fix now; an unreachable OpenRouter is not, so only a clear
// 401/403 blocks the save.
func (s *service) verifyKey(ctx context.Context, key string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.baseURL+"/auth/key", nil)
	if err != nil {
		return nil
	}
	req.Header.Set("Authorization", "Bearer "+key)
	resp, err := s.http.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return ErrKeyInvalid
	}
	return nil
}

// openRouterModel is the subset of OpenRouter's /models entry the picker needs.
// Prices arrive as decimal strings in USD per token.
type openRouterModel struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	ContextLength *int   `json:"context_length"`
	Pricing       struct {
		Prompt     string `json:"prompt"`
		Completion string `json:"completion"`
	} `json:"pricing"`
}

func (s *service) ListModels(ctx context.Context, orgID uuid.UUID) ([]models.AIModel, error) {
	row, err := s.repo.Get(ctx, orgID)
	if err != nil {
		return nil, err
	}
	if row == nil || row.APIKeyCiphertext == nil || *row.APIKeyCiphertext == "" {
		return nil, ErrKeyMissing
	}

	s.modelsMu.Lock()
	defer s.modelsMu.Unlock()
	if s.models != nil && time.Since(s.modelsFetched) < modelsCacheTTL {
		return s.models, nil
	}

	c, err := s.cipher.Cipher(ctx, orgID)
	if err != nil {
		return nil, err
	}
	key, err := c.Decrypt(ctx, *row.APIKeyCiphertext)
	if err != nil {
		return nil, err
	}
	list, err := s.fetchModels(ctx, key)
	if err != nil {
		return nil, err
	}
	s.models = list
	s.modelsFetched = time.Now()
	return list, nil
}

func (s *service) fetchModels(ctx context.Context, key string) ([]models.AIModel, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.baseURL+"/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openrouter models: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, generation.ErrProviderAuth
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("openrouter models: unexpected status %d", resp.StatusCode)
	}
	var body struct {
		Data []openRouterModel `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 16<<20)).Decode(&body); err != nil {
		return nil, fmt.Errorf("openrouter models: decode: %w", err)
	}
	out := make([]models.AIModel, 0, len(body.Data))
	for _, m := range body.Data {
		if strings.TrimSpace(m.ID) == "" {
			continue
		}
		entry := models.AIModel{ID: m.ID, Name: m.Name, PromptPerMillion: perMillion(m.Pricing.Prompt), CompletionPerMillion: perMillion(m.Pricing.Completion)}
		if m.ContextLength != nil {
			entry.ContextLength = *m.ContextLength
		}
		if entry.Name == "" {
			entry.Name = entry.ID
		}
		out = append(out, entry)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// perMillion turns OpenRouter's per-token price string into USD per million
// tokens. Anything unparsable (or a negative "dynamic" price) reads as zero.
func perMillion(perToken string) float64 {
	f, err := strconv.ParseFloat(strings.TrimSpace(perToken), 64)
	if err != nil || f < 0 {
		return 0
	}
	return f * 1_000_000
}
