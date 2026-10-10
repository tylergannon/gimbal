package web

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/tylergannon/gimbal/internal/invocation"
)

func TestInvalidJSONStartDoesNotAdmitProject(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	base := t.TempDir()
	project := filepath.Join(base, "project")
	instanceDir := filepath.Join(base, "instance")
	if err := os.Mkdir(project, 0755); err != nil {
		t.Fatal(err)
	}
	instance, err := NewInstance(ctx, instanceDir, nil, WithNoWeb())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cancel(); instance.Wait() })
	for _, raw := range []string{`{}`, `{"goal":""}`, `{"goal":"ok","unexpected":1}`} {
		if _, err := Start(ctx, instanceDir, invocation.Request{Project: project, Workflow: "review", Input: json.RawMessage(raw)}); err == nil {
			t.Errorf("accepted %s", raw)
		}
	}
	if len(instance.Owner.Projects()) != 0 {
		t.Fatal("invalid input admitted project")
	}
}
