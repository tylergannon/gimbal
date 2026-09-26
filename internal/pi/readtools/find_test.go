package readtools

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
)

func findLines(t *testing.T, out string) []string {
	t.Helper()
	if out == "No files found matching pattern" {
		return nil
	}
	var files []string
	for line := range strings.SplitSeq(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "[") {
			continue
		}
		files = append(files, line)
	}
	sort.Strings(files)
	return files
}

func TestFindIncludesHiddenUnignoredFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".secret", "hidden.txt"), []byte("hidden"))
	writeFile(t, filepath.Join(dir, "visible.txt"), []byte("visible"))

	def := FindTool(dir, nil)
	res, err := runTool(t, def, map[string]any{"pattern": "**/*.txt", "path": dir})
	if err != nil {
		t.Fatal(err)
	}
	files := findLines(t, resultText(t, res))
	if !containsString(files, "visible.txt") || !containsString(files, ".secret/hidden.txt") {
		t.Fatalf("find results = %v", files)
	}
}

func TestFindRespectsGitignore(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".gitignore"), []byte("ignored.txt\n"))
	writeFile(t, filepath.Join(dir, "ignored.txt"), []byte("ignored"))
	writeFile(t, filepath.Join(dir, "kept.txt"), []byte("kept"))

	def := FindTool(dir, nil)
	res, err := runTool(t, def, map[string]any{"pattern": "**/*.txt", "path": dir})
	if err != nil {
		t.Fatal(err)
	}
	out := resultText(t, res)
	if !strings.Contains(out, "kept.txt") || strings.Contains(out, "ignored.txt") {
		t.Fatalf("find output = %q", out)
	}
}

func TestFindSurfacesGlobParseError(t *testing.T) {
	dir := t.TempDir()
	def := FindTool(dir, nil)
	_, err := runTool(t, def, map[string]any{"pattern": "[", "path": dir})
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "parsing glob") {
		t.Fatalf("expected glob parse error, got %v", err)
	}
}

func TestFindFlagLikePatternIsSearchText(t *testing.T) {
	dir := t.TempDir()
	def := FindTool(dir, nil)
	res, err := runTool(t, def, map[string]any{"pattern": "--help", "path": dir})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resultText(t, res), "No files found matching pattern") {
		t.Fatalf("find output = %q", resultText(t, res))
	}
}

func TestFindOperationsExistsAndGlobAreConsulted(t *testing.T) {
	var existsPath string
	ops := &FindOperations{
		Exists: func(_ context.Context, p string) (bool, error) {
			existsPath = p
			return true, nil
		},
		Glob: func(_ context.Context, pattern, cwd string, _ []string, _ int) ([]string, error) {
			return []string{filepath.Join(cwd, "a.go"), filepath.Join(cwd, "b.go")}, nil
		},
	}
	def := FindTool("/base", &FindToolOptions{Operations: ops})
	res, err := runTool(t, def, map[string]any{"pattern": "*.go"})
	if err != nil {
		t.Fatal(err)
	}
	if existsPath != "/base" {
		t.Fatalf("exists path = %q", existsPath)
	}
	files := findLines(t, resultText(t, res))
	if len(files) != 2 || files[0] != "a.go" || files[1] != "b.go" {
		t.Fatalf("find results = %v", files)
	}
}

