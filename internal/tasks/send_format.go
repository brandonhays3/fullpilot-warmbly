package tasks

import (
	"hash/fnv"

	"github.com/google/uuid"

	"github.com/warmbly/warmbly/internal/config"
)

// Send formats recorded on campaign_tasks.send_format.
const (
	SendFormatText = "text" // text/plain only
	SendFormatHTML = "html" // multipart with the HTML part
)

// sendFormatKeepsHTML decides, for a plain-text campaign, whether this send
// keeps its HTML part. Hashed from the task id with its own salt so it is
// independent of the transport split and stable across retries.
func sendFormatKeepsHTML(taskID uuid.UUID) bool {
	h := fnv.New32a()
	_, _ = h.Write([]byte("format:" + taskID.String()))
	return int(h.Sum32()%100) < config.SendFormatHTMLPercent()
}
