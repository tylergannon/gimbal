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

func TestInstanceRestoresAccessibleProjectDespiteAnotherOwner(t *testing.T) {
	base := t.TempDir()
	busy, available := filepath.Join(base, "busy"), filepath.Join(base, "available")
	for _, path := range []string{busy, available} {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	firstCtx, cancelFirst := context.WithCancel(t.Context())
	first, err := NewInstance(firstCtx, filepath.Join(base, "first-instance"), []string{busy}, WithNoWeb())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { cancelFirst(); first.Wait() }()
	secondDir := filepath.Join(base, "second-instance")
	if err := os.Mkdir(secondDir, 0700); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal([]string{busy, available})
	if err := os.WriteFile(filepath.Join(secondDir, "projects.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	secondCtx, cancelSecond := context.WithCancel(t.Context())
	second, err := NewInstance(secondCtx, secondDir, nil, WithNoWeb())
	if err != nil {
		t.Fatalf("busy saved project blocked startup: %v", err)
	}
	defer func() { cancelSecond(); second.Wait() }()
	if project, err := second.Owner.Project(available); err != nil || project == nil {
		t.Fatalf("accessible project unavailable: %v", err)
	}
	if len(second.Owner.Projects()) != 1 {
		t.Fatal("busy project was admitted")
	}
}
