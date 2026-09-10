package config

import (
	"os"
	"strconv"
	"strings"
)

// SendFormatHTMLPercentDefault is the share of a plain-text campaign's sends
// that still carry the HTML part (multipart/alternative); the rest go out as
// text/plain only. Recorded per send so the two formats can be compared.
const SendFormatHTMLPercentDefault = 50

// SendFormatHTMLPercent reads SEND_FORMAT_HTML_PERCENT (0..100), falling back
// to SendFormatHTMLPercentDefault when unset or unparsable, clamped to range.
func SendFormatHTMLPercent() int {
	raw := strings.TrimSpace(os.Getenv("SEND_FORMAT_HTML_PERCENT"))
	if raw == "" {
		return SendFormatHTMLPercentDefault
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return SendFormatHTMLPercentDefault
	}
	if n < 0 {
		return 0
	}
	if n > 100 {
		return 100
	}
	return n
}
