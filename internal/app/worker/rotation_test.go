// Contract tests for SMTP placement on ephemeral workers and for the rotation
// plan. Same stub style as assignment_test.go: only the repository methods the
// code under test touches have bodies.

package worker

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/repository"
)

var t0 = time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)

// ephemeralRow is a live cloud_run_worker capacity row of the premium tier
// that started at t0+age.
func ephemeralRow(id uuid.UUID, age time.Duration, base, load float64) repository.WorkerCapacityRowDB {
	started := t0.Add(age)
	row := capacityRow(id, base, load)
	row.FreeTier = false
	row.Deployment = models.WorkerDeploymentCloudRun
	row.StartedAt = &started
	return row
}

// vmRow is the persistent cloud_vm shared worker of the premium tier.
func vmRow(id uuid.UUID, base, load float64) repository.WorkerCapacityRowDB {
	started := t0.Add(-24 * time.Hour)
	row := capacityRow(id, base, load)
	row.FreeTier = false
	row.Deployment = models.WorkerDeploymentCloudVM
	row.StartedAt = &started
	return row
}

func smtpHint() *repository.EmailAccountPlacementHint {
	return &repository.EmailAccountPlacementHint{Provider: string(models.InboxProviderSMTPIMAP)}
}

func workersByID(rows []repository.WorkerCapacityRowDB) map[uuid.UUID]models.Worker {
	out := map[uuid.UUID]models.Worker{}
	for _, r := range rows {
		out[r.WorkerID] = models.Worker{ID: r.WorkerID, FreeTier: r.FreeTier, WorkerType: models.WorkerTypeShared, Active: true, Deployment: r.Deployment}
	}
	return out
}

func TestAssign_SmtpMailbox_EphemeralFirst(t *testing.T) {
	vm := uuid.New()
	newest := uuid.New()
	older := uuid.New()
	oldest := uuid.New()

	cases := []struct {
		name string
		rows []repository.WorkerCapacityRowDB
		hint *repository.EmailAccountPlacementHint
		want uuid.UUID
	}{
		{
			name: "only the VM is live: it takes the mailbox",
			rows: []repository.WorkerCapacityRowDB{vmRow(vm, 16, 3)},
			hint: smtpHint(),
			want: vm,
		},
		{
			name: "VM plus one ephemeral: the ephemeral wins even when the VM is emptier",
			rows: []repository.WorkerCapacityRowDB{vmRow(vm, 16, 0), ephemeralRow(newest, 0, 16, 5)},
			hint: smtpHint(),
			want: newest,
		},
		{
			name: "several ephemerals: the most recently started wins",
			rows: []repository.WorkerCapacityRowDB{
				vmRow(vm, 16, 0),
				ephemeralRow(oldest, -4*time.Minute, 16, 0),
				ephemeralRow(newest, -30*time.Second, 16, 0),
				ephemeralRow(older, -2*time.Minute, 16, 0),
			},
			hint: smtpHint(),
			want: newest,
		},
		{
			name: "newest is one mailbox ahead: the next one takes its turn",
			rows: []repository.WorkerCapacityRowDB{
				ephemeralRow(newest, -30*time.Second, 16, 1),
				ephemeralRow(older, -2*time.Minute, 16, 0),
			},
			hint: smtpHint(),
			want: older,
		},
		{
			name: "newest is full: the next live ephemeral takes it",
			rows: []repository.WorkerCapacityRowDB{
				vmRow(vm, 16, 0),
				ephemeralRow(newest, -30*time.Second, 16, 16),
				ephemeralRow(older, -2*time.Minute, 16, 2),
			},
			hint: smtpHint(),
			want: older,
		},
		{
			name: "every ephemeral is full: fall back to the VM",
			rows: []repository.WorkerCapacityRowDB{
				vmRow(vm, 16, 0),
				ephemeralRow(newest, -30*time.Second, 16, 16),
				ephemeralRow(older, -2*time.Minute, 16, 16),
			},
			hint: smtpHint(),
			want: vm,
		},
		{
			name: "gmail mailbox: least utilised wins, ephemeral or not",
			rows: []repository.WorkerCapacityRowDB{vmRow(vm, 16, 0), ephemeralRow(newest, 0, 16, 5)},
			hint: &repository.EmailAccountPlacementHint{Provider: string(models.InboxProviderGoogle)},
			want: vm,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wr := &stubWorkerRepo{
				capacityPremium: tc.rows,
				workersByID:     workersByID(tc.rows),
				placementHint:   tc.hint,
			}
			svc := NewAssignmentService(wr, &stubSubRepo{sub: paidSub()}, &stubPlanRepo{plan: &models.Plan{}})
			got, err := svc.AssignWorkerToEmail(context.Background(), uuid.New(), uuid.New())
			if err != nil {
				t.Fatalf("AssignWorkerToEmail: %v", err)
			}
			if got == nil || *got != tc.want {
				t.Fatalf("placed on %v, want %s", got, tc.want)
			}
		})
	}
}

func TestPickEphemeralWorker_SpreadsRoundRobinFromNewest(t *testing.T) {
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	cands := []ephemeralCandidate{
		{WorkerID: a, StartedAt: t0.Add(-3 * time.Minute), Load: 0, Headroom: 10},
		{WorkerID: b, StartedAt: t0.Add(-1 * time.Minute), Load: 0, Headroom: 10},
		{WorkerID: c, StartedAt: t0.Add(-2 * time.Minute), Load: 0, Headroom: 10},
	}
	// Six placements in a row, bumping load as the real path does.
	want := []uuid.UUID{b, c, a, b, c, a}
	for i, w := range want {
		got, ok := pickEphemeralWorker(cands, 1, uuid.Nil)
		if !ok || got != w {
			t.Fatalf("placement %d: got %s ok=%v, want %s", i, got, ok, w)
		}
		for j := range cands {
			if cands[j].WorkerID == got {
				cands[j].Load++
				cands[j].Headroom--
			}
		}
	}
}

