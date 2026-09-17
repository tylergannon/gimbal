package generate_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tylergannon/gimble/internal/generate"
	"github.com/tylergannon/gimble/workflow"
)

// TestControlFlowGraph reads a compact workflow fixture and checks the main
// control-flow, nested scope, loop, session, call, and command shapes.
func TestControlFlowGraph(t *testing.T) {
	g, err := generate.Extract("testdata/fixture", "SprintShape", "sprint")
	if err != nil {
		t.Fatal(err)
	}
	if g.Name != "sprint" {
		t.Fatalf("name = %q", g.Name)
	}
	rounds, ok := find[workflow.Repeat](g.Body, func(r workflow.Repeat) bool { return strings.Contains(r.Cond, "round") })
	if !ok {
		t.Fatal("the sprint's rounds are a Repeat in the entry body")
	}
	round, ok := find[workflow.Scope](rounds.Body, func(s workflow.Scope) bool { return s.Name == "round" })
	if !ok {
		t.Fatal("the rounds hold the scope round")
	}
	loop, ok := find[workflow.Loop](round.Body, func(l workflow.Loop) bool { return l.Name == "sprint" })
	if !ok {
		t.Fatal("the round holds the loop sprint")
	}
	if loop.Planner != "planner" {
		t.Errorf("the loop's planner = %q, want planner", loop.Planner)
	}

	coder, ok := find[workflow.AgentCall](loop.Body, func(c workflow.AgentCall) bool { return c.Session == "coder" })
	if !ok {
		t.Fatal("the task body holds the coder's call")
	}
	if coder.Role != "researcher" {
		t.Errorf("the coder's role = %q, want researcher, the session it was forked from", coder.Role)
	}
	if gitCommands(loop.Body) == 0 {
		t.Fatal("the loop holds the conditional git command")
	}
}

// gitCommands counts the git commands in body and in the branches of its
// conditions, which is how deep the sprint's commit sequence nests.
func gitCommands(body []workflow.Operation) int {
	commands := 0
	for _, op := range body {
		switch op := op.(type) {
		case workflow.Command:
			if op.Name == "git" {
				commands++
			}
		case workflow.Condition:
			for _, branch := range op.Branches {
				commands += gitCommands(branch.Body)
			}
		}
	}
	return commands
}

// TestFixtureGraph reads the fixture workflow, which holds the sites the
// rules read and the sites they refuse to guess at.
func TestFixtureGraph(t *testing.T) {
	g, err := generate.Extract("testdata/fixture", "Fixture", "fixture")
	if err != nil {
		t.Fatal(err)
	}

	pair, ok := find[workflow.Group](g.Body, func(gr workflow.Group) bool { return gr.Name == "pair" })
	if !ok {
		t.Fatal("the fixture holds the group pair")
	}
	if len(pair.Children) != 2 || pair.Children[0].Name != "left" || pair.Children[1].Name != "right" {
		t.Fatalf("the group's children are left and right, got %+v", pair.Children)
	}
	for _, child := range pair.Children {
		if _, ok := find[workflow.Set](child.Body, func(s workflow.Set) bool { return s.Key == child.Name }); !ok {
			t.Errorf("the child %q writes its own key", child.Name)
		}
	}

	work, ok := find[workflow.AgentCall](g.Body, func(c workflow.AgentCall) bool { return c.Prompt == "do the work" })
	if !ok {
		t.Fatal("the fixture holds the lead's call")
	}
	if len(work.Supervisors) != 1 || work.Supervisors[0].Role != "watcher" {
		t.Fatalf("the call is watched by the watcher, got %+v", work.Supervisors)
	}
	watched := work.Supervisors[0].Supervisors
	if len(watched) != 1 || watched[0].Role != "chief" {
		t.Fatalf("the watcher is watched by the chief, got %+v", watched)
	}

	late, ok := find[workflow.Condition](g.Body, func(c workflow.Condition) bool {
		return len(c.Branches) == 1 && c.Branches[0].Case == "!passed"
	})
	if !ok {
		t.Fatal("the inlined helper's late return is a branch of the body it was inlined into")
	}
	if !late.Branches[0].Exits {
		t.Error("the helper's late return exits")
	}

	// A session bound in a branch does not survive the join.
	chosen, ok := find[workflow.Condition](g.Body, func(c workflow.Condition) bool {
		return len(c.Branches) == 2 && c.Branches[0].Case == "roleName() == \"reader\""
	})
	if !ok {
		t.Fatal("the fixture chooses a session in a branch")
	}
	for i, name := range []string{"first", "second"} {
		if _, ok := find[workflow.Session](chosen.Branches[i].Body, func(s workflow.Session) bool { return s.Name == name }); !ok {
			t.Errorf("branch %d declares the session %q", i, name)
		}
	}
	if _, ok := find[workflow.AgentCall](g.Body, func(c workflow.AgentCall) bool { return c.Session == "second" }); ok {
		t.Error("the call after the join names no session the source declares, so it is a diagnostic")
	}

	// A group is started in the body that declares it.
	held, ok := find[workflow.Group](g.Body, func(gr workflow.Group) bool { return gr.Name == "held" })
	if !ok {
		t.Fatal("the fixture holds the group held")
	}
	if len(held.Children) != 0 {
		t.Errorf("a Go inside a branch is not read, so the group stays childless, got %+v", held.Children)
	}

	// A group reassigned after its declaration is not read, and neither is
	// what is started on it.
	one, ok := find[workflow.Group](g.Body, func(gr workflow.Group) bool { return gr.Name == "one" })
	if !ok {
		t.Fatal("the fixture holds the group one")
	}
	if len(one.Children) != 0 {
		t.Errorf("nothing is added to a group whose identifier was reassigned, got %+v", one.Children)
	}

	// A Gimble call inside another call's arguments is a diagnostic, not a node.
	if _, ok := find[workflow.Session](g.Body, func(s workflow.Session) bool { return s.Name == "inner" }); ok {
		t.Error("a session created inside another call's arguments is a diagnostic, not a node")
	}

	// A callback's early return is shape even inside an inlined helper.
	guarded, ok := find[workflow.Scope](g.Body, func(s workflow.Scope) bool { return s.Name == "guarded" })
	if !ok {
		t.Fatal("the fixture holds the scope guarded")
	}
	skip, ok := find[workflow.Condition](guarded.Body, func(c workflow.Condition) bool {
		return len(c.Branches) == 1 && c.Branches[0].Case == "skipped(ctx)"
	})
	if !ok || !skip.Branches[0].Exits {
		t.Error("a callback's early return is its own control flow, not a helper's guard")
	}

	// Fourteen sites the rules refuse to guess at; reassignments and the
	// calls on what they left unbound share messages, so there are nine
	// distinct ones.
	const sites = 14
	want := []string{
		"NewSession's role is not a constant",
		"a call through a function value is not read",
		"ends only the helper it is written in",
		"a session reassigned after its declaration is not read",
		"Generate is called on something that is not a session",
		"Go on a group declared outside this body is not read",
		"a group reassigned after its declaration is not read",
		"Go is called on something that is not a group",
		"a Gimble call nested in a Gimble call's arguments is not read",
	}
	if len(g.Diagnostics) != sites {
		t.Fatalf("the fixture has %d diagnostics, want %d: %v", len(g.Diagnostics), sites, g.Diagnostics)
	}
	for _, message := range want {
		found := false
		for _, d := range g.Diagnostics {
			if strings.Contains(d.Message, message) {
				found = true
			}
		}
		if !found {
			t.Errorf("no diagnostic says %q: %v", message, g.Diagnostics)
		}
	}
	if _, ok := find[workflow.Session](g.Body, func(s workflow.Session) bool { return s.Name == "reader" }); ok {
		t.Error("a session whose role the source does not spell out is a diagnostic, not a node")
	}
}

