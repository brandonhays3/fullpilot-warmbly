package config

import (
	"os"
	"strconv"
)

const (
	// SmtpRotateAfterSecondsDefault is how long an SMTP/IMAP mailbox stays on
	// one ephemeral worker before the reconciler moves it to another live one,
	// so successive sends from a customer's own domain leave from different
	// egress IPs even while the worker it sits on is still alive.
	SmtpRotateAfterSecondsDefault = 600

	// SmtpMoveDrainSeconds is how long the old worker keeps a moved mailbox
	// loaded before it is told to drop it. A send dispatched to the old worker
	// just before the assignment switched is answered within this window, so
	// the REMOVE_EMAIL never lands under an in-flight send.
	SmtpMoveDrainSeconds = 60
)

// SmtpRotateAfterSeconds is SMTP_ROTATE_AFTER_SECONDS, or the default when
// unset or not a positive integer. Zero and negatives are not accepted: a
// rotation with no dwell would move every mailbox on every pass.
func SmtpRotateAfterSeconds() int {
	v, err := strconv.Atoi(os.Getenv("SMTP_ROTATE_AFTER_SECONDS"))
	if err != nil || v <= 0 {
		return SmtpRotateAfterSecondsDefault
	}
	return v
}
