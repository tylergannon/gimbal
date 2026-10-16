package main

import (
	"strings"
	"testing"
)

func TestConventionalVersionSelection(t *testing.T) {
	for _, tt := range []struct {
		message, want string
	}{
		{"fix: repair cancellation", "v0.13.46"},
		{"perf(runtime): reduce polling", "v0.13.46"},
		{"feat(ui): show transcripts", "v0.14.0"},
		{"FEAT: show transcripts", "v0.14.0"},
		{"feat: show transcripts\x00\nfix: scrolling", "v0.14.0"},
		{"fix: scrolling\x00\nfeat: show transcripts", "v0.14.0"},
		{"docs: explain the UI\x00\ntest: check scrolling\x00\nchore: release v0.13.45\x00\nrefactor: simplify code\x00\nci: update jobs", ""},
		{"fix: explain syntax\n\nExample: feat!: an example, not a declaration", "v0.13.46"},
		{"", ""},
	} {
		t.Run(tt.message, func(t *testing.T) {
			got, err := nextVersion("v0.13.45", tt.message)
			if err != nil || got != tt.want {
				t.Fatalf("got %q, %v; want %q", got, err, tt.want)
			}
		})
	}
}

func TestRejectInvalidOrBreakingMessages(t *testing.T) {
	for _, message := range []string{"feat!: replace API", "FIX(api)!: replace API", "feat: replace API\n\nBREAKING CHANGE: removed API", "fix: repair API\n\nBREAKING-CHANGE: removed API", "Add new feature", "feat:", "feat: "} {
		t.Run(message, func(t *testing.T) {
			if tag, err := nextVersion("v0.13.45", message); err == nil || tag != "" {
				t.Fatalf("accepted %q: %q, %v", message, tag, err)
			}
		})
	}
	if _, err := nextVersion("v1.0.0", "fix: repair API"); err == nil || !strings.Contains(err.Error(), "forbidden") {
		t.Fatal("accepted major version baseline")
	}
}