func TestPickEphemeralWorker_ExcludesCurrentAndFull(t *testing.T) {
	cur, other := uuid.New(), uuid.New()
	cands := []ephemeralCandidate{
		{WorkerID: cur, StartedAt: t0, Load: 0, Headroom: 10},
		{WorkerID: other, StartedAt: t0.Add(-time.Minute), Load: 9, Headroom: 0.5},
	}
	if _, ok := pickEphemeralWorker(cands, 1, cur); ok {
		t.Fatal("the only other worker has no headroom; nothing should be picked")
	}
	cands[1].Headroom = 1
	got, ok := pickEphemeralWorker(cands, 1, cur)
	if !ok || got != other {
		t.Fatalf("got %s ok=%v, want %s", got, ok, other)
	}
}

func TestPlanSmtpRotation(t *testing.T) {
	vm := uuid.New()
	cur := uuid.New()
	fresh := uuid.New()
	rotateAfter := 10 * time.Minute
	assignedLongAgo := t0.Add(-11 * time.Minute)
	assignedJustNow := t0.Add(-2 * time.Minute)

	live := func(id uuid.UUID, dep models.WorkerDeployment, at *time.Time) repository.SmtpRotationCandidate {
		return repository.SmtpRotationCandidate{
			AccountID: uuid.New(), OrganizationID: uuid.New(), UserID: uuid.New(),
			WorkerID: &id, WorkerAssignedAt: at, WorkerType: models.WorkerTypeShared,
			WorkerDeployment: dep, WorkerLive: true,
		}
	}

	cases := []struct {
		name string
		cand repository.SmtpRotationCandidate
		rows []repository.WorkerCapacityRowDB
		want *uuid.UUID
	}{
		{
			name: "on the VM with an ephemeral live: move now",
			cand: live(vm, models.WorkerDeploymentCloudVM, &assignedJustNow),
			rows: []repository.WorkerCapacityRowDB{vmRow(vm, 16, 1), ephemeralRow(fresh, 0, 16, 0)},
			want: &fresh,
		},
		{
			name: "on the VM with no ephemeral live: stay",
			cand: live(vm, models.WorkerDeploymentCloudVM, &assignedLongAgo),
			rows: []repository.WorkerCapacityRowDB{vmRow(vm, 16, 1)},
			want: nil,
		},
		{
			name: "on an ephemeral inside the dwell: stay even though a newer one is live",
			cand: live(cur, models.WorkerDeploymentCloudRun, &assignedJustNow),
			rows: []repository.WorkerCapacityRowDB{ephemeralRow(cur, -3*time.Minute, 16, 1), ephemeralRow(fresh, 0, 16, 0)},
			want: nil,
		},
		{
			name: "on an ephemeral past the dwell: move to another live ephemeral",
			cand: live(cur, models.WorkerDeploymentCloudRun, &assignedLongAgo),
			rows: []repository.WorkerCapacityRowDB{ephemeralRow(cur, -3*time.Minute, 16, 1), ephemeralRow(fresh, 0, 16, 0)},
			want: &fresh,
		},
		{
			name: "past the dwell but alone: stay rather than hop to the VM",
			cand: live(cur, models.WorkerDeploymentCloudRun, &assignedLongAgo),
			rows: []repository.WorkerCapacityRowDB{vmRow(vm, 16, 0), ephemeralRow(cur, -3*time.Minute, 16, 1)},
			want: nil,
		},
		{
			name: "unknown assignment time counts as expired",
			cand: live(cur, models.WorkerDeploymentCloudRun, nil),
			rows: []repository.WorkerCapacityRowDB{ephemeralRow(cur, -3*time.Minute, 16, 1), ephemeralRow(fresh, 0, 16, 0)},
			want: &fresh,
		},
		{
			name: "worker gone: not planned here, the load path re-places it",
			cand: func() repository.SmtpRotationCandidate {
				c := live(cur, models.WorkerDeploymentCloudRun, &assignedLongAgo)
				c.WorkerLive = false
				return c
			}(),
			rows: []repository.WorkerCapacityRowDB{ephemeralRow(fresh, 0, 16, 0)},
			want: nil,
		},
		{
			name: "dedicated worker: never rotated",
			cand: func() repository.SmtpRotationCandidate {
				c := live(vm, models.WorkerDeploymentCloudVM, &assignedLongAgo)
				c.WorkerType = models.WorkerTypeDedicated
				return c
			}(),
			rows: []repository.WorkerCapacityRowDB{ephemeralRow(fresh, 0, 16, 0)},
			want: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wr := &stubWorkerRepo{
				capacityPremium: tc.rows,
				workersByID:     workersByID(tc.rows),
				placementHint:   smtpHint(),
			}
			svc := NewAssignmentService(wr, &stubSubRepo{}, &stubPlanRepo{})
			got, err := svc.PlanSmtpRotation(context.Background(), tc.cand, t0, rotateAfter)
			if err != nil {
				t.Fatalf("PlanSmtpRotation: %v", err)
			}
			switch {
			case tc.want == nil && got != nil:
				t.Fatalf("planned a move to %s, want none", got.ID)
			case tc.want != nil && got == nil:
				t.Fatalf("planned no move, want %s", *tc.want)
			case tc.want != nil && got.ID != *tc.want:
				t.Fatalf("planned a move to %s, want %s", got.ID, *tc.want)
			}
		})
	}
}
