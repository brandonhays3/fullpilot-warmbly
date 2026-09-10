package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/infrastructure/db"
	"github.com/warmbly/warmbly/internal/models"
)

// sendMethodSelect groups sent progress rows by the method their confirmed
// task recorded. A step can have several campaign_tasks rows (a failed
// attempt and its retry), and only the one the worker confirmed carries a
// method, so the lateral lookup takes the stamped one; a step with none is
// the empty label. Outcomes come from the progress row, the same source the
// campaign summary reads, so the buckets add up to its sent total.
const sendMethodSelect = `
	SELECT
		COALESCE(m.send_method, '') AS send_method,
		COUNT(*) AS sent,
		COUNT(*) FILTER (WHERE ccp.bounced_at IS NULL) AS delivered,
		COUNT(*) FILTER (WHERE ccp.opened_at IS NOT NULL) AS opened,
		COUNT(*) FILTER (WHERE ccp.replied_at IS NOT NULL) AS replied,
		COUNT(*) FILTER (WHERE ccp.bounced_at IS NOT NULL) AS bounced
	FROM campaign_contact_progress ccp
	JOIN campaigns c ON c.id = ccp.campaign_id
	LEFT JOIN LATERAL (
		SELECT ct.send_method
		FROM campaign_tasks ct
		WHERE ct.campaign_id = ccp.campaign_id
		  AND ct.contact_id = ccp.contact_id
		  AND ct.sequence_id = ccp.sequence_id
		  AND ct.send_method IS NOT NULL
		LIMIT 1
	) m ON TRUE
	WHERE ccp.sent_at IS NOT NULL
`

const sendMethodGroup = `
	GROUP BY 1
	ORDER BY sent DESC, send_method ASC
`

func (r *analyticsRepository) GetCampaignSendMethodStats(ctx context.Context, campaignID uuid.UUID) ([]models.SendMethodStats, *errx.Error) {
	query := sendMethodSelect + ` AND ccp.campaign_id = $1` + sendMethodGroup
	params := []any{campaignID}
	rows, err := r.DB.Query(ctx, query, params...)
	if err != nil {
		db.CaptureError(err, query, params, "query")
		return nil, errx.InternalError()
	}
	defer rows.Close()
	return scanSendMethodStats(rows, query, params)
}

func (r *analyticsRepository) GetWorkspaceSendMethodStats(ctx context.Context, orgID uuid.UUID, from, to time.Time) ([]models.SendMethodStats, *errx.Error) {
	query := sendMethodSelect + ` AND c.organization_id = $1 AND ccp.sent_at >= $2 AND ccp.sent_at <= $3` + sendMethodGroup
	params := []any{orgID, from, to}
	rows, err := r.DB.Query(ctx, query, params...)
	if err != nil {
		db.CaptureError(err, query, params, "query")
		return nil, errx.InternalError()
	}
	defer rows.Close()
	return scanSendMethodStats(rows, query, params)
}

func scanSendMethodStats(rows pgx.Rows, query string, params []any) ([]models.SendMethodStats, *errx.Error) {
	out := make([]models.SendMethodStats, 0, 4)
	for rows.Next() {
		var s models.SendMethodStats
		if err := rows.Scan(&s.SendMethod, &s.Sent, &s.Delivered, &s.Opened, &s.Replied, &s.Bounced); err != nil {
			db.CaptureError(err, query, params, "scan")
			return nil, errx.InternalError()
		}
		s.FillRates()
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		db.CaptureError(err, query, params, "rows")
		return nil, errx.InternalError()
	}
	return out, nil
}
