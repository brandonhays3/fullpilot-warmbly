package sendrouting

import (
	"testing"

	"github.com/google/uuid"
)

func TestPickWalksTheRingFromTheCursor(t *testing.T) {
	live := []uuid.UUID{uuid.MustParse("00000000-0000-0000-0000-000000000001"), uuid.MustParse("00000000-0000-0000-0000-000000000002"), uuid.MustParse("00000000-0000-0000-0000-000000000003")}
	for _, tc := range []struct {
		name   string
		cursor int64
		skip   uuid.UUID
		want   uuid.UUID
		ok     bool
	}{
		{"first INCR lands on the first worker", 1, uuid.Nil, live[0], true},
		{"second on the second", 2, uuid.Nil, live[1], true},
		{"wraps after the last", 4, uuid.Nil, live[0], true},
		{"a skipped worker yields the next one", 2, live[1], live[2], true},
		{"a skipped last worker wraps to the first", 3, live[2], live[0], true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := Pick(tc.cursor, live, func(id uuid.UUID) bool { return id != tc.skip })
			if ok != tc.ok || got != tc.want {
				t.Fatalf("Pick = %s, %v; want %s, %v", got, ok, tc.want, tc.ok)
			}
		})
	}
	if _, ok := Pick(7, nil, nil); ok {
		t.Fatal("an empty ring must not pick")
	}
	if _, ok := Pick(1, live, func(uuid.UUID) bool { return false }); ok {
		t.Fatal("a ring with nothing acceptable must not pick")
	}
}

func TestIsAuthCode(t *testing.T) {
	for code, want := range map[string]bool{
		"AUTHENTICATION_FAILED":        true,
		"INVALID_CREDENTIALS":          true,
		"GOOGLE_AUTHENTICATION_FAILED": true,
		"RECIPIENT_REJECTED":           false,
		"":                             false,
	} {
		if got := IsAuthCode(code); got != want {
			t.Errorf("IsAuthCode(%q) = %v, want %v", code, got, want)
		}
	}
}