// TestSourceWritesTheWorkflowsPackage checks that the writer names the
// package the workflow is in, not the external test package beside it, and
// that it honours an absolute output path.
func TestSourceWritesTheWorkflowsPackage(t *testing.T) {
	output := filepath.Join(t.TempDir(), "workflow_gen.go")
	if err := generate.Source("testdata/fixture", "Fixture", "fixture", output); err != nil {
		t.Fatal(err)
	}
	written, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"//go:build !jsonschema",
		"\npackage fixture\n",
		"func init() { gimble.RegisterGraph(Graph) }",
		"var Graph = workflow.Graph{",
		"func Command(defaults map[gimble.WorkflowRole]string) *cobra.Command {",
		`Use:   "fixture",`,
		`cmd.Flags().StringVar(&workDir, "work-dir", ".", "the working directory for this run")`,
		`cmd.Flags().StringVar(&leadModel, "lead", defaults[gimble.WorkflowRole("lead")], "the model for role lead, as model or model:effort")`,
		`if defaults[gimble.WorkflowRole("lead")] == "" {`,
		`env := gimble.Env{WorkDir: workDir}`,
		`return Fixture(ctx, env) })`,
	} {
		if !strings.Contains(string(written), want) {
			t.Errorf("the generated file lacks %q:\n%s", want, firstLines(string(written)))
		}
	}
}

func TestGeneratedCommandUsesCentralizedRoleDefaults(t *testing.T) {
	output := filepath.Join(t.TempDir(), "workflow_gen.go")
	if err := generate.Source("testdata/fixture", "Fixture", "fixture", output); err != nil {
		t.Fatal(err)
	}
	written, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	text := string(written)
	if strings.Contains(text, "roles[") {
		t.Fatalf("generated command still reads workflow roles")
	}
	for _, want := range []string{`defaults[gimble.WorkflowRole("lead")]`, `if defaults[gimble.WorkflowRole("lead")] == ""`, `gimble.WorkflowRole("lead"): leadModel`} {
		if !strings.Contains(text, want) {
			t.Errorf("generated command lacks %q", want)
		}
	}
}

func TestGeneratedEntryRequiresEnv(t *testing.T) {
	_, err := generate.Extract("testdata/fixture", "MissingEnv", "missing-env")
	if err == nil || !strings.Contains(err.Error(), "second parameter is not gimble.Env") {
		t.Fatalf("Extract MissingEnv error = %v", err)
	}
}

func TestWorkflowParamsCannotClaimWorkDir(t *testing.T) {
	err := generate.Source("testdata/fixture", "HasWorkDirParams", "has-work-dir-params", filepath.Join(t.TempDir(), "workflow_gen.go"))
	if err == nil || !strings.Contains(err.Error(), "parameter field WorkDir would be --work-dir, which is the Gimble environment's") {
		t.Fatalf("Source HasWorkDirParams error = %v", err)
	}
}

func firstLines(text string) string {
	lines := strings.SplitN(text, "\n", 6)
	return strings.Join(lines[:min(len(lines), 5)], "\n")
}

// find returns the first operation of type T in a body that answers want.
func find[T workflow.Operation](body []workflow.Operation, want func(T) bool) (T, bool) {
	for _, op := range body {
		if node, ok := op.(T); ok && want(node) {
			return node, true
		}
	}
	var zero T
	return zero, false
}