// TestFindNestedGitignoreDoesNotLeak is the Go port of the #3303 regression:
// each .gitignore scopes to its own subtree and must not filter a sibling.
func TestFindNestedGitignoreDoesNotLeak(t *testing.T) {
	t.Run("flat siblings", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "a", ".gitignore"), []byte("ignored.txt\n"))
		writeFile(t, filepath.Join(dir, "a", "ignored.txt"), nil)
		writeFile(t, filepath.Join(dir, "a", "kept.txt"), nil)
		writeFile(t, filepath.Join(dir, "b", "ignored.txt"), nil)
		writeFile(t, filepath.Join(dir, "b", "kept.txt"), nil)
		writeFile(t, filepath.Join(dir, "root.txt"), nil)

		def := FindTool(dir, nil)
		res, err := runTool(t, def, map[string]any{"pattern": "**/*.txt"})
		if err != nil {
			t.Fatal(err)
		}
		got := findLines(t, resultText(t, res))
		want := []string{"a/kept.txt", "b/ignored.txt", "b/kept.txt", "root.txt"}
		if !equalStrings(got, want) {
			t.Fatalf("find = %v, want %v", got, want)
		}
	})

	t.Run("deeply nested", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "a", ".gitignore"), []byte("ignored.txt\n"))
		writeFile(t, filepath.Join(dir, "a", "deep", ".gitignore"), []byte("secret.txt\n"))
		writeFile(t, filepath.Join(dir, "a", "ignored.txt"), nil)
		writeFile(t, filepath.Join(dir, "a", "kept.txt"), nil)
		writeFile(t, filepath.Join(dir, "a", "deep", "ignored.txt"), nil)
		writeFile(t, filepath.Join(dir, "a", "deep", "secret.txt"), nil)
		writeFile(t, filepath.Join(dir, "a", "deep", "kept.txt"), nil)
		writeFile(t, filepath.Join(dir, "b", "ignored.txt"), nil)
		writeFile(t, filepath.Join(dir, "b", "kept.txt"), nil)
		writeFile(t, filepath.Join(dir, "root.txt"), nil)

		def := FindTool(dir, nil)
		res, err := runTool(t, def, map[string]any{"pattern": "**/*.txt"})
		if err != nil {
			t.Fatal(err)
		}
		got := findLines(t, resultText(t, res))
		want := []string{"a/deep/kept.txt", "a/kept.txt", "b/ignored.txt", "b/kept.txt", "root.txt"}
		if !equalStrings(got, want) {
			t.Fatalf("find = %v, want %v", got, want)
		}
	})
}

// TestFindRespectsNestedRepoBoundaries ports upstream 756a4e8f (#5960): inside
// a repo, fd's git-aware traversal stops a parent .gitignore at a nested repo
// boundary, so a checked-out sub-repo is governed by its own ignore rules.
func TestFindRespectsNestedRepoBoundaries(t *testing.T) {
	dir := t.TempDir()
	mustMkdir(t, filepath.Join(dir, ".git"))
	writeFile(t, filepath.Join(dir, ".gitignore"), []byte("ignored.txt\n"))
	writeFile(t, filepath.Join(dir, "ignored.txt"), nil)
	writeFile(t, filepath.Join(dir, "keep.txt"), nil)

	nested := filepath.Join(dir, "nested")
	mustMkdir(t, filepath.Join(nested, ".git"))
	writeFile(t, filepath.Join(nested, ".gitignore"), []byte("secret.txt\n"))
	writeFile(t, filepath.Join(nested, "ignored.txt"), nil)
	writeFile(t, filepath.Join(nested, "keep.txt"), nil)
	writeFile(t, filepath.Join(nested, "secret.txt"), nil)

	def := FindTool(dir, nil)
	res, err := runTool(t, def, map[string]any{"pattern": "**/*.txt"})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, line := range findLines(t, resultText(t, res)) {
		got[line] = true
	}
	if got["ignored.txt"] {
		t.Fatalf("outer .gitignore should hide top-level ignored.txt: %v", got)
	}
	if !got["nested/ignored.txt"] {
		t.Fatalf("outer .gitignore must not cross the nested-repo boundary: %v", got)
	}
	if got["nested/secret.txt"] {
		t.Fatalf("nested .gitignore should hide nested/secret.txt: %v", got)
	}
	if !got["keep.txt"] || !got["nested/keep.txt"] {
		t.Fatalf("visible files missing: %v", got)
	}
}

func containsString(haystack []string, needle string) bool {
	return slices.Contains(haystack, needle)
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}
