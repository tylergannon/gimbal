package resources

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProjectTrustStoreSetGet(t *testing.T) {
	agentDir := t.TempDir()
	store := NewProjectTrustStore(agentDir)
	project := t.TempDir()

	if decision := store.Get(project); decision != nil {
		t.Fatalf("initial decision = %v, want nil", *decision)
	}

	yes := true
	store.Set(project, &yes)
	if decision := store.Get(project); decision == nil || !*decision {
		t.Fatalf("decision = %v, want true", decision)
	}

	no := false
	store.Set(project, &no)
	if decision := store.Get(project); decision == nil || *decision {
		t.Fatalf("decision = %v, want false", decision)
	}

	store.Set(project, nil)
	if decision := store.Get(project); decision != nil {
		t.Fatalf("decision = %v, want nil after removal", *decision)
	}
}

func TestProjectTrustStoreAncestorLookup(t *testing.T) {
	agentDir := t.TempDir()
	store := NewProjectTrustStore(agentDir)
	parent := t.TempDir()
	child := filepath.Join(parent, "child")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}

	yes := true
	store.Set(parent, &yes)

	decision := store.Get(child)
	if decision == nil || !*decision {
		t.Fatalf("decision = %v, want true from parent", decision)
	}
	entry, ok := store.GetEntry(child)
	if !ok || entry.Path != canonicalizePath(parent) {
		t.Fatalf("entry = %+v, ok = %v", entry, ok)
	}
}

func TestHasTrustRequiringProjectResources(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if got := HasTrustRequiringProjectResources(home); got {
		t.Errorf("home with no resources = true, want false")
	}

	project := t.TempDir()
	skillsDir := filepath.Join(project, ".pi", "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if !HasTrustRequiringProjectResources(project) {
		t.Errorf("project with .pi/skills = false, want true")
	}
}

func TestGetProjectTrustOptions(t *testing.T) {
	project := t.TempDir()
	options := GetProjectTrustOptions(project, true)
	labels := map[string]bool{}
	for _, option := range options {
		labels[option.Label] = true
	}
	for _, want := range []string{"Trust", "Trust (this session only)", "Do not trust", "Do not trust (this session only)"} {
		if !labels[want] {
			t.Errorf("missing option %q in %v", want, labels)
		}
	}

	withoutSession := GetProjectTrustOptions(project, false)
	if len(withoutSession) >= len(options) {
		t.Errorf("session-only options not gated: %d vs %d", len(withoutSession), len(options))
	}
}

func TestResolveProjectTrustedOverride(t *testing.T) {
	override := true
	trusted, err := ResolveProjectTrusted(t.Context(), ResolveProjectTrustedOptions{
		Cwd:           t.TempDir(),
		TrustOverride: &override,
	})
	if err != nil || !trusted {
		t.Fatalf("trusted = %v, err = %v", trusted, err)
	}
}

func TestResolveProjectTrustedNoResources(t *testing.T) {
	trusted, err := ResolveProjectTrusted(t.Context(), ResolveProjectTrustedOptions{Cwd: t.TempDir()})
	if err != nil || !trusted {
		t.Fatalf("trusted = %v, err = %v", trusted, err)
	}
}
