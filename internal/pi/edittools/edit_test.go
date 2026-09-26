package edittools

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/tylergannon/gimbal/internal/pi/model"
)

func executeEdit(t *testing.T, tool model.AgentTool, path string, edits ...Edit) (model.AgentToolResult, error) {
	t.Helper()
	raw := make([]any, 0, len(edits))
	for _, e := range edits {
		raw = append(raw, map[string]any{"oldText": e.OldText, "newText": e.NewText})
	}
	return tool.Execute(context.Background(), "test-call", map[string]any{"path": path, "edits": raw}, nil)
}

func resultText(t *testing.T, res model.AgentToolResult) string {
	t.Helper()
	text, ok := res.Content.TextContentText()
	if !ok {
		t.Fatalf("tool result has no text content: %#v", res.Content)
	}
	return text
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}
}

func TestEditReplaceText(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "edit-test.txt")
	writeTestFile(t, path, "Hello, world!")

	res, err := executeEdit(t, EditTool(dir, nil), path, Edit{OldText: "world", NewText: "testing"})
	if err != nil {
		t.Fatalf("edit failed: %v", err)
	}
	if got := resultText(t, res); !strings.Contains(got, "Successfully replaced") {
		t.Fatalf("unexpected result text: %q", got)
	}
	details, ok := res.Details.(EditToolDetails)
	if !ok {
		t.Fatalf("details type = %T", res.Details)
	}
	if !strings.Contains(details.Diff, "testing") {
		t.Fatalf("diff missing replacement: %q", details.Diff)
	}
	for _, want := range []string{"--- ", "+++ ", "@@", "-Hello, world!", "+Hello, testing!"} {
		if !strings.Contains(details.Patch, want) {
			t.Fatalf("patch missing %q:\n%s", want, details.Patch)
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "Hello, testing!" {
		t.Fatalf("file content = %q", data)
	}
}

func TestEditNotFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "edit-test.txt")
	writeTestFile(t, path, "Hello, world!")

	_, err := executeEdit(t, EditTool(dir, nil), path, Edit{OldText: "nonexistent", NewText: "testing"})
	if err == nil || !strings.Contains(err.Error(), "Could not find the exact text") {
		t.Fatalf("expected not-found error, got %v", err)
	}
}

func TestEditMissingFileIncludesENOENT(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "missing.txt")

	_, err := executeEdit(t, EditTool(dir, nil), missing, Edit{OldText: "hello", NewText: "world"})
	want := "Could not edit file: " + missing + ". Error code: ENOENT."
	if err == nil || err.Error() != want {
		t.Fatalf("error = %v, want %q", err, want)
	}
}

func TestEditDuplicateText(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "edit-test.txt")
	writeTestFile(t, path, "foo foo foo")

	_, err := executeEdit(t, EditTool(dir, nil), path, Edit{OldText: "foo", NewText: "bar"})
	if err == nil || !strings.Contains(err.Error(), "Found 3 occurrences") {
		t.Fatalf("expected duplicate error, got %v", err)
	}
}

func TestEditMultipleDisjointRegions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "edit-multi.txt")
	writeTestFile(t, path, "alpha\nbeta\ngamma\ndelta\n")

	res, err := executeEdit(t, EditTool(dir, nil), path,
		Edit{OldText: "alpha\n", NewText: "ALPHA\n"},
		Edit{OldText: "gamma\n", NewText: "GAMMA\n"},
	)
	if err != nil {
		t.Fatalf("edit failed: %v", err)
	}
	if got := resultText(t, res); !strings.Contains(got, "Successfully replaced 2 block(s)") {
		t.Fatalf("result = %q", got)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "ALPHA\nbeta\nGAMMA\ndelta\n" {
		t.Fatalf("file = %q", data)
	}
	details := res.Details.(EditToolDetails)
	if !strings.Contains(details.Diff, "ALPHA") || !strings.Contains(details.Diff, "GAMMA") {
		t.Fatalf("diff missing edits:\n%s", details.Diff)
	}
}

func TestEditCollapsesLargeUnchangedGaps(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "edit-multi-large-gap.txt")
	var lines []string
	for i := 1; i <= 600; i++ {
		lines = append(lines, fmt.Sprintf("line %03d", i))
	}
	writeTestFile(t, path, strings.Join(lines, "\n")+"\n")

	res, err := executeEdit(t, EditTool(dir, nil), path,
		Edit{OldText: "line 100\n", NewText: "LINE 100\n"},
		Edit{OldText: "line 300\n", NewText: "LINE 300\n"},
		Edit{OldText: "line 500\n", NewText: "LINE 500\n"},
	)
	if err != nil {
		t.Fatalf("edit failed: %v", err)
	}
	diff := res.Details.(EditToolDetails).Diff
	for _, want := range []string{"LINE 100", "LINE 300", "LINE 500", "..."} {
		if !strings.Contains(diff, want) {
			t.Fatalf("diff missing %q:\n%s", want, diff)
		}
	}
	if strings.Contains(diff, "line 250") {
		t.Fatalf("diff should have collapsed the gap:\n%s", diff)
	}
	if got := len(strings.Split(diff, "\n")); got >= 50 {
		t.Fatalf("diff has %d lines, want < 50:\n%s", got, diff)
	}
}

