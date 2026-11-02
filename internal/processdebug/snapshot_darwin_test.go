package processdebug

import "testing"

func TestCStringStopsAtFirstNUL(t *testing.T) {
	if got := cString([]byte{'n', 'o', 'd', 'e', 0, 0, 's', 'k'}); got != "node" {
		t.Fatalf("executable = %q", got)
	}
	if got := cString([]byte("sleep")); got != "sleep" {
		t.Fatalf("un-terminated executable = %q", got)
	}
}
