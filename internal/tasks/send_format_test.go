package tasks

import (
	"testing"

	"github.com/google/uuid"
)

func TestSendFormatSplitIsStableAndRoughlyEven(t *testing.T) {
	t.Setenv("SEND_FORMAT_HTML_PERCENT", "50")
	html := 0
	for i := 0; i < 2000; i++ {
		id := uuid.New()
		a, b := sendFormatKeepsHTML(id), sendFormatKeepsHTML(id)
		if a != b {
			t.Fatalf("decision changed for the same task id")
		}
		if a {
			html++
		}
	}
	if html < 850 || html > 1150 {
		t.Fatalf("expected roughly half html, got %d of 2000", html)
	}
	t.Setenv("SEND_FORMAT_HTML_PERCENT", "0")
	if sendFormatKeepsHTML(uuid.New()) {
		t.Fatalf("0 percent must never keep html")
	}
	t.Setenv("SEND_FORMAT_HTML_PERCENT", "100")
	if !sendFormatKeepsHTML(uuid.New()) {
		t.Fatalf("100 percent must always keep html")
	}
}
