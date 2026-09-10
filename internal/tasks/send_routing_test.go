package tasks

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/warmbly/warmbly/internal/events"
	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/repository"
	"github.com/warmbly/warmbly/internal/sendrouting"
)

type fakeTargets struct {
	targets *repository.SendTargets
	err     error
}

func (f fakeTargets) ListSendTargets(context.Context, uuid.UUID) (*repository.SendTargets, error) {
	return f.targets, f.err
}

// fakeStore is the Redis slice routing touches: a counter per cursor key and
// plain string keys with no expiry.
type fakeStore struct {
	counters map[string]int64
	keys     map[string]string
	setnx    int
}

func newFakeStore() *fakeStore {
	return &fakeStore{counters: map[string]int64{}, keys: map[string]string{}}
}

func (s *fakeStore) Incr(_ context.Context, key string) *redis.IntCmd {
	s.counters[key]++
	return redis.NewIntResult(s.counters[key], nil)
}

func (s *fakeStore) MGet(_ context.Context, keys ...string) *redis.SliceCmd {
	out := make([]interface{}, len(keys))
	for i, k := range keys {
		if v, ok := s.keys[k]; ok {
			out[i] = v
		}
	}
	return redis.NewSliceResult(out, nil)
}

func (s *fakeStore) SetNX(_ context.Context, key string, value interface{}, _ time.Duration) *redis.BoolCmd {
	s.setnx++
	if _, ok := s.keys[key]; ok {
		return redis.NewBoolResult(false, nil)
	}
	s.keys[key] = "1"
	return redis.NewBoolResult(true, nil)
}

func (s *fakeStore) Del(_ context.Context, keys ...string) *redis.IntCmd {
	var n int64
	for _, k := range keys {
		if _, ok := s.keys[k]; ok {
			delete(s.keys, k)
			n++
		}
	}
	return redis.NewIntResult(n, nil)
}

type fakeLoader struct {
	loads []uuid.UUID
	fail  bool
}

func (l *fakeLoader) LoadAccountForSending(_ context.Context, _ uuid.UUID, workerID uuid.UUID) error {
	if l.fail {
		return errors.New("bus down")
	}
	l.loads = append(l.loads, workerID)
	return nil
}

type fakeLiveness struct{ dead map[uuid.UUID]bool }

func (f fakeLiveness) IsWorkerLive(_ context.Context, id uuid.UUID) (bool, error) {
	return !f.dead[id], nil
}

type capturePublisher struct {
	events.Publisher

	sentTo []uuid.UUID
}

func (p *capturePublisher) PublishSendEmail(_ context.Context, workerID uuid.UUID, _ *events.SendEmailParams) error {
	p.sentTo = append(p.sentTo, workerID)
	return nil
}

func ring(n int) []uuid.UUID {
	out := make([]uuid.UUID, n)
	for i := range out {
		out[i] = uuid.MustParse("00000000-0000-0000-0000-00000000000" + string(rune('1'+i)))
	}
	return out
}

