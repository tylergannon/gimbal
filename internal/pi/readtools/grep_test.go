package readtools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGrepSingleFileIncludesFilename(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "example.txt"), []byte("first line\nmatch line\nlast line"))

	def := GrepTool(dir, nil)
	res, err := runTool(t, def, map[string]any{"pattern": "match", "path": filepath.Join(dir, "example.txt")})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resultText(t, res), "example.txt:2: match line") {
		t.Fatalf("grep output = %q", resultText(t, res))
	}
}

func TestGrepRespectsLimitAndContext(t *testing.T) {
	dir := t.TempDir()
	content := strings.Join([]string{"before", "match one", "after", "middle", "match two", "after two"}, "\n")
	writeFile(t, filepath.Join(dir, "context.txt"), []byte(content))

	def := GrepTool(dir, nil)
	res, err := runTool(t, def, map[string]any{
		"pattern": "match",
		"path":    filepath.Join(dir, "context.txt"),
		"limit":   float64(1),
		"context": float64(1),
	})
	if err != nil {
		t.Fatal(err)
	}
	out := resultText(t, res)
	for _, want := range []string{
		"context.txt-1- before",
		"context.txt:2: match one",
		"context.txt-3- after",
		"[1 matches limit reached. Use limit=2 for more, or refine pattern]",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("grep output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "match two") {
		t.Fatalf("second match leaked through the limit:\n%s", out)
	}
}

func TestGrepFlagLikePatternIsSearchText(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "grep-injection-marker")
	payload := filepath.Join(dir, "payload.sh")
	writeFile(t, payload, []byte("#!/bin/sh\necho executed > "+marker+"\ncat \"$1\"\n"))
	if err := os.Chmod(payload, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "target.txt"), []byte("target\n"))

	def := GrepTool(dir, nil)
	res, err := runTool(t, def, map[string]any{"pattern": "--pre=" + payload, "path": dir})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resultText(t, res), "No matches found") {
		t.Fatalf("grep output = %q", resultText(t, res))
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("grep must not execute the pattern")
	}
}

func TestGrepOperationsAreConsulted(t *testing.T) {
	var readPath string
	ops := &GrepOperations{
		IsDirectory: func(_ context.Context, p string) (bool, error) {
			return false, nil
		},
		ReadFile: func(_ context.Context, p string) ([]byte, error) {
			readPath = p
			return []byte("alpha\nbeta\n"), nil
		},
	}
	def := GrepTool("/base", &GrepToolOptions{Operations: ops})
	res, err := runTool(t, def, map[string]any{"pattern": "alpha", "path": "f.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if readPath != filepath.Join("/base", "f.txt") {
		t.Fatalf("read path = %q", readPath)
	}
	if !strings.Contains(resultText(t, res), "f.txt:1: alpha") {
		t.Fatalf("grep output = %q", resultText(t, res))
	}
}

func TestGrepGitignoreRequiresRepo(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".gitignore"), []byte("ignored.txt\n"))
	writeFile(t, filepath.Join(dir, "ignored.txt"), []byte("secret match\n"))
	writeFile(t, filepath.Join(dir, "visible.txt"), []byte("match here\n"))

	def := GrepTool(dir, nil)
	res, err := runTool(t, def, map[string]any{"pattern": "match"})
	if err != nil {
		t.Fatal(err)
	}
	out := resultText(t, res)
	// rg only honors .gitignore inside a git repository, and this directory is
	// not one.
	if !strings.Contains(out, "ignored.txt") {
		t.Fatalf("outside a repo, grep should not apply .gitignore:\n%s", out)
	}
}

func TestGrepGitignoreInsideRepo(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, ".gitignore"), []byte("ignored.txt\n"))
	writeFile(t, filepath.Join(dir, "ignored.txt"), []byte("secret match\n"))
	writeFile(t, filepath.Join(dir, "visible.txt"), []byte("match here\n"))

	def := GrepTool(dir, nil)
	res, err := runTool(t, def, map[string]any{"pattern": "match"})
	if err != nil {
		t.Fatal(err)
	}
	out := resultText(t, res)
	if strings.Contains(out, "ignored.txt") {
		t.Fatalf("grep should respect .gitignore inside a repo:\n%s", out)
	}
	if !strings.Contains(out, "visible.txt") {
		t.Fatalf("grep missed the visible file:\n%s", out)
	}
}

func TestGrepTruncatesLongLines(t *testing.T) {
	dir := t.TempDir()
	long := "match " + strings.Repeat("x", 600)
	writeFile(t, filepath.Join(dir, "long.txt"), []byte(long+"\n"))

	def := GrepTool(dir, nil)
	res, err := runTool(t, def, map[string]any{"pattern": "match", "path": filepath.Join(dir, "long.txt")})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resultText(t, res), "... [truncated]") {
		t.Fatalf("long line was not truncated: %q", resultText(t, res)[:min(80, len(resultText(t, res)))])
	}
}
