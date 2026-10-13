package host

import (
	"strings"
	"testing"
)

func TestNonClaudePromisesModelFailsAdmission(t *testing.T) {
	t.Setenv("GIMBAL_PROMISES_MODEL", "gpt-5.6-luna")
	owner := New(t.Context(), t.TempDir())
	defer owner.Close()
	if _, err := owner.AdmitProject(t.TempDir()); err == nil || !strings.Contains(err.Error(), "GIMBAL_PROMISES_MODEL") {
		t.Fatalf("admission error %v", err)
	}
	if n := len(owner.Projects()); n != 0 {
		t.Fatalf("projects=%d after a failed admission", n)
	}
}