func TestSendRouterChoose(t *testing.T) {
	live := ring(3)
	pinned := live[0]
	mailbox := uuid.New()
	account := models.Email{ID: mailbox, WorkerID: &pinned}

	for _, tc := range []struct {
		name     string
		targets  *repository.SendTargets
		err      error
		authFail []uuid.UUID
		dead     []uuid.UUID
		loadFail bool
		sends    int
		want     []uuid.UUID
		wantOK   []bool
		loads    []uuid.UUID
	}{
		{
			name:    "consecutive sends walk every live worker and wrap",
			targets: &repository.SendTargets{Live: live},
			sends:   4,
			want:    []uuid.UUID{live[0], live[1], live[2], live[0]},
			wantOK:  []bool{true, true, true, true},
			loads:   []uuid.UUID{live[1], live[2]},
		},
		{
			name:     "a worker that reported an auth failure for the mailbox is skipped",
			targets:  &repository.SendTargets{Live: live},
			authFail: []uuid.UUID{live[1]},
			sends:    3,
			want:     []uuid.UUID{live[0], live[2], live[2]},
			wantOK:   []bool{true, true, true},
			loads:    []uuid.UUID{live[2]},
		},
		{
			name:    "a dedicated worker keeps its organization's sends",
			targets: &repository.SendTargets{Dedicated: true, Live: live},
			sends:   2,
			want:    []uuid.UUID{uuid.Nil, uuid.Nil},
			wantOK:  []bool{false, false},
		},
		{
			name:    "an empty live list falls back to the pinned worker",
			targets: &repository.SendTargets{},
			sends:   1,
			want:    []uuid.UUID{uuid.Nil},
			wantOK:  []bool{false},
		},
		{
			name:   "a repository error falls back to the pinned worker",
			err:    errors.New("db down"),
			sends:  1,
			want:   []uuid.UUID{uuid.Nil},
			wantOK: []bool{false},
		},
		{
			name:    "a worker whose heartbeat key is gone is passed over",
			targets: &repository.SendTargets{Live: live},
			dead:    []uuid.UUID{live[1]},
			sends:   2,
			want:    []uuid.UUID{live[0], live[2]},
			wantOK:  []bool{true, true},
			loads:   []uuid.UUID{live[2]},
		},
		{
			name:     "a send-only load that cannot be published passes over that worker",
			targets:  &repository.SendTargets{Live: live},
			loadFail: true,
			sends:    2,
			want:     []uuid.UUID{live[0], live[0]},
			wantOK:   []bool{true, true},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := newFakeStore()
			for _, id := range tc.authFail {
				store.keys[sendrouting.AuthFailedKey(mailbox.String(), id.String())] = "1"
			}
			dead := map[uuid.UUID]bool{}
			for _, id := range tc.dead {
				dead[id] = true
			}
			loader := &fakeLoader{fail: tc.loadFail}
			r := &sendRouter{
				targets:  fakeTargets{targets: tc.targets, err: tc.err},
				store:    store,
				loader:   loader,
				liveness: fakeLiveness{dead: dead},
			}
			for i := 0; i < tc.sends; i++ {
				got, ok := r.choose(context.Background(), account, pinned)
				if ok != tc.wantOK[i] || got != tc.want[i] {
					t.Fatalf("send %d: choose = %s, %v; want %s, %v", i+1, got, ok, tc.want[i], tc.wantOK[i])
				}
			}
			if len(loader.loads) != len(tc.loads) {
				t.Fatalf("send-only loads = %v, want %v", loader.loads, tc.loads)
			}
			for i := range tc.loads {
				if loader.loads[i] != tc.loads[i] {
					t.Fatalf("send-only loads = %v, want %v", loader.loads, tc.loads)
				}
			}
			if tc.loadFail {
				for k := range store.keys {
					t.Fatalf("a failed load must give its loaded-on mark back, still have %s", k)
				}
			}
		})
	}
}

// The credentials go out once per worker: the second send routed to a worker
// finds the loaded-on mark and publishes nothing.
func TestSendRouterLoadsAWorkerOnce(t *testing.T) {
	live := ring(2)
	pinned := live[0]
	mailbox := uuid.New()
	store := newFakeStore()
	loader := &fakeLoader{}
	r := &sendRouter{targets: fakeTargets{targets: &repository.SendTargets{Live: live}}, store: store, loader: loader}

	for i := 0; i < 4; i++ {
		r.choose(context.Background(), models.Email{ID: mailbox}, pinned)
	}
	if len(loader.loads) != 1 || loader.loads[0] != live[1] {
		t.Fatalf("loads = %v, want exactly one on %s", loader.loads, live[1])
	}
	if _, ok := store.keys[sendrouting.LoadedOnKey(mailbox, live[1])]; !ok {
		t.Fatal("the loaded-on mark for the send-only worker is missing")
	}
	if _, ok := store.keys[sendrouting.LoadedOnKey(mailbox, pinned)]; ok {
		t.Fatal("the pinned worker must not get a send-only load")
	}
}

// Send publishes to the routed worker, and to the pinned one when routing
// is not wired.
func TestSendPublishesToTheRoutedWorker(t *testing.T) {
	live := ring(2)
	pinned := live[0]
	org := uuid.New()
	account := models.Email{ID: uuid.New(), OrganizationID: &org, WorkerID: &pinned}

	pub := &capturePublisher{}
	s := &emailSender{publisher: pub}
	if err := s.Send(context.Background(), uuid.New(), EmailMessage{}, account); err != nil {
		t.Fatalf("Send: %v", err)
	}
	s.WireSendRouting(fakeTargets{targets: &repository.SendTargets{Live: live}}, newFakeStore(), &fakeLoader{})
	for i := 0; i < 2; i++ {
		if err := s.Send(context.Background(), uuid.New(), EmailMessage{}, account); err != nil {
			t.Fatalf("Send: %v", err)
		}
	}
	want := []uuid.UUID{pinned, live[0], live[1]}
	if len(pub.sentTo) != len(want) {
		t.Fatalf("published to %v, want %v", pub.sentTo, want)
	}
	for i := range want {
		if pub.sentTo[i] != want[i] {
			t.Fatalf("published to %v, want %v", pub.sentTo, want)
		}
	}
}
