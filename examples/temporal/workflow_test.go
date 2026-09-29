package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/contextdata"
	"go.temporal.io/sdk/testsuite"
)

// These are the intended output contract; update them with an intentional
// workflow change, independently of the generated implementation.
const expectedDeliverable = "delivery.txt"
const expectedCheck = "delivery"

// This exercises real Gimbal operations and shell execution with Temporal's
// in-process scheduler. The provider alone is replaced with deterministic text.
func TestDelivery(t *testing.T) {
	t.Setenv("TYPESAFE_API_KEY", "test-key")
	project := t.TempDir()
	store := contextdata.Store{Root: t.TempDir()}
	a := &Activities{workdir: project, store: store, rootContext: gimbal.Project(t.Context(), project), models: map[gimbal.WorkflowRole]gimbal.ModelBinding{coder: {Adapter: &demoAdapter{}, Model: "deterministic-demo"}}}
	a.open = func(ctx context.Context, name string, models map[gimbal.WorkflowRole]gimbal.ModelBinding, initial contextdata.Snapshot, local string) (context.Context, func(error) error, error) {
		return gimbal.OpenRun(ctx, name, models, initial, gimbal.ContextAccess{Store: &a.store, LocalDir: local})
	}
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterActivity(a)
	env.RegisterActivity(ProvisionEnvironment)
	env.RegisterActivity(ReleaseEnvironment)
	env.ExecuteWorkflow(DeliveryWorkflow, Input{})
	if err := env.GetWorkflowError(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(project, expectedDeliverable))
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "done\n" {
		t.Fatalf("deliverable = %q", raw)
	}
	if len(a.scopes) != 0 {
		t.Fatalf("unclosed scopes: %v", a.scopes)
	}
}

// Regenerate in a clean separate module, change ordinary authored source, and
// require the changed check in both generated graph and actual file execution.
func TestSourceMaintenance(t *testing.T) {
	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	err = filepath.WalkDir(original, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(original, path)
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(dir, rel), 0755)
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if rel == "go.mod" {
			data = []byte(strings.ReplaceAll(string(data), "=> ../..", "=> "+root))
		}
		return os.WriteFile(filepath.Join(dir, rel), data, 0644)
	})
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(dir, "authored/workflow.go")
	raw, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	changed := strings.ReplaceAll(string(raw), expectedDeliverable, "maintenance-probe.txt")
	changed = strings.Replace(changed, `"`+expectedCheck+`", env.WorkDir`, `"maintenance-probe", env.WorkDir`, 1)
	if changed == string(raw) || !strings.Contains(changed, `"maintenance-probe", env.WorkDir`) || !strings.Contains(changed, "maintenance-probe.txt") {
		t.Fatal("source no longer matches the intended maintenance fixture; update its explicit test contract")
	}
	if err = os.WriteFile(source, []byte(changed), 0644); err != nil {
		t.Fatal(err)
	}
	// The disposable consumer updates its independent acceptance assertion too,
	// exactly as the maintenance guide requires for an intended behavior change.
	testPath := filepath.Join(dir, "workflow_test.go")
	testSource, err := os.ReadFile(testPath)
	if err != nil {
		t.Fatal(err)
	}
	updatedTest := strings.Replace(string(testSource), `const expectedDeliverable = "`+expectedDeliverable+`"`, `const expectedDeliverable = "maintenance-probe.txt"`, 1)
	if updatedTest == string(testSource) {
		t.Fatal("expected-output assertion was not updated")
	}
	if err = os.WriteFile(testPath, []byte(updatedTest), 0644); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) {
		t.Helper()
		cmd := exec.CommandContext(t.Context(), "go", args...)
		cmd.Dir = dir
		cmd.Env = os.Environ()
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("go %v: %v\n%s", args, err, out)
		}
	}
	run("generate", "./...")
	graph, err := os.ReadFile(filepath.Join(dir, "authored/workflow_gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(graph), `Name: "maintenance-probe"`) {
		t.Fatalf("graph did not follow source edit: %s", graph)
	}
	run("test", ".", "-run", "^TestDelivery$", "-count=1")
	// Go's module boundary rejects private Gimbal imports because this module has
	// a different import prefix. Keep consumer source honest too (tests included).
	err = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".go") {
			b, e := os.ReadFile(path)
			if e != nil {
				return e
			}
			if strings.Contains(string(b), "github.com/tylergannon/gimbal/"+"internal/") {
				t.Errorf("private Gimbal import: %s", path)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
