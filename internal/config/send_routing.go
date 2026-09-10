package config

import (
	"os"
	"strings"
)

// SendRoundRobin is SEND_ROUND_ROBIN, read by the backend: whether each send
// is routed to the next live worker of the mailbox's tier (round robin across
// the fleet, so consecutive sends leave from different egress IPs) or always
// to the worker that syncs the mailbox. On unless set to an explicit no.
func SendRoundRobin() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("SEND_ROUND_ROBIN"))) {
	case "0", "false", "off", "no":
		return false
	}
	return true
}
