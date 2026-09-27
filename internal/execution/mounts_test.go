package execution

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestWorktreeMountsIncludesExternalGitMetadataAndArtifacts(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "project")
	worktree := filepath.Join(root, "worktrees", "topic")
	gitCommon := filepath.Join(root, "main", ".git")
	gitDir := filepath.Join(gitCommon, "worktrees", "topic")
	artifacts := filepath.Join(project, ".gimbal")
	extra := filepath.Join(root, "operator-input")
	for _, dir := range []string{project, filepath.Join(worktree, "nested"), gitDir, gitCommon, artifacts, extra} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(worktree, ".git"), []byte("gitdir: "+gitDir+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "commondir"), []byte("../..\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mounts, err := WorktreeMounts(project, filepath.Join(worktree, "nested"), artifacts, extra, project)
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{project, filepath.Join(worktree, "nested"), worktree, artifacts, extra, gitDir, gitCommon} {
		if !slices.Contains(mounts, required) {
			t.Errorf("mounts %v do not include required path %q", mounts, required)
		}
	}
	if slices.Contains(mounts, root) {
		t.Fatalf("mounts include broad parent directory %q: %v", root, mounts)
	}
	if len(mounts) != 7 {
		t.Fatalf("configured and derived mounts = %v, want seven unique paths", mounts)
	}
}

func TestUniqueMountsDeduplicatesExplicitAndDerivedPaths(t *testing.T) {
	mounts := uniqueMounts([]string{"/tmp/project", "/tmp/extra", "/tmp/project", "/tmp/extra/.."})
	want := []string{"/tmp/project", "/tmp/extra", "/tmp"}
	if !slices.Equal(mounts, want) {
		t.Fatalf("mounts = %v, want %v", mounts, want)
	}
}
