package tasks

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/repository"
	"github.com/warmbly/warmbly/internal/sendrouting"
)

// Send-side routing: the mailbox stays pinned to one worker for sync, but
// each SEND_EMAIL goes to the next live worker of its tier so consecutive
// sends leave from different egress IPs. A worker that is not the sync owner
// is first handed the mailbox's credentials with a send-only ADD_EMAIL, once
// per worker per liveness window.

// SendTargetLister is the repository half: the ring of live workers a
// mailbox's sends may be routed to. Satisfied by repository.WorkerRepository.
type SendTargetLister interface {
	ListSendTargets(ctx context.Context, pinnedWorkerID uuid.UUID) (*repository.SendTargets, error)
}

// RoutingStore is the Redis half: the round-robin cursor, the loaded-on marks
// and the auth-failure marks. Satisfied by *cache.Cache.
type RoutingStore interface {
	Incr(ctx context.Context, key string) *redis.IntCmd
	MGet(ctx context.Context, keys ...string) *redis.SliceCmd
	SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.BoolCmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd
}

// SendOnlyLoader ships a mailbox's credentials to a worker that is not its
// sync owner. Satisfied by the email service's LoadAccountForSending.
type SendOnlyLoader interface {
	LoadAccountForSending(ctx context.Context, accountID, workerID uuid.UUID) error
}

type sendRouter struct {
	targets  SendTargetLister
	store    RoutingStore
	loader   SendOnlyLoader
	liveness WorkerLiveness
}

// WireSendRouting turns on per-send worker choice (SEND_ROUND_ROBIN).
// Without it every send goes to the mailbox's pinned worker.
func (s *emailSender) WireSendRouting(targets SendTargetLister, store RoutingStore, loader SendOnlyLoader) {
	s.router = &sendRouter{targets: targets, store: store, loader: loader, liveness: s.liveness}
}

// choose returns the worker this send should leave from, or false to send
// through the pinned worker: a dedicated owner, an empty ring, a store or
// repository error, or no candidate that is live, not auth-failed for the
// mailbox and loadable. Every failure here degrades to the pinned worker
// rather than failing the send.
func (r *sendRouter) choose(ctx context.Context, account models.Email, pinned uuid.UUID) (uuid.UUID, bool) {
	targets, err := r.targets.ListSendTargets(ctx, pinned)
	if err != nil {
		log.Warn().Err(err).Str("email_id", account.ID.String()).Msg("send routing: listing live workers failed; sending through the pinned worker")
		return uuid.Nil, false
	}
	if targets == nil || targets.Dedicated || len(targets.Live) == 0 {
		return uuid.Nil, false
	}

	skip := r.authFailed(ctx, account.ID, targets.Live)
	cursor, err := r.store.Incr(ctx, sendrouting.CursorKey(targets.FreeTier, string(targets.RiskPool))).Result()
	if err != nil {
		log.Warn().Err(err).Str("email_id", account.ID.String()).Msg("send routing: cursor unavailable; sending through the pinned worker")
		return uuid.Nil, false
	}

	chosen, ok := sendrouting.Pick(cursor, targets.Live, func(id uuid.UUID) bool {
		if skip[id] {
			return false
		}
		if id != pinned && r.liveness != nil {
			// The list is live by heartbeat row; the Redis heartbeat key
			// catches a worker that died since.
			if live, lerr := r.liveness.IsWorkerLive(ctx, id); lerr != nil || !live {
				return false
			}
		}
		if id == pinned {
			return true
		}
		return r.ensureLoaded(ctx, account.ID, id)
	})
	if !ok {
		return uuid.Nil, false
	}
	return chosen, true
}

// authFailed is the set of candidates that reported an authentication
// failure for the mailbox inside sendrouting.AuthFailureTTL. A store error
// skips nothing: a cache blip must not narrow the ring to the pinned worker.
func (r *sendRouter) authFailed(ctx context.Context, mailboxID uuid.UUID, live []uuid.UUID) map[uuid.UUID]bool {
	keys := make([]string, len(live))
	for i, id := range live {
		keys[i] = sendrouting.AuthFailedKey(mailboxID.String(), id.String())
	}
	vals, err := r.store.MGet(ctx, keys...).Result()
	if err != nil || len(vals) != len(live) {
		return nil
	}
	skip := map[uuid.UUID]bool{}
	for i, v := range vals {
		if v != nil {
			skip[live[i]] = true
		}
	}
	return skip
}

// ensureLoaded makes sure workerID holds the mailbox's credentials before a
// send is routed there. The loaded-on mark is claimed first (SETNX with the
// liveness TTL) so the credentials are published once per worker per window;
// a publish that fails gives the mark back and the candidate is passed over.
func (r *sendRouter) ensureLoaded(ctx context.Context, mailboxID, workerID uuid.UUID) bool {
	if r.loader == nil {
		return false
	}
	key := sendrouting.LoadedOnKey(mailboxID, workerID)
	claimed, err := r.store.SetNX(ctx, key, "1", sendrouting.LoadedOnTTL).Result()
	if err != nil {
		log.Warn().Err(err).Str("email_id", mailboxID.String()).Str("worker_id", workerID.String()).Msg("send routing: loaded-on mark unavailable")
		return false
	}
	if !claimed {
		return true
	}
	if err := r.loader.LoadAccountForSending(ctx, mailboxID, workerID); err != nil {
		log.Warn().Err(err).Str("email_id", mailboxID.String()).Str("worker_id", workerID.String()).Msg("send routing: send-only load failed; passing over this worker")
		_ = r.store.Del(ctx, key).Err()
		return false
	}
	log.Info().Str("email_id", mailboxID.String()).Str("worker_id", workerID.String()).Msg("send routing: mailbox loaded on a worker for sending")
	return true
}
