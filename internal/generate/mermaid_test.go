package generate_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/tylergannon/gimble/internal/generate"
	"github.com/tylergannon/gimble/workflow"
)

func TestMermaidGolden(t *testing.T) {
	graph := workflow.Graph{
		Name:     `all & "escaped"`,
		Services: []workflow.Service{{Name: "root service"}},
		Body: []workflow.Operation{
			workflow.Session{Name: "lead"},
			workflow.Session{Name: "copy", From: "lead"},
			workflow.AgentCall{Session: "copy", Supervisors: []workflow.Supervisor{{
				Session: "watcher", Role: "reviewer", Supervisors: []workflow.Supervisor{{Session: "chief", Role: "chief"}},
			}}},
			workflow.Interview{Name: "requirements", Session: "lead"},
			workflow.Command{Name: "tests"},
			workflow.Set{Key: "evidence"},
			workflow.Scope{Name: "delivery", Services: []workflow.Service{{Name: "preview"}}, Body: []workflow.Operation{
				workflow.Set{Key: "inside"},
			}},
			workflow.PromiseLoop{Name: "tasks", Planner: "lead", Services: []workflow.Service{{Name: "task service"}}, Body: []workflow.Operation{
				workflow.Command{Name: "task check"},
			}},
			workflow.Iterate{Name: "review", Body: []workflow.Operation{workflow.AgentCall{Session: "lead"}}},
			workflow.Repeat{Cond: `i < 2 && ready`, Body: []workflow.Operation{workflow.Command{Name: "again"}}},
			workflow.Group{Name: "pair", Children: []workflow.GroupChild{
				{Name: "left", Body: []workflow.Operation{workflow.Set{Key: "left"}}},
				{Name: "right", Services: []workflow.Service{{Name: "fixture"}}, Body: []workflow.Operation{workflow.Set{Key: "right"}}},
			}},
			workflow.Condition{Branches: []workflow.Branch{
				{Case: `result == "pass" | retried`, Body: []workflow.Operation{workflow.Command{Name: "publish"}}},
				{Case: `failed`, Exits: true},
				{Body: []workflow.Operation{workflow.Set{Key: "unknown"}}},
			}},
		},
		Diagnostics: []workflow.Diagnostic{{Message: `couldn't read <call>`}},
	}

	got := generate.Mermaid(graph)
	want, err := os.ReadFile("testdata/mermaid.golden")
	if err != nil {
		t.Fatalf("read golden: %v\n%s", err, got)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("Mermaid output differs from golden\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
	if again := generate.Mermaid(graph); !bytes.Equal(got, again) {
		t.Fatal("Mermaid output changed between identical renders")
	}
}

func TestSourceWritesMermaidAndSvelteComponent(t *testing.T) {
	dir := t.TempDir()
	goOutput := dir + "/workflow_gen.go"
	mermaidOutput := dir + "/fixture.mmd"
	if err := generate.Source("testdata/fixture", "Fixture", "fixture", goOutput, mermaidOutput); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{goOutput, mermaidOutput, dir + "/fixture.svelte"} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("generated output %s: %v", path, err)
		}
	}
	component, err := os.ReadFile(dir + "/fixture.svelte")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range [][]byte{
		[]byte(`import src from "./fixture.svg";`),
		[]byte(`<WorkflowDiagram title="fixture workflow" {src} />`),
	} {
		if !bytes.Contains(component, want) {
			t.Errorf("generated component lacks %q:\n%s", want, component)
		}
	}
}

func TestMermaidOmitsProgramDetails(t *testing.T) {
	graph := workflow.Graph{
		Name:     "shape",
		Services: []workflow.Service{{Name: "server"}},
		Body: []workflow.Operation{workflow.PromiseLoop{
			Name: "tasks",
			Body: []workflow.Operation{
				workflow.Set{Key: "task"},
				workflow.Session{Name: "worker"},
			},
			Supervisors: []workflow.Supervisor{{
				Session: "coach",
				Role:    "scope-coach",
			}},
		}},
	}

	got := generate.Mermaid(graph)
	for _, unwanted := range [][]byte{
		[]byte(`set<br/>`),
		[]byte(`session<br/>`),
		[]byte(`service<br/>`),
		[]byte(`supervisor<br/>`),
	} {
		if bytes.Contains(got, unwanted) {
			t.Errorf("Mermaid output contains program detail %q:\n%s", unwanted, got)
		}
	}
	if !bytes.Contains(got, []byte(`subgraph s2["Loop · tasks"]`)) {
		t.Errorf("Mermaid output lacks loop box:\n%s", got)
	}
}

func TestMermaidSummarizesFanOutArms(t *testing.T) {
	graph := workflow.Graph{Name: "fan-out", Body: []workflow.Operation{workflow.Group{
		Name: "workers",
		Children: []workflow.GroupChild{{Name: "worker1", Body: []workflow.Operation{
			workflow.Session{Name: "writer"},
			workflow.AgentCall{Session: "writer", Role: "document-authoring"},
			workflow.Set{Key: "draft"},
			workflow.AgentCall{Session: "writer", Role: "document-authoring"},
			workflow.Command{Name: "count-tokens"},
		}}},
	}}}

	got := generate.Mermaid(graph)
	for _, want := range [][]byte{
		[]byte(`Arm · worker1<br/>2× Generate · document-authoring<br/>Command · count-tokens`),
		[]byte(`subgraph s2["Fan out · workers"]`),
	} {
		if !bytes.Contains(got, want) {
			t.Errorf("Mermaid output lacks %q:\n%s", want, got)
		}
	}
}

func TestMermaidKeepsConditionLabelsPresentational(t *testing.T) {
	graph := workflow.Graph{Name: "condition", Body: []workflow.Operation{workflow.Condition{
		Branches: []workflow.Branch{{
			Case: `ready := strings.TrimSpace(result.Ready); ready != ""`,
			Body: []workflow.Operation{workflow.Command{Name: "readiness"}},
		}},
	}}}

	got := generate.Mermaid(graph)
	if !bytes.Contains(got, []byte(`-->|"ready != &#34;&#34;"|`)) {
		t.Errorf("Mermaid output lacks the final predicate:\n%s", got)
	}
	if bytes.Contains(got, []byte(`ready :=`)) {
		t.Errorf("Mermaid output includes assignment machinery:\n%s", got)
	}
}
