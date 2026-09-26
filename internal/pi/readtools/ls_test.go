package readtools

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestLsListsDotfilesAndDirectories(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".hidden-file"), []byte("secret"))
	mustMkdir(t, filepath.Join(dir, ".hidden-dir"))

	def := LsTool(dir, nil)
	res, err := runTool(t, def, map[string]any{"path": dir})
	if err != nil {
		t.Fatal(err)
	}
	out := resultText(t, res)
	if !strings.Contains(out, ".hidden-file") || !strings.Contains(out, ".hidden-dir/") {
		t.Fatalf("ls output = %q", out)
	}
}

func TestLsMissingPath(t *testing.T) {
	dir := t.TempDir()
	def := LsTool(dir, nil)
	_, err := runTool(t, def, map[string]any{"path": filepath.Join(dir, "nope")})
	if err == nil || !strings.Contains(err.Error(), "path not found") {
		t.Fatalf("expected path not found, got %v", err)
	}
}

func TestLsNotADirectory(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "f.txt"), []byte("x"))
	def := LsTool(dir, nil)
	_, err := runTool(t, def, map[string]any{"path": filepath.Join(dir, "f.txt")})
	if err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("expected not a directory, got %v", err)
	}
}

func TestLsEmptyDirectory(t *testing.T) {
	dir := t.TempDir()
	def := LsTool(dir, nil)
	res, err := runTool(t, def, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if resultText(t, res) != "(empty directory)" {
		t.Fatalf("ls output = %q", resultText(t, res))
	}
}

func TestLsOperationsAreConsulted(t *testing.T) {
	ops := &LsOperations{
		Exists: func(_ context.Context, _ string) (bool, error) { return true, nil },
		Stat: func(_ context.Context, p string) (bool, error) {
			return p == "/base" || strings.HasSuffix(p, "sub"), nil
		},
		Readdir: func(_ context.Context, _ string) ([]string, error) {
			return []string{"a.txt", "sub"}, nil
		},
	}
	def := LsTool("/base", &LsToolOptions{Operations: ops})
	res, err := runTool(t, def, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	out := resultText(t, res)
	if !strings.Contains(out, "a.txt") || !strings.Contains(out, "sub/") {
		t.Fatalf("ls output = %q", out)
	}
}
