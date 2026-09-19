package gimble

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckRecordsRepeatedResultsForTheNextTurn(t *testing.T) {
	workdir := t.TempDir()
	var prompt string
	var dir string
	adapter := &fake{answer: func(_ context.Context, _ string, got string, _ json.RawMessage, _ func(AgentEvent) error) (string, error) {
		prompt = got
		return "assessed", nil
	}}
	err := runTest(t, bind(adapter, "model", "evaluator"), func(ctx context.Context) error {
		dir = runDir(ctx)
		if err := Check(ctx, "tests.1", workdir, "sh", "-c", "printf failed-out; printf failed-err >&2; exit 1"); err != nil {
			return err
		}
		if err := Check(ctx, "tests.2", workdir, "sh", "-c", "printf passed-out"); err != nil {
			return err
		}
		scope, _ := current(ctx)
		if err := checkKeyAvailable(scope, "tests.2"); err == nil {
			t.Error("a duplicate Check key passed the runtime preflight")
		}
		_, err := NewSession(ctx, "evaluator", workdir).Generate[Text](ctx, "assess the checks")
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"assess the checks",
		"## tests.1",
		`"exit_code": 1`,
		`"stdout": "failed-out"`,
		`"stderr": "failed-err"`,
		"## tests.2",
		`"exit_code": 0`,
		`"stdout": "passed-out"`,
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("evaluator prompt lacks %q:\n%s", want, prompt)
		}
	}
	if strings.Index(prompt, "## tests.1") > strings.Index(prompt, "## tests.2") {
		t.Error("the failed observation does not precede the successful rerun")
	}
	if rows := commandRows(t, dir); len(rows) != 2 {
		t.Fatalf("duplicate-key preflight ran %d commands, want only the two uniquely named checks", len(rows))
	}
}

func TestCheckReturnsExecutionErrorsAndRecordsThemAsErrors(t *testing.T) {
	var checkErr error
	var recorded ScopeValue
	err := runTest(t, nil, func(ctx context.Context) error {
		checkErr = Check(ctx, "missing", t.TempDir(), "gimble-no-such-command")
		recorded = scopeData(ctx).By["missing"]
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !errors.Is(checkErr, exec.ErrNotFound) {
		t.Fatalf("Check missing executable = %v, want exec.ErrNotFound", checkErr)
	}
	result, ok := recorded.Value.(map[string]any)
	if !ok {
		t.Fatalf("recorded missing check = %#v", recorded.Value)
	}
	if result["exit_code"] != float64(-1) || result["error"] == "" {
		t.Fatalf("recorded missing check = %#v, want exit -1 with an execution error", result)
	}
}

func TestCheckKeepsCompleteLargeOutputReachable(t *testing.T) {
	var stdout string
	err := runTest(t, nil, func(ctx context.Context) error {
		if err := Check(ctx, "large", "", "sh", "-c", "head -c 70000 /dev/zero | tr '\\0' x; printf end"); err != nil {
			return err
		}
		result := scopeData(ctx).By["large"].Value.(map[string]any)
		stdout, _ = result["stdout"].(string)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	const marker = "Complete output: "
	start := strings.Index(stdout, marker)
	if start < 0 || !strings.HasSuffix(stdout, "xend") {
		t.Fatalf("recorded stdout lacks its bounded preview and complete-output reference: %q", stdout[max(0, len(stdout)-300):])
	}
	path := strings.TrimSpace(strings.SplitN(stdout[start+len(marker):], "\n", 2)[0])
	whole, err := os.ReadFile(path)
	if err != nil || len(whole) != 70003 || !strings.HasSuffix(string(whole), "xend") {
		t.Fatalf("complete output %s has %d bytes (%v), want the exact 70003-byte stream", path, len(whole), err)
	}
}

func TestCheckReturnsAResultRecordingFailure(t *testing.T) {
	var checkErr error
	err := runTest(t, nil, func(ctx context.Context) error {
		blocked := filepath.Join(runDir(ctx), "artifacts", "values", "root")
		if err := os.MkdirAll(filepath.Dir(blocked), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(blocked, []byte("not a directory"), 0o644); err != nil {
			return err
		}
		checkErr = Check(ctx, "large", "", "sh", "-c", "head -c 70000 /dev/zero | tr '\\0' x")
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if checkErr == nil || !strings.Contains(checkErr.Error(), "record result") {
		t.Fatalf("Check recording failure = %v, want a result recording error", checkErr)
	}
}

func TestCheckResultReachesThePromiseLoopPlanner(t *testing.T) {
	var prompts []string
	adapter := &fake{answer: func(_ context.Context, _ string, prompt string, _ json.RawMessage, _ func(AgentEvent) error) (string, error) {
		prompts = append(prompts, prompt)
		if len(prompts) == 1 {
			return `{"tasks":[{"name":"one","description":"do it","definition_of_done":"done","validation":{"command":"","query":""}}],"next":0}`, nil
		}
		return `{"tasks":[],"next":null}`, nil
	}}
	err := runTest(t, bind(adapter, "model", "planner"), func(ctx context.Context) error {
		planner := NewSession(ctx, "planner", t.TempDir())
		loop := PromiseLoop(ctx, "work", "finish", planner)
		loop.Tasks(func(taskCtx context.Context, _ Task) bool {
			if err := Check(taskCtx, "tests.1", "", "sh", "-c", "printf task-out; printf task-err >&2; exit 1"); err != nil {
				t.Errorf("Check nonzero exit = %v, want evidence without an error", err)
				return false
			}
			return true
		})
		return loop.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(prompts) != 2 {
		t.Fatalf("planner got %d prompts, want its initial and post-task decisions", len(prompts))
	}
	for _, want := range []string{"Previous task record", "## tests.1", `"exit_code": 1`, "task-out", "task-err"} {
		if !strings.Contains(prompts[1], want) {
			t.Errorf("planner's second prompt lacks %q:\n%s", want, prompts[1])
		}
	}
}
