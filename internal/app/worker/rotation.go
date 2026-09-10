// SMTP rotation across ephemeral workers.
//
// An SMTP/IMAP mailbox sends through the worker's own egress IP, so on a
// deployment where workers are short-lived Cloud Run tasks (a fresh process
// and a fresh IP every run) successive sends can leave from different IPs by
// moving the mailbox between live instances. Gmail and Outlook mailboxes send
// from the provider's IPs whatever worker drives them, so they are never
// moved by this policy; the candidate query only returns smtp_imap rows.
//
// Two decisions live here, both pure enough to table-test:
//
//   - pickEphemeralWorker: which live ephemeral worker to place on. Newest
//     first (most remaining lifetime, fewest re-moves), but never more than
//     one mailbox ahead of the least-loaded live instance, which spreads a
//     batch round-robin across instances instead of piling it on the newest.
//   - PlanSmtpRotation: whether a placed mailbox should move now. It moves
//     off a persistent worker as soon as an ephemeral one is live, and off
//     an ephemeral worker once it has sat there for rotateAfter and another
//     ephemeral worker is live.
//
// A mailbox whose worker is gone is not planned here: the email service
// re-places it through the normal load path, which comes back to
// AssignWorkerToEmail and its ephemeral-first selection.

package worker

import (
	"context"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/repository"
)

// ephemeralCandidate is a live ephemeral worker with headroom, as seen by
// the placement rule.
type ephemeralCandidate struct {
	WorkerID  uuid.UUID
	StartedAt time.Time
	Load      float64
	Headroom  float64
}

// pickEphemeralWorker chooses among cands for a mailbox of the given weight,
// never returning exclude (the worker the mailbox is leaving). Candidates
// without headroom for the weight are skipped; the rest are walked newest
// first and the first one whose load is less than one mailbox above the
// least-loaded candidate wins. ok is false when nothing qualifies.
func pickEphemeralWorker(cands []ephemeralCandidate, weight float64, exclude uuid.UUID) (id uuid.UUID, ok bool) {
	fit := make([]ephemeralCandidate, 0, len(cands))
	for _, c := range cands {
		if c.WorkerID == exclude || c.Headroom < weight {
			continue
		}
		fit = append(fit, c)
	}
	if len(fit) == 0 {
		return uuid.Nil, false
	}
	sort.SliceStable(fit, func(i, j int) bool {
		if !fit[i].StartedAt.Equal(fit[j].StartedAt) {
			return fit[i].StartedAt.After(fit[j].StartedAt)
		}
		if fit[i].Load != fit[j].Load {
			return fit[i].Load < fit[j].Load
		}
		return fit[i].WorkerID.String() < fit[j].WorkerID.String()
	})
	minLoad := fit[0].Load
	for _, c := range fit[1:] {
		if c.Load < minLoad {
			minLoad = c.Load
		}
	}
	slack := weight
	if slack <= 0 {
		slack = defaultMailboxWeight
	}
	for _, c := range fit {
		if c.Load-minLoad < slack {
			return c.WorkerID, true
		}
	}
	return fit[0].WorkerID, true
}

// ephemeralCandidates reads the live, healthy shared workers of the tier
// from the capacity view and keeps the ephemeral ones, with their headroom
// computed the same way selectSharedWorkerForWeight does so both paths agree
// on "full".
func (s *workerAssignmentService) ephemeralCandidates(ctx context.Context, freeTier bool) ([]ephemeralCandidate, error) {
	rows, err := s.workerRepo.ListCapacityCandidates(ctx, freeTier, nil)
	if err != nil {
		return nil, err
	}
	out := make([]ephemeralCandidate, 0, len(rows))
	for _, row := range rows {
		if !row.Deployment.IsEphemeral() {
			continue
		}
		capacity := ComputeCapacity(WorkerCapacityRow{
			WorkerID:         row.WorkerID,
			WorkerType:       row.WorkerType,
			FreeTier:         row.FreeTier,
			EgressKind:       row.EgressKind,
			HealthState:      row.HealthState,
			LoadScore:        row.LoadScore,
			BaseCapacity:     row.BaseCapacity,
			HealthMultiplier: row.HealthMultiplier,
			AgeMultiplier:    row.AgeMultiplier,
		})
		var started time.Time
		if row.StartedAt != nil {
			started = *row.StartedAt
		}
		out = append(out, ephemeralCandidate{
			WorkerID:  row.WorkerID,
			StartedAt: started,
			Load:      capacity.Load,
			Headroom:  capacity.Effective - capacity.Load,
		})
	}
	return out, nil
}

