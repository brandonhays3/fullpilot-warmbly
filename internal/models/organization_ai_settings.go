package models

import (
	"time"

	"github.com/google/uuid"
)

// AIProviderOpenRouter is the only provider a workspace can bring its own key
// for. One OpenAI-compatible endpoint fronting every vendor's models keeps the
// model picker a single list and the key a single secret.
const AIProviderOpenRouter = "openrouter"

// OrganizationAISettings is the stored row: the sealed key, its last four
// characters for display, and the workspace's default model.
type OrganizationAISettings struct {
	OrganizationID   uuid.UUID
	Provider         string
	APIKeyCiphertext *string
	APIKeyLast4      string
	Model            string
	UpdatedAt        time.Time
}

// AISettingsView is what the API returns: never the key, only whether one is
// set and how it ends.
type AISettingsView struct {
	Provider  string     `json:"provider"`
	HasKey    bool       `json:"has_key"`
	KeyLast4  string     `json:"key_last4"`
	Model     string     `json:"model"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
}

// AIModel is one entry of the provider's model catalog, with its prices
// normalised to USD per million tokens so a picker can compare them.
type AIModel struct {
	ID                   string  `json:"id"`
	Name                 string  `json:"name"`
	ContextLength        int     `json:"context_length"`
	PromptPerMillion     float64 `json:"prompt_per_million"`
	CompletionPerMillion float64 `json:"completion_per_million"`
}
