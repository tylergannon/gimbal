package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/tylergannon/polytype/goload"
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "gimbal-appgen-tests-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, name := range []string{"GIMBAL_GENERATE_CACHE_DIR", "POLYTYPE_CACHE_DIR", "SKGO_CACHE_DIR"} {
		if os.Getenv(name) == "" {
			if err := os.Setenv(name, filepath.Join(dir, name)); err != nil {
				fmt.Fprintln(os.Stderr, err)
				_ = os.RemoveAll(dir)
				os.Exit(1)
			}
		}
	}
	status := m.Run()
	if err := os.RemoveAll(dir); err != nil {
		fmt.Fprintln(os.Stderr, err)
		status = 1
	}
	os.Exit(status)
}

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
		if err := observationOutput(dir, outputDir, nil); err != nil {
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
	if err := observationOutput(filepath.Join(root, "internal/observation"), dir, nil); err != nil {
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

func TestObservationProjectionPersistentHitSkipsSharedSession(t *testing.T) {
	t.Setenv("POLYTYPE_CACHE_DIR", t.TempDir())
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module github.com/tylergannon/gimbal/internal/observation\n\ngo 1.27\n"), 0600); err != nil {
		t.Fatal(err)
	}
	source := "package observation\ntype InterviewRow struct { Question string }\ntype TurnRow struct { Original string }\n"
	path := filepath.Join(dir, "rows.go")
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	output := t.TempDir()
	var first goload.Session
	if err := observationOutput(dir, output, &first); err != nil {
		t.Fatal(err)
	}
	if stats := first.Stats(); stats.PackageChecks == 0 {
		t.Fatalf("cold projection did not use shared session: %+v", stats)
	}
	if err := os.Remove(filepath.Join(output, "types.ts")); err != nil {
		t.Fatal(err)
	}
	var warm goload.Session
	if err := observationOutput(dir, output, &warm); err != nil {
		t.Fatal(err)
	}
	if stats := warm.Stats(); stats != (goload.Stats{}) {
		t.Fatalf("persistent replay touched shared session: %+v", stats)
	}
	data, err := os.ReadFile(filepath.Join(output, "types.ts"))
	if err != nil || !strings.Contains(string(data), `"Original": string`) {
		t.Fatalf("missing literal restored field: %v\n%s", err, data)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(source, "Original string", "Edited bool", 1)), 0600); err != nil {
		t.Fatal(err)
	}
	if err := observationOutput(dir, output, &first); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(filepath.Join(output, "types.ts"))
	if err != nil || !strings.Contains(string(data), `"Edited": boolean`) || strings.Contains(string(data), `"Original"`) {
		t.Fatalf("shared session retained stale projection: %v\n%s", err, data)
	}
}
