package gimbal

import (
	"testing"

	"github.com/tylergannon/gimbal/workflow"
)

func TestRegisteredGraphReturnsCompiledWorkflow(t *testing.T) {
	const name = "registered-graph-accessor-test"
	want := workflow.Graph{Name: name, Source: workflow.Source{File: "workflow.go", Line: 12}}
	RegisterGraph(want)

	got, ok := RegisteredGraph(name)
	if !ok || got.Name != want.Name || got.Source != want.Source {
		t.Fatalf("RegisteredGraph(%q) = (%+v, %t), want (%+v, true)", name, got, ok, want)
	}
	if _, ok := RegisteredGraph("not-registered"); ok {
		t.Fatal("RegisteredGraph returned a graph for an unknown workflow")
	}
}
