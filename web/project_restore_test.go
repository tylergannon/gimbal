package web

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestInstanceRestoresAccessibleProjectsDespiteMissingSavedPaths(t *testing.T) {
	base := t.TempDir()
	instanceDir := filepath.Join(base, "instance")
	first, second := filepath.Join(base, "first"), filepath.Join(base, "second")
	for _, path := range []string{instanceDir, first, second} {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	paths := []string{filepath.Join(base, "removed"), first, filepath.Join(base, "moved"), second}
	raw, _ := json.Marshal(paths)
	if err := os.WriteFile(filepath.Join(instanceDir, "projects.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	instance, err := NewInstance(ctx, instanceDir, nil, WithNoWeb())
	if err != nil {
		t.Fatalf("missing project stopped central: %v", err)
	}
	defer func() { cancel(); <-instance.done }()
	for _, path := range []string{first, second} {
		if project, err := instance.Owner.Project(path); err != nil || project == nil {
			t.Fatalf("accessible project %s unavailable: %v", path, err)
		}
	}
	if projects := instance.Owner.Projects(); len(projects) != 2 {
		t.Fatalf("restored %d projects", len(projects))
	}
}
