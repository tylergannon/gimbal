package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func put(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0644); err != nil {
		t.Fatal(err)
	}
}
func TestRoutingEvidenceFixture(t *testing.T) {
	root := t.TempDir()
	put(t, filepath.Join(root, "README.md"), "[routes](topics.md) [escape](../outside.md)\n")
	put(t, filepath.Join(root, "topics.md"), "Streams [leaf](repos/pdf.md)\n")
	put(t, filepath.Join(root, "repos/pdf.md"), "AcroForm appearance streams [cycle](../README.md)")
	record := eval{ID: "q1", Query: "AcroForm appearance streams", Routes: []string{"topics.md"}, Citations: []string{"repos/pdf.md", "missing.md"}}
	result := scoreQuery(record, root, 6, 30)
	if result["files_read"] != 3 || result["citation_precision"] != 0.5 || result["route_found"] != true || result["tool_call_estimate"] != 2 || result["path_length_to_first_hit"] != 1 {
		t.Fatalf("%#v", result)
	}
	if len(route(filepath.Join(root, "README.md"), root, map[string]bool{}, 0, 30)) != 1 {
		t.Fatal("hop limit")
	}
	if scoreQuery(record, root, 6, 1)["files_read"] != 1 {
		t.Fatal("file limit")
	}
	if resolveLink("../outside.md", filepath.Join(root, "README.md"), root) != "" {
		t.Fatal("escape accepted")
	}
	missing := scoreQuery(record, t.TempDir(), 6, 30)
	if missing["error"] != "no_entrypoint" {
		t.Fatal(missing)
	}
	agg := aggregate([]map[string]any{result, missing})
	if agg["success_at_1"] != 0.5 || agg["mean_tool_call_estimate"] != 1.0 {
		t.Fatal(agg)
	}
}
func TestJSONLAndWrittenRun(t *testing.T) {
	root := t.TempDir()
	put(t, filepath.Join(root, "README.md"), "Appearance")
	put(t, filepath.Join(root, ".semantic-index/evals.jsonl"), "# comment\nnot json\n{\"id\":\"q\",\"query\":\"appearance\",\"expected_citations\":[\"README.md\"]}\n")
	var out bytes.Buffer
	if err := run([]string{"--index", root, "--write-results"}, &out); err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob(filepath.Join(root, ".semantic-index/benchmarks/run-*.json"))
	if err != nil || len(files) != 1 {
		t.Fatal(files, err)
	}
	if !bytes.Contains(out.Bytes(), []byte("skipping evals line 2")) {
		t.Fatal(out.String())
	}
}
