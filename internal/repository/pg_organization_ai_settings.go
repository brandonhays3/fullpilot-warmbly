package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/warmbly/warmbly/internal/infrastructure/db"
	"github.com/warmbly/warmbly/internal/models"
)

// OrganizationAISettingsRepository persists a workspace's own AI provider key
// and default model. A missing row means "no key, default model".
type OrganizationAISettingsRepository interface {
	Get(ctx context.Context, orgID uuid.UUID) (*models.OrganizationAISettings, error)
	// SetKey stores a new sealed key (and its last four characters), creating
	// the row if needed. The model is left as it is.
	SetKey(ctx context.Context, orgID uuid.UUID, ciphertext, last4 string) error
	// SetModel stores the default model, creating the row if needed.
	SetModel(ctx context.Context, orgID uuid.UUID, model string) error
	// ClearKey removes the key but keeps the model choice.
	ClearKey(ctx context.Context, orgID uuid.UUID) error
}

type organizationAISettingsRepository struct {
	DB *db.DB
}

func NewOrganizationAISettingsRepository(database *db.DB) OrganizationAISettingsRepository {
	return &organizationAISettingsRepository{DB: database}
}

const orgAISettingsCols = `organization_id, provider, api_key_ciphertext, api_key_last4, model, updated_at`

func (r *organizationAISettingsRepository) Get(ctx context.Context, orgID uuid.UUID) (*models.OrganizationAISettings, error) {
	s := &models.OrganizationAISettings{}
	err := r.DB.QueryRow(ctx, `SELECT `+orgAISettingsCols+` FROM organization_ai_settings WHERE organization_id = $1`, orgID).
		Scan(&s.OrganizationID, &s.Provider, &s.APIKeyCiphertext, &s.APIKeyLast4, &s.Model, &s.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return s, nil
}

func (r *organizationAISettingsRepository) SetKey(ctx context.Context, orgID uuid.UUID, ciphertext, last4 string) error {
	_, err := r.DB.Exec(ctx, `
		INSERT INTO organization_ai_settings (organization_id, api_key_ciphertext, api_key_last4)
		VALUES ($1, $2, $3)
		ON CONFLICT (organization_id) DO UPDATE SET
			api_key_ciphertext = EXCLUDED.api_key_ciphertext,
			api_key_last4 = EXCLUDED.api_key_last4,
			updated_at = now()`, orgID, ciphertext, last4)
	return err
}

func (r *organizationAISettingsRepository) SetModel(ctx context.Context, orgID uuid.UUID, model string) error {
	_, err := r.DB.Exec(ctx, `
		INSERT INTO organization_ai_settings (organization_id, model)
		VALUES ($1, $2)
		ON CONFLICT (organization_id) DO UPDATE SET
			model = EXCLUDED.model,
			updated_at = now()`, orgID, model)
	return err
}

func (r *organizationAISettingsRepository) ClearKey(ctx context.Context, orgID uuid.UUID) error {
	_, err := r.DB.Exec(ctx, `
		UPDATE organization_ai_settings
		SET api_key_ciphertext = NULL, api_key_last4 = '', updated_at = now()
		WHERE organization_id = $1`, orgID)
	return err
}
