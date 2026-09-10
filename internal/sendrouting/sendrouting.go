// Package sendrouting is the state shared between the backend, which picks a
// worker per send, and the consumer, which learns from the worker's answer.
// Sync ownership stays pinned (email_accounts.worker_id); this only decides
// which live worker a given SEND_EMAIL leaves from.
package sendrouting

import (
	"time"

	"github.com/google/uuid"
	"github.com/warmbly/warmbly/internal/errx"
)

const (
	// LoadedOnTTL is how long the backend remembers that a worker was handed
	// a mailbox's credentials for sending. It matches the ephemeral worker
	// liveness window (repository.EphemeralWorkerLivenessWindow): a worker
	// that has not heartbeated for that long is gone with its memory, and a
	// new run on the same id would need the ADD_EMAIL again.
	LoadedOnTTL = 4 * time.Minute

	// AuthFailureTTL is how long a worker that reported an authentication
	// failure for a mailbox is skipped for that mailbox.
	AuthFailureTTL = 10 * time.Minute
)

// CursorKey is the round-robin cursor for one tier and risk pool. INCR on it
// walks every send across the fleet through the live list in order.
func CursorKey(freeTier bool, riskPool string) string {
	tier := "premium"
	if freeTier {
		tier = "free"
	}
	if riskPool == "" || riskPool == "clean" {
		return "send:rr:" + tier
	}
	return "send:rr:" + tier + ":" + riskPool
}

// LoadedOnKey marks that workerID was sent the mailbox's credentials for
// sending (a send-only ADD_EMAIL). Present means "do not publish it again".
func LoadedOnKey(mailboxID, workerID uuid.UUID) string {
	return "mailbox:" + mailboxID.String() + ":loaded_on:" + workerID.String()
}

// AuthFailedKey marks that workerID could not authenticate the mailbox. Ids
// are strings because the consumer reads them off the wire unparsed.
func AuthFailedKey(mailboxID, workerID string) string {
	return "mailbox:" + mailboxID + ":auth_failed_on:" + workerID
}

// IsAuthCode reports whether a send failure code is an authentication
// refusal, the same set the worker maps to EMAIL_AUTH_ERROR.
func IsAuthCode(code string) bool {
	switch errx.MailErrorCode(code) {
	case errx.MailErrorCodeGoogleAuth, errx.MailErrorCodeAuthenticationFailed, errx.MailErrorCodeInvalidCredentials:
		return true
	}
	return false
}

// Pick walks live from the cursor's position and returns the first worker
// that ok accepts. live is expected ordered by worker id so every backend
// walks the same ring. ok may be nil. The second result is false when
// nothing qualifies, in which case the caller sends through the pinned worker.
func Pick(cursor int64, live []uuid.UUID, ok func(uuid.UUID) bool) (uuid.UUID, bool) {
	n := int64(len(live))
	if n == 0 {
		return uuid.Nil, false
	}
	start := ((cursor-1)%n + n) % n
	for i := int64(0); i < n; i++ {
		id := live[(start+i)%n]
		if ok == nil || ok(id) {
			return id, true
		}
	}
	return uuid.Nil, false
}
