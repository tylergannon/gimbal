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

func TestMermaidRendersPromiseLoopSupervisors(t *testing.T) {
	graph := workflow.Graph{
		Name: "supervised-loop",
		Body: []workflow.Operation{workflow.PromiseLoop{
			Name:    "tasks",
			Planner: "planner",
			Supervisors: []workflow.Supervisor{{
				Session: "coach",
				Role:    "scope-coach",
			}},
		}},
	}

	got := generate.Mermaid(graph)
	for _, want := range [][]byte{
		[]byte(`supervisor<br/>scope-coach<br/>coach`),
		[]byte(`-. "watches" .->`),
	} {
		if !bytes.Contains(got, want) {
			t.Errorf("Mermaid output lacks %q:\n%s", want, got)
		}
	}
}
