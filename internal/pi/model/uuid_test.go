package model

import (
	"regexp"
	"strings"
	"testing"
)

var uuidV7Pattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestUUIDv7Format(t *testing.T) {
	for range 8 {
		id := UUIDv7()
		if !uuidV7Pattern.MatchString(id) {
			t.Fatalf("UUIDv7 %q does not match the v7 pattern", id)
		}
	}
}

func TestUUIDv7AtPreservesTimestamp(t *testing.T) {
	// The first 48 bits are the millisecond timestamp; 1000 ms is 0x0000000003e8.
	if got, want := strings.ReplaceAll(UUIDv7At(1000), "-", "")[:12], "0000000003e8"; got != want {
		t.Fatalf("timestamp prefix = %q, want %q", got, want)
	}
}

func TestThinkingLevelOptions(t *testing.T) {
	if DefaultThinkingLevel != ThinkingMedium {
		t.Fatalf("default = %q", DefaultThinkingLevel)
	}
	if len(ThinkingLevelOptions) != 7 || ThinkingLevelOptions[0] != ThinkingOff {
		t.Fatalf("options = %v", ThinkingLevelOptions)
	}
}
