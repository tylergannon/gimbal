package main

import (
	"bytes"
	"encoding/json"
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
func TestMetricsFixture(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	put(t, filepath.Join(root, "README.md"), "[routes](topics.md) [leaf](leaves/one.md) `leaves/two.md`\n")
	put(t, filepath.Join(root, "topics.md"), "[evidence](https://example.org) source.go:L2-L4 TODO\n")
	put(t, filepath.Join(root, "leaves/one.md"), "One source.md:2\n")
	put(t, filepath.Join(root, "leaves/two.md"), "Two FIXME\n")
	put(t, filepath.Join(root, "leaves/orphan.md"), "No pointer\n")
	put(t, filepath.Join(root, ".semantic-index/evals.jsonl"), "{}\n")
	report, err := indexMetrics(root)
	if err != nil {
		t.Fatal(err)
	}
	if report["file_count"] != 6 || report["routing_file_count"] != 3 || report["likely_leaf_count"] != 3 || report["orphan_leaf_count"] != 1 || report["citation_count"] != 3 || report["debt_marker_count"] != 2 || report["benchmark_file_count"] != 1 {
		t.Fatalf("%#v", report)
	}
	orphan := report["orphan_leaves"].([]string)
	if len(orphan) != 1 || orphan[0] != "leaves/orphan.md" {
		t.Fatal(orphan)
	}
	fan := report["fan_out"].(map[string]any)
	if fan["mean"] != 4.0 || fan["max"] != 4 || fan["variance"] != 0.0 {
		t.Fatal(fan)
	}
}
func TestStateAndSourceFixture(t *testing.T) {
	root := t.TempDir()
	source := t.TempDir()
	put(t, filepath.Join(root, "README.md"), "Index")
	put(t, filepath.Join(source, "source.txt"), "Corpus")
	var out bytes.Buffer
	if err := run([]string{"--index", root, "--source", source, "--write-state", "--mode", "rebalance"}, &out); err != nil {
		t.Fatal(err)
	}
	var report map[string]any
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	path := report["state_written"].(string)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var state map[string]any
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatal(err)
	}
	if state["last_mode"] != "rebalance" || state["source"].(map[string]any)["file_count"] != 1.0 {
		t.Fatal(state)
	}
	missing, err := indexMetrics(filepath.Join(root, "missing"))
	if err != nil || missing["file_count"] != 0 || missing["exists"] != false {
		t.Fatal(missing, err)
	}
}
