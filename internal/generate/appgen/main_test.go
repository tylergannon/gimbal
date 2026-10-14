package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tylergannon/polytype/grammar"
	"golang.org/x/tools/go/packages"
)

func TestObservationProjectionUsesFreshCallerSymbols(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/observation\n\ngo 1.27\n"), 0644); err != nil {
		t.Fatal(err)
	}
	load := &packages.Config{Dir: dir, Mode: grammar.LoadMode | packages.NeedDeps, BuildFlags: []string{"-tags=jsonschema"}}
	project := func(source string) (string, error) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, "rows.go"), []byte("package observation\ntype InterviewRow struct { Question string `json:\"question\"` }\n"+source), 0644); err != nil {
			t.Fatal(err)
		}
		roots, err := packages.Load(load, ".")
		if err != nil {
			t.Fatal(err)
		}
		output, err := observationOutput(load, roots[0])
		if err != nil {
			return "", err
		}
		return string(output.Files[0].Content), nil
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
	if err == nil || !strings.Contains(err.Error(), "channels are outside the static type grammar") {
		t.Fatalf("lost source diagnostic: %v", err)
	}
}
