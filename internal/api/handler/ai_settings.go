// Workspace AI settings: the OpenRouter key every user-facing generation call
// runs on, and the workspace's default model. The key is written only; reads
// return whether one is set and its last four characters.

package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/warmbly/warmbly/internal/api/middleware"
	"github.com/warmbly/warmbly/internal/app/aisettings"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/pkg/generation"
)

type updateAISettingsRequest struct {
	// APIKey replaces the stored key when present and non-empty. Absent (or
	// empty) leaves the current key alone.
	APIKey *string `json:"api_key"`
	// Model sets the workspace default when present; "" restores the built-in
	// default. Absent leaves it alone.
	Model *string `json:"model"`
}

// GetAISettings — GET /organization/current/ai
func (h *Handler) GetAISettings(c *gin.Context) {
	orgID := middleware.GetOrganizationID(c)
	if orgID == nil {
		errx.JSON(c, errx.New(errx.BadRequest, "no organization selected"))
		return
	}
	if h.AISettingsService == nil {
		c.JSON(http.StatusOK, &models.AISettingsView{Provider: models.AIProviderOpenRouter, Model: aisettings.DefaultModel})
		return
	}
	view, err := h.AISettingsService.Get(c.Request.Context(), *orgID)
	if err != nil {
		errx.JSON(c, errx.InternalError())
		return
	}
	c.JSON(http.StatusOK, view)
}

// UpdateAISettings — PUT /organization/current/ai
func (h *Handler) UpdateAISettings(c *gin.Context) {
	orgID := middleware.GetOrganizationID(c)
	if orgID == nil {
		errx.JSON(c, errx.New(errx.BadRequest, "no organization selected"))
		return
	}
	if h.AISettingsService == nil {
		errx.JSON(c, errx.New(errx.ServiceUnavailable, "AI settings are not available on this deployment."))
		return
	}
	var req updateAISettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errx.JSON(c, errx.ErrInvalid)
		return
	}
	if req.APIKey == nil && req.Model == nil {
		errx.JSON(c, errx.New(errx.BadRequest, "nothing to update: send api_key, model, or both"))
		return
	}

	ctx := c.Request.Context()
	changes := map[string]string{}
	keySaved := false
	if req.APIKey != nil && strings.TrimSpace(*req.APIKey) != "" {
		if _, err := h.AISettingsService.SetKey(ctx, *orgID, *req.APIKey); err != nil {
			switch {
			case errors.Is(err, aisettings.ErrKeyInvalid):
				errx.JSON(c, errx.ErrAIKeyInvalid)
			default:
				errx.JSON(c, errx.InternalError())
			}
			return
		}
		changes["api_key"] = "replaced"
		keySaved = true
	}
	if req.Model != nil {
		if _, err := h.AISettingsService.SetModel(ctx, *orgID, *req.Model); err != nil {
			switch {
			case errors.Is(err, aisettings.ErrModelInvalid):
				errx.JSON(c, errx.ErrAIModelInvalid)
			default:
				errx.JSON(c, errx.InternalError())
			}
			return
		}
		changes["model"] = strings.TrimSpace(*req.Model)
	}

	view, err := h.AISettingsService.Get(ctx, *orgID)
	if err != nil {
		errx.JSON(c, errx.InternalError())
		return
	}
	h.auditOrg(c, models.AuditActionUpdate, models.AuditEntitySettings, nil, changes, map[string]string{"ai": "settings"})

	// Campaigns that parked for want of a key can go again now that there is one.
	if keySaved && h.CampaignService != nil {
		h.CampaignService.ResumeAIKeyPaused(ctx, *orgID)
	}
	c.JSON(http.StatusOK, view)
}

// DeleteAISettings — DELETE /organization/current/ai (removes the key; the
// model choice stays).
func (h *Handler) DeleteAISettings(c *gin.Context) {
	orgID := middleware.GetOrganizationID(c)
	if orgID == nil {
		errx.JSON(c, errx.New(errx.BadRequest, "no organization selected"))
		return
	}
	if h.AISettingsService == nil {
		errx.JSON(c, errx.New(errx.ServiceUnavailable, "AI settings are not available on this deployment."))
		return
	}
	if err := h.AISettingsService.ClearKey(c.Request.Context(), *orgID); err != nil {
		errx.JSON(c, errx.InternalError())
		return
	}
	h.auditOrg(c, models.AuditActionUpdate, models.AuditEntitySettings, nil, map[string]string{"api_key": "removed"}, map[string]string{"ai": "settings"})
	view, err := h.AISettingsService.Get(c.Request.Context(), *orgID)
	if err != nil {
		errx.JSON(c, errx.InternalError())
		return
	}
	c.JSON(http.StatusOK, view)
}

// ListAIModels — GET /organization/current/ai/models
func (h *Handler) ListAIModels(c *gin.Context) {
	orgID := middleware.GetOrganizationID(c)
	if orgID == nil {
		errx.JSON(c, errx.New(errx.BadRequest, "no organization selected"))
		return
	}
	if h.AISettingsService == nil {
		errx.JSON(c, errx.ErrAIKeyMissing)
		return
	}
	list, err := h.AISettingsService.ListModels(c.Request.Context(), *orgID)
	if err != nil {
		switch {
		case errors.Is(err, aisettings.ErrKeyMissing):
			errx.JSON(c, errx.ErrAIKeyMissing)
		case errors.Is(err, generation.ErrProviderAuth):
			errx.JSON(c, errx.ErrAIProviderRejected)
		default:
			errx.JSON(c, errx.New(errx.ServiceUnavailable, "OpenRouter's model list could not be fetched right now."))
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": list})
}

// orgAI resolves the workspace's own provider for a user-facing generation
// call. Without a key it answers ai_key_missing (409) and returns false; the
// platform's AI_* key is never used in its place.
func (h *Handler) orgAI(c *gin.Context, orgID uuid.UUID) (*aisettings.Resolved, bool) {
	if h.AISettingsService == nil {
		errx.JSON(c, errx.ErrAIKeyMissing)
		return nil, false
	}
	resolved, err := h.AISettingsService.Resolve(c.Request.Context(), orgID)
	if err != nil {
		if errors.Is(err, aisettings.ErrKeyMissing) {
			errx.JSON(c, errx.ErrAIKeyMissing)
			return nil, false
		}
		errx.JSON(c, errx.New(errx.ServiceUnavailable, "The workspace's AI settings could not be read right now."))
		return nil, false
	}
	return resolved, true
}

// aiGenerationError maps a provider failure to the response: a refused key is
// a configuration problem the caller can fix; anything else is transient.
func aiGenerationError(c *gin.Context, gerr error, transient string) {
	if errors.Is(gerr, generation.ErrProviderAuth) {
		errx.JSON(c, errx.ErrAIProviderRejected)
		return
	}
	errx.JSON(c, errx.New(errx.ServiceUnavailable, transient))
}
