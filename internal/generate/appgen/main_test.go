package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestObservationProjectionUsesFreshSource(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module github.com/tylergannon/gimbal/internal/observation\n\ngo 1.27\n"), 0644); err != nil {
		t.Fatal(err)
	}
	outputDir := t.TempDir()
	project := func(source string) (string, error) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, "rows.go"), []byte("package observation\ntype InterviewRow struct { Question string `json:\"question\"` }\n"+source), 0644); err != nil {
			t.Fatal(err)
		}
		if err := observationOutput(dir, outputDir); err != nil {
			return "", err
		}
		output, err := os.ReadFile(filepath.Join(outputDir, "types.ts"))
		return string(output), err
	}
	before, err := project("type TurnRow struct { Original string `json:\"original\"` }\n")
	if err != nil {
		t.Fatal(err)
	}
	after, err := project("// TurnRow changed in this invocation.\ntype TurnRow struct { Edited bool `json:\"edited\"` }\n")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(before, `"original": string`) || !strings.Contains(after, `"edited": boolean`) || strings.Contains(after, `"original"`) || !strings.Contains(after, "changed in this invocation") {
		t.Fatalf("projection did not reflect fresh symbols/comments:\nbefore: %s\nafter: %s", before, after)
	}
	_, err = project("type TurnRow struct { Unsupported chan string }\n")
	if err == nil || (!strings.Contains(err.Error(), "TurnRow") || !strings.Contains(err.Error(), "ChanType")) {
		t.Fatalf("lost source diagnostic: %v", err)
	}
}

func TestObservationProjectionMatchesApplicationTypes(t *testing.T) {
	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := observationOutput(filepath.Join(root, "internal/observation"), dir); err != nil {
		t.Fatal(err)
	}
	actual, err := os.ReadFile(filepath.Join(dir, "types.ts"))
	if err != nil {
		t.Fatal(err)
	}
	expected, err := os.ReadFile(filepath.Join(root, "web/src/lib/observation/types.ts"))
	if err != nil {
		t.Fatal(err)
	}
	// The committed application runs Prettier, which removes optional quotes
	// from identifier-shaped property names. Compare everything else exactly.
	property := regexp.MustCompile(`(?m)^(  )"([a-z_]+)"(\??:)`)
	if property.ReplaceAllString(string(actual), "$1$2$3") != string(expected) {
		t.Fatal("public generation changed application observation declarations")
	}
	if !strings.Contains(string(actual), `"question_id": string`) || !strings.Contains(string(actual), `"unresolved": boolean`) {
		t.Fatal("missing literal observation fields")
	}
}
