package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIndexRejectsOverlapThroughSymlinkBeforeStarting(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "cache")
	if err := os.Mkdir(source, 0o755); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "alias")
	if err := os.Symlink(source, alias); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	err := runIndex([]string{"-from", source, "-to", filepath.Join(alias, "new", "index"), "-repo", root, "-no-web"}, &out, &out)
	if err == nil || !strings.Contains(err.Error(), "overlap") {
		t.Fatalf("expected overlap error, got %v", err)
	}
	entries, err := os.ReadDir(source)
	if err != nil || len(entries) != 0 {
		t.Fatalf("invalid invocation changed the cache: %v %v", entries, err)
	}
	if strings.Contains(out.String(), "run records") {
		t.Fatal("started a run for invalid paths")
	}
}

func TestIndexPreviewDoesNotWriteIndexOrPlanningConfig(t *testing.T) {
	root := t.TempDir()
	source, output := filepath.Join(root, "cache"), filepath.Join(root, "index")
	if err := os.Mkdir(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "one.md"), []byte("source content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := runIndex([]string{"-from", source, "-to", output, "-repo", root, "-dry-run"}, &out, &out); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{output, filepath.Join(root, ".gimble", "semantic-index.json")} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("preview created %s: %v", path, err)
		}
	}
	data, err := os.ReadFile(filepath.Join(source, "one.md"))
	if err != nil || string(data) != "source content\n" {
		t.Fatal("preview changed the source")
	}
}

func TestBuiltinSemanticPathsResolveFromRepository(t *testing.T) {
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "INDEX.md"), []byte("index"), 0o644); err != nil {
		t.Fatal(err)
	}
	in := builtinRequest{Repo: repo, Goal: "Plan a change", SemanticIndex: "INDEX.md", TokenCache: "."}
	if err := normalizeBuiltin(&in); err != nil {
		t.Fatal(err)
	}
	if in.SemanticIndex != filepath.Join(repo, "INDEX.md") || in.TokenCache != repo {
		t.Fatalf("wrong semantic input paths: %+v", in)
	}
}
