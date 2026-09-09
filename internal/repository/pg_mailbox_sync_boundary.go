package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/warmbly/warmbly/internal/infrastructure/db"
)

// MailboxSyncBoundaryRepository keeps the sync start boundary of every
// address an organization has connected: the moment it was first connected.
// Nothing received before it is imported or processed. Keyed by the address
// rather than the account row, so disconnecting and reconnecting keeps the
// original boundary instead of letting the reconnect pull in older history.
type MailboxSyncBoundaryRepository interface {
	// Ensure returns the boundary for the address, writing since as the
	// boundary when the address has none. An existing boundary is never
	// moved.
	Ensure(ctx context.Context, organizationID uuid.UUID, email string, since time.Time) (time.Time, error)
	// Get returns nil, nil when the address has no boundary.
	Get(ctx context.Context, organizationID uuid.UUID, email string) (*time.Time, error)
}

type pgMailboxSyncBoundaryRepository struct {
	db *db.DB
}

func NewMailboxSyncBoundaryRepository(d *db.DB) MailboxSyncBoundaryRepository {
	return &pgMailboxSyncBoundaryRepository{db: d}
}

func normalizeBoundaryEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func (r *pgMailboxSyncBoundaryRepository) Ensure(ctx context.Context, organizationID uuid.UUID, email string, since time.Time) (time.Time, error) {
	// DO UPDATE with a no-op assignment makes RETURNING yield the stored row
	// on conflict, so one round trip answers both the insert and the read.
	const q = `
		INSERT INTO mailbox_sync_boundaries (organization_id, email, sync_since)
		VALUES ($1, $2, $3)
		ON CONFLICT (organization_id, email) DO UPDATE SET email = EXCLUDED.email
		RETURNING sync_since
	`
	var stored time.Time
	if err := r.db.QueryRow(ctx, q, organizationID, normalizeBoundaryEmail(email), since.UTC()).Scan(&stored); err != nil {
		return time.Time{}, fmt.Errorf("mailbox_sync_boundaries: ensure: %w", err)
	}
	return stored, nil
}

func (r *pgMailboxSyncBoundaryRepository) Get(ctx context.Context, organizationID uuid.UUID, email string) (*time.Time, error) {
	const q = `SELECT sync_since FROM mailbox_sync_boundaries WHERE organization_id = $1 AND email = $2`
	var stored time.Time
	err := r.db.QueryRow(ctx, q, organizationID, normalizeBoundaryEmail(email)).Scan(&stored)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("mailbox_sync_boundaries: get: %w", err)
	}
	return &stored, nil
}