// selectEphemeralWorker returns the live ephemeral shared worker of the tier
// an SMTP mailbox of the given weight should go to, or nil when none is live
// with headroom (the caller then falls back to the persistent shared pool).
func (s *workerAssignmentService) selectEphemeralWorker(ctx context.Context, freeTier bool, weight float64, exclude uuid.UUID) (*models.Worker, error) {
	cands, err := s.ephemeralCandidates(ctx, freeTier)
	if err != nil {
		return nil, err
	}
	id, ok := pickEphemeralWorker(cands, weight, exclude)
	if !ok {
		return nil, nil
	}
	return s.workerRepo.GetByID(ctx, id)
}

// ListSmtpRotationCandidates is the repository pass-through for the reconciler.
func (s *workerAssignmentService) ListSmtpRotationCandidates(ctx context.Context) ([]repository.SmtpRotationCandidate, error) {
	return s.workerRepo.ListSmtpRotationCandidates(ctx)
}

// PlanSmtpRotation decides whether cand should move now, and where to.
//
//   - no worker, or a worker that is not live: nil. The load path re-places
//     it (and prefers an ephemeral worker when it does).
//   - dedicated worker: nil. One customer per worker is the placement the
//     org pays for; it is not rotated.
//   - persistent worker: the best live ephemeral worker of the same tier, or
//     nil when none has headroom.
//   - ephemeral worker: nil until the mailbox has sat there for rotateAfter
//     (an unknown assignment time counts as expired, so a mailbox placed
//     before the clock existed starts one on its first move), then another
//     live ephemeral worker of the same tier, or nil when there is none.
func (s *workerAssignmentService) PlanSmtpRotation(ctx context.Context, cand repository.SmtpRotationCandidate, now time.Time, rotateAfter time.Duration) (*models.Worker, error) {
	if cand.WorkerID == nil || !cand.WorkerLive {
		return nil, nil
	}
	if cand.WorkerType == models.WorkerTypeDedicated {
		return nil, nil
	}
	if cand.WorkerDeployment.IsEphemeral() {
		if cand.WorkerAssignedAt != nil && now.Sub(*cand.WorkerAssignedAt) < rotateAfter {
			return nil, nil
		}
	}
	weight := s.resolveMailboxWeight(ctx, cand.AccountID)
	return s.selectEphemeralWorker(ctx, cand.WorkerFreeTier, weight, *cand.WorkerID)
}

// MoveEmailToWorker re-points a mailbox at newWorkerID, keeping both workers'
// account counts and load scores in step. The command traffic (ADD_EMAIL to
// the new worker before, REMOVE_EMAIL to the old one after a drain) is the
// email service's job; this is only the bookkeeping.
func (s *workerAssignmentService) MoveEmailToWorker(ctx context.Context, emailAccountID, oldWorkerID, newWorkerID uuid.UUID) error {
	if oldWorkerID == newWorkerID {
		return nil
	}
	if err := s.migrateEmailToWorker(ctx, emailAccountID, oldWorkerID, newWorkerID); err != nil {
		return err
	}
	weight := s.resolveMailboxWeight(ctx, emailAccountID)
	// Best-effort like every other load_score touch: the capacity view
	// refresh corrects drift.
	_ = s.workerRepo.AddLoadScore(ctx, oldWorkerID, -weight)
	_ = s.workerRepo.AddLoadScore(ctx, newWorkerID, weight)
	return nil
}

// HasInFlightSend reports whether a send tick for the mailbox is running now.
func (s *workerAssignmentService) HasInFlightSend(ctx context.Context, emailAccountID uuid.UUID) (bool, error) {
	return s.workerRepo.HasActiveSendTask(ctx, emailAccountID)
}
