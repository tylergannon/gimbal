package edittools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeForFuzzyMatch(t *testing.T) {
	input := "  keep  \n\u201csmart\u201d \u2014 dash \u00a0space\n"
	got := NormalizeForFuzzyMatch(input)
	want := "  keep\n\"smart\" - dash  space\n"
	if got != want {
		t.Fatalf("NormalizeForFuzzyMatch = %q, want %q", got, want)
	}
}

func TestFuzzyFindTextExactPreferred(t *testing.T) {
	content := "A\nKEEP   \n"
	match := FuzzyFindText(content, "A")
	if !match.Found || match.UsedFuzzyMatch {
		t.Fatalf("expected exact match, got %#v", match)
	}
	if match.Index != 0 || match.MatchLength != 1 {
		t.Fatalf("unexpected match offsets: %#v", match)
	}
}

func TestFuzzyFindTextFallsBack(t *testing.T) {
	content := "foo()   \n"
	match := FuzzyFindText(content, "foo()\n")
	if !match.Found || !match.UsedFuzzyMatch {
		t.Fatalf("expected fuzzy match, got %#v", match)
	}
	if match.ContentForReplacement != "foo()\n" {
		t.Fatalf("replacement content = %q", match.ContentForReplacement)
	}
}

func TestApplyEditsFuzzyTrailingWhitespace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.go")
	writeTestFile(t, path, "func main() {\n    foo()   \n}\n")

	_, err := executeEdit(t, EditTool(dir, nil), path, Edit{OldText: "    foo()\n", NewText: "    bar()\n"})
	if err != nil {
		t.Fatalf("fuzzy edit failed: %v", err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "bar()") {
		t.Fatalf("edit not applied: %q", data)
	}
}

func TestApplyEditsFuzzySmartQuotes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	writeTestFile(t, path, "print(\"hello\")\n")

	_, err := executeEdit(t, EditTool(dir, nil), path, Edit{OldText: "print(\u201chello\u201d)", NewText: "print(\"world\")"})
	if err != nil {
		t.Fatalf("fuzzy edit failed: %v", err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "world") {
		t.Fatalf("edit not applied: %q", data)
	}
}

func TestApplyEditsExactPreservesUntouchedLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	writeTestFile(t, path, "a\nKEEP   \nb\n")

	_, err := executeEdit(t, EditTool(dir, nil), path, Edit{OldText: "a", NewText: "A"})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "A\nKEEP   \nb\n" {
		t.Fatalf("exact-match edit should not normalize untouched lines: %q", data)
	}
}

func TestApplyEditsFuzzyPreservesUntouchedLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	writeTestFile(t, path, "replace me   \nafter   \n")

	_, err := executeEdit(t, EditTool(dir, nil), path, Edit{OldText: "replace me\n", NewText: "after\n"})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if want := "after\nafter   \n"; string(data) != want {
		t.Fatalf("fuzzy edit must preserve untouched lines: got %q, want %q", data, want)
	}
}

func TestApplyEditsFuzzyMultiEditPreservesUntouchedLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	original := strings.Join([]string{
		"keep before  ",
		"first target  ",
		"first after",
		"keep middle   ",
		"second target  ",
		"second after",
		"keep after  ",
		"",
	}, "\n")
	writeTestFile(t, path, original)

	_, err := executeEdit(t, EditTool(dir, nil), path,
		Edit{OldText: "first target\nfirst after", NewText: "FIRST\nFIRST2"},
		Edit{OldText: "second target\nsecond after", NewText: "SECOND\nSECOND2"},
	)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	want := strings.Join([]string{
		"keep before  ",
		"FIRST",
		"FIRST2",
		"keep middle   ",
		"SECOND",
		"SECOND2",
		"keep after  ",
		"",
	}, "\n")
	if string(data) != want {
		t.Fatalf("multi fuzzy edit must preserve untouched lines:\n got %q\nwant %q", data, want)
	}
}

func TestApplyEditsNoChange(t *testing.T) {
	_, err := ApplyEditsToNormalizedContent("hello\n", []Edit{{OldText: "hello", NewText: "hello"}}, "f.txt")
	if err == nil || !strings.Contains(err.Error(), "No changes made") {
		t.Fatalf("expected no-change error, got %v", err)
	}
}

func TestApplyEditsEmptyOldText(t *testing.T) {
	_, err := ApplyEditsToNormalizedContent("hello\n", []Edit{{OldText: "", NewText: "x"}}, "f.txt")
	if err == nil || !strings.Contains(err.Error(), "oldText must not be empty") {
		t.Fatalf("expected empty-oldText error, got %v", err)
	}
}

func TestApplyEditsPreservesCRLF(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	writeTestFile(t, path, "line1\r\nTARGET\r\nline3\r\n")

	_, err := executeEdit(t, EditTool(dir, nil), path, Edit{OldText: "TARGET", NewText: "CHANGED"})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "\r\n") || !strings.Contains(string(data), "CHANGED") {
		t.Fatalf("CRLF endings not preserved: %q", data)
	}
	if strings.Contains(string(data), "line1\n") {
		t.Fatalf("CRLF collapsed to LF: %q", data)
	}
}

func TestApplyEditsPreservesBOM(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	writeTestFile(t, path, "\ufeffhello\n")

	_, err := executeEdit(t, EditTool(dir, nil), path, Edit{OldText: "hello", NewText: "world"})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "\ufeffworld\n" {
		t.Fatalf("BOM not preserved: %q", data)
	}
}

func TestGenerateDiffStringAndPatch(t *testing.T) {
	diff, first, ok := GenerateDiffString("Hello, world!", "Hello, testing!")
	if !ok || first != 1 {
		t.Fatalf("first changed line = %d, ok=%v", first, ok)
	}
	if !strings.Contains(diff, "+1 Hello, testing!") {
		t.Fatalf("diff = %q", diff)
	}
	patch := GenerateUnifiedPatch("file.txt", "Hello, world!", "Hello, testing!")
	for _, want := range []string{"--- file.txt", "+++ file.txt", "@@ -1,1 +1,1 @@", "-Hello, world!", "+Hello, testing!", "\\ No newline at end of file"} {
		if !strings.Contains(patch, want) {
			t.Fatalf("patch missing %q:\n%s", want, patch)
		}
	}
}
