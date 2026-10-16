package gimbal

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/tylergannon/gimbal/workflow"
)

func TestRegisteredGraphReturnsCompiledWorkflow(t *testing.T) {
	const name = "registered-graph-accessor-test"
	want := workflow.Graph{Name: name, Source: workflow.Source{File: "workflow.go", Line: 12}}
	d := testDefinition(name)
	d.Graph = want
	if err := DefaultRegistry.RegisterWorkflow(d); err != nil {
		t.Fatal(err)
	}

	got, ok := RegisteredGraph(name)
	if !ok || got.Name != want.Name || got.Source != want.Source {
		t.Fatalf("RegisteredGraph(%q) = (%+v, %t), want (%+v, true)", name, got, ok, want)
	}
	if _, ok := RegisteredGraph("not-registered"); ok {
		t.Fatal("RegisteredGraph returned a graph for an unknown workflow")
	}
}

func TestRunSavesTheDesignerSourcePageBesideItsGraph(t *testing.T) {
	const name = "run-saves-source-page-test"
	d := testDefinition(name)
	d.SourcePage = json.RawMessage(`{"name":"run-saves-source-page-test","body":[]}`)
	d.Graph.Body = []workflow.Operation{}
	d.Graph.Services = []workflow.Service{}
	if err := DefaultRegistry.RegisterWorkflow(d); err != nil {
		t.Fatal(err)
	}
	project := t.TempDir()
	if err := Run(Project(t.Context(), project), name, nil, func(context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}
	saved, err := filepath.Glob(filepath.Join(project, "runs", "*", "source.json"))
	if err != nil || len(saved) != 1 {
		t.Fatalf("source.json files = %v, %v; want one", saved, err)
	}
	raw, err := os.ReadFile(saved[0])
	if err != nil || string(raw) != string(d.SourcePage) {
		t.Fatalf("source.json = %q, %v; want %q", raw, err, d.SourcePage)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(saved[0]), "graph.json")); err != nil {
		t.Fatalf("graph.json beside source.json: %v", err)
	}
}