func TestEditMatchesOriginalNotIncrementally(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "edit-multi-original.txt")
	writeTestFile(t, path, "foo\nbar\nbaz\n")

	_, err := executeEdit(t, EditTool(dir, nil), path,
		Edit{OldText: "foo\n", NewText: "foo bar\n"},
		Edit{OldText: "bar\n", NewText: "BAR\n"},
	)
	if err != nil {
		t.Fatalf("edit failed: %v", err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "foo bar\nBAR\nbaz\n" {
		t.Fatalf("file = %q", data)
	}
}

func TestEditEmptyEdits(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "edit-empty-edits.txt")
	writeTestFile(t, path, "hello\nworld\n")

	_, err := EditTool(dir, nil).Execute(context.Background(), "id", map[string]any{"path": path, "edits": []any{}}, nil)
	if err == nil || !strings.Contains(err.Error(), "edits must contain at least one replacement") {
		t.Fatalf("expected empty-edits error, got %v", err)
	}
}

func TestEditOverlappingRegions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "edit-overlap.txt")
	writeTestFile(t, path, "one\ntwo\nthree\n")

	_, err := executeEdit(t, EditTool(dir, nil), path,
		Edit{OldText: "one\ntwo\n", NewText: "ONE\nTWO\n"},
		Edit{OldText: "two\nthree\n", NewText: "TWO\nTHREE\n"},
	)
	if err == nil || !strings.Contains(err.Error(), "overlap") {
		t.Fatalf("expected overlap error, got %v", err)
	}
}

func TestEditDoesNotPartiallyApply(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "edit-no-partial.txt")
	const original = "alpha\nbeta\ngamma\n"
	writeTestFile(t, path, original)

	_, err := executeEdit(t, EditTool(dir, nil), path,
		Edit{OldText: "alpha\n", NewText: "ALPHA\n"},
		Edit{OldText: "missing\n", NewText: "MISSING\n"},
	)
	if err == nil || !strings.Contains(err.Error(), "Could not find") {
		t.Fatalf("expected not-found error, got %v", err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != original {
		t.Fatalf("file mutated after failed edit: %q", data)
	}
}

func TestEditReadOnlyIncludesEACCES(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses file permission checks")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "edit-readonly.txt")
	writeTestFile(t, path, "hello\n")
	if err := os.Chmod(path, 0o444); err != nil {
		t.Fatal(err)
	}

	_, err := executeEdit(t, EditTool(dir, nil), path, Edit{OldText: "hello", NewText: "world"})
	want := "Could not edit file: " + path + ". Error code: EACCES."
	if err == nil || err.Error() != want {
		t.Fatalf("error = %v, want %q", err, want)
	}
}

func TestEditUnknownAccessError(t *testing.T) {
	dir := t.TempDir()
	tool := EditTool(dir, &EditOptions{Operations: &EditOperations{
		Access: func(context.Context, string) error { return errors.New("disk offline") },
		ReadFile: func(context.Context, string) ([]byte, error) {
			return []byte("hello\n"), nil
		},
		WriteFile: func(context.Context, string, string) error { return nil },
	}})

	_, err := executeEdit(t, tool, "broken.txt", Edit{OldText: "hello", NewText: "world"})
	want := "Could not edit file: broken.txt. Error: disk offline."
	if err == nil || err.Error() != want {
		t.Fatalf("error = %v, want %q", err, want)
	}
}

func TestEditOperationsAndSchema(t *testing.T) {
	definition := EditDefinition("", nil)
	if _, ok := definition.Parameters.Properties["oldText"]; ok {
		t.Fatal("public schema must not carry legacy oldText")
	}
	if _, ok := definition.Parameters.Properties["newText"]; ok {
		t.Fatal("public schema must not carry legacy newText")
	}
	res, err := EditTool(t.TempDir(), nil).Execute(context.Background(), "id", map[string]any{}, nil)
	if err == nil {
		t.Fatalf("expected validation error, got result %#v", res)
	}
}

func TestEditMutationQueueSerializesSameFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "counter.txt")
	writeTestFile(t, path, "alpha beta gamma delta epsilon\n")
	tool := EditTool(dir, nil)

	replacements := map[string]string{
		"alpha": "ALPHA", "beta": "BETA", "gamma": "GAMMA", "delta": "DELTA", "epsilon": "EPSILON",
	}
	var wg sync.WaitGroup
	var errMu sync.Mutex
	var errs []error
	for old, replacement := range replacements {
		wg.Go(func() {
			_, err := executeEdit(t, tool, "counter.txt", Edit{OldText: old, NewText: replacement})
			if err != nil {
				errMu.Lock()
				errs = append(errs, err)
				errMu.Unlock()
			}
		})
	}
	wg.Wait()
	if len(errs) != 0 {
		t.Fatalf("concurrent edits errored: %v", errs)
	}
	data, _ := os.ReadFile(path)
	for _, want := range []string{"ALPHA", "BETA", "GAMMA", "DELTA", "EPSILON"} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("lost edit %q under concurrency; final: %q", want, data)
		}
	}
}
