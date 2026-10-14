package main

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/tylergannon/polytype/codegen"
)

func TestWorkflowWorkerPreservesSourceUnionAndRepairsOutput(t *testing.T) {
	t.Setenv("GOWORK", "off")
	t.Setenv("POLYTYPE_CACHE_DIR", t.TempDir())
	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	ordinary, retained := t.TempDir(), t.TempDir()
	options := func(out string) codegen.Options {
		return codegen.Options{TargetDir: filepath.Join(root, "workflow"), TypeScript: &codegen.TypeScriptOptions{Dir: out}}
	}
	if err := codegen.Generate(workflowConfiguration(), options(ordinary)); err != nil {
		t.Fatal(err)
	}
	var worker codegen.Worker
	defer func() { _ = worker.Close() }()
	if err := worker.Generate(workflowConfiguration(), options(retained)); err != nil {
		t.Fatal(err)
	}
	read := func(dir string) []byte {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(dir, "types.ts"))
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	generated := read(retained)
	if !bytes.Equal(generated, read(ordinary)) {
		t.Fatal("retained workflow projection differs from ordinary producer")
	}
	for _, literal := range []string{`export type Graph =`, `"?kind"?: "agent_call"`, `"?kind"?: "promise_loop"`, `"?diagnostics"?: Array<Diagnostic>`} {
		if !regexp.MustCompile(literal).Match(generated) {
			t.Fatalf("missing literal workflow declaration %s", literal)
		}
	}
	before := worker.Stats()
	if err := os.WriteFile(filepath.Join(retained, "types.ts"), []byte("broken generated output"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := worker.Generate(workflowConfiguration(), options(retained)); err != nil {
		t.Fatal(err)
	}
	after := worker.Stats()
	if after.Analyses != before.Analyses || after.Emissions != before.Emissions || after.Replays != before.Replays+1 {
		t.Fatalf("unchanged workflow repeated producer work: %+v -> %+v", before, after)
	}
	if !bytes.Equal(read(retained), generated) {
		t.Fatal("workflow output not repaired")
	}
}
