package config

import (
	"os"
	"strconv"
	"strings"
)

// WorkerDeploymentUnknown is what a worker reports as its deployment when
// WORKER_DEPLOYMENT is unset. Known values are cloud_run_worker (a rotating
// Cloud Run Job) and cloud_vm (the persistent worker on the control-plane
// VM); the value is free-form so a new placement needs no code change.
const WorkerDeploymentUnknown = "unknown"

// SendTransportAPIPercent is the share (0..100) of sends from a Gmail or
// Outlook mailbox that go through the provider API; the rest go through the
// provider's SMTP submission endpoint. From SEND_TRANSPORT_API_PERCENT, with
// SendTransportAPIPercentDefault when unset or unparsable, clamped to range.
func SendTransportAPIPercent() int {
	raw := strings.TrimSpace(os.Getenv("SEND_TRANSPORT_API_PERCENT"))
	if raw == "" {
		return SendTransportAPIPercentDefault
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return SendTransportAPIPercentDefault
	}
	if n < 0 {
		return 0
	}
	if n > 100 {
		return 100
	}
	return n
}

// WorkerDeployment is the label a worker stamps on every send it makes, from
// WORKER_DEPLOYMENT, normalized to the [a-z0-9_] alphabet the send method
// string is built from.
func WorkerDeployment() string {
	raw := strings.ToLower(strings.TrimSpace(os.Getenv("WORKER_DEPLOYMENT")))
	if raw == "" {
		return WorkerDeploymentUnknown
	}
	var b strings.Builder
	for _, r := range raw {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
		case r == '-', r == ' ', r == '.':
			b.WriteRune('_')
		}
	}
	if b.Len() == 0 {
		return WorkerDeploymentUnknown
	}
	return b.String()
}
