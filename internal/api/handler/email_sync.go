package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/warmbly/warmbly/internal/api/middleware"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
)

// emailSyncResponse is GET /emails/:id/sync: where the mailbox's import
// stands, whether fair use is holding it, and the budget it runs under.
type emailSyncResponse struct {
	// State is null until the worker has reported once.
	State  *models.SyncState `json:"state"`
	Policy models.SyncPolicy `json:"policy"`
	// Worker is where the mailbox runs right now; null while unplaced.
	Worker *emailSyncWorker `json:"worker"`
}

// emailSyncWorker is the placement shown in the mailbox drawer. Ephemeral
// workers are the short-lived ones SMTP mailboxes rotate across.
type emailSyncWorker struct {
	ID         uuid.UUID               `json:"id"`
	Deployment models.WorkerDeployment `json:"deployment"`
	Ephemeral  bool                    `json:"ephemeral"`
	StartedAt  *time.Time              `json:"started_at,omitempty"`
	LastSeenAt *time.Time              `json:"last_seen_at,omitempty"`
}

// GetEmailSync reports a mailbox's sync progress and fair-use status.
func (h *Handler) GetEmailSync(c *gin.Context) {
	userID, err := middleware.GetUserUUID(c)
	if err != nil {
		errx.JSON(c, errx.ErrUnauthorized)
		return
	}
	state, policy, xerr := h.EmailService.GetSyncState(c.Request.Context(), userID.String(), c.Param("id"))
	if xerr != nil {
		errx.JSON(c, xerr)
		return
	}
	c.JSON(http.StatusOK, emailSyncResponse{
		State:  state,
		Policy: policy,
		Worker: h.emailSyncWorker(c, userID.String(), c.Param("id")),
	})
}

// emailSyncWorker looks up the mailbox's worker for the drawer. Best effort:
// the sync card must not fail because the placement could not be read.
func (h *Handler) emailSyncWorker(c *gin.Context, userID, emailID string) *emailSyncWorker {
	if h.WorkerRepo == nil {
		return nil
	}
	acc, xerr := h.EmailService.Get(c.Request.Context(), userID, emailID)
	if xerr != nil || acc == nil || acc.WorkerID == nil {
		return nil
	}
	w, err := h.WorkerRepo.GetWorkerDetail(c.Request.Context(), *acc.WorkerID)
	if err != nil || w == nil {
		return nil
	}
	return &emailSyncWorker{
		ID:         w.ID,
		Deployment: w.Deployment,
		Ephemeral:  w.Deployment.IsEphemeral(),
		StartedAt:  w.StartedAt,
		LastSeenAt: w.LastSeenAt,
	}
}
