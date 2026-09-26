package piport

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/tylergannon/gimbal"
)

func TestPortIntegratesIsolatedWorkers(t *testing.T) {
	repo := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %s: %v", args, out, err)
		}
		return string(out)
	}
	git("init")
	git("config", "user.name", "Workflow Test")
	git("config", "user.email", "workflow@example.invalid")
	if err := os.WriteFile(filepath.Join(repo, "go.mod"), []byte("module example.org/porttest\n\ngo 1.26\n"), 0600); err != nil {
		t.Fatal(err)
	}
	git("add", "go.mod")
	git("commit", "-m", "baseline")
	inputs := t.TempDir()
	handoff := filepath.Join(inputs, "task.md")
	if err := os.WriteFile(handoff, []byte("Port assigned package"), 0600); err != nil {
		t.Fatal(err)
	}
	tasks := []*module{{ID: "model", Handoff: handoff, Paths: []string{"internal/pi/model"}, Check: []string{"go", "test", "./internal/pi/model"}}, {ID: "files", Handoff: handoff, Paths: []string{"internal/pi/files"}, Check: []string{"go", "test", "./internal/pi/files"}}}
	raw, _ := json.Marshal([]wave{{Modules: tasks}})
	manifest := filepath.Join(inputs, "waves.json")
	if err := os.WriteFile(manifest, raw, 0600); err != nil {
		t.Fatal(err)
	}
	adapter := &fakeAdapter{sessions: map[string]string{}}
	bindings := map[gimbal.WorkflowRole]gimbal.ModelBinding{roleCoding: {Adapter: adapter, Model: "worker"}, roleReview: {Adapter: adapter, Model: "reviewer"}, roleScope: {Adapter: adapter, Model: "coach"}}
	err := gimbal.Run(gimbal.Project(context.Background(), t.TempDir()), "pi-port", bindings, func(ctx context.Context) error {
		return Port(ctx, gimbal.Env{WorkDir: repo}, Params{AssignmentsFile: manifest, ScratchDir: t.TempDir()})
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"model", "files"} {
		if _, err := os.Stat(filepath.Join(repo, "internal/pi", name, "port.go")); err != nil {
			t.Fatal(err)
		}
	}
	if got := strings.TrimSpace(git("status", "--porcelain")); got != "" {
		t.Fatalf("dirty integration: %s", got)
	}
	if got := strings.Count(strings.TrimSpace(git("log", "--format=%s")), "Port Pi module:"); got != 2 {
		t.Fatalf("integrated %d commits", got)
	}
}

func TestOwnershipRejectsOtherPackages(t *testing.T) {
	if err := owned("internal/pi/model/types.go\x00internal/pi/files/file.go\x00", []string{"internal/pi/model"}); err == nil {
		t.Fatal("accepted another package")
	}
	if err := owned("internal/pi/model/types.go\x00", []string{"internal/pi/model"}); err != nil {
		t.Fatal(err)
	}
	if err := owned("", []string{"internal/pi/model"}); err == nil {
		t.Fatal("accepted empty result")
	}
}

func TestGeneratedGraphReadable(t *testing.T) {
	if len(Graph.Diagnostics) != 0 {
		t.Fatalf("graph diagnostics: %+v", Graph.Diagnostics)
	}
}

type fakeAdapter struct {
	mu       sync.Mutex
	sessions map[string]string
}

func (f *fakeAdapter) CreateSession(_ context.Context, model, _, dir string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := model + ":" + dir
	f.sessions[id] = dir
	return id, nil
}
func (f *fakeAdapter) RunTurn(_ context.Context, id, _ string, schema json.RawMessage, _ func(gimbal.AgentEvent) error) (gimbal.TurnResult, error) {
	if len(schema) > 0 {
		return gimbal.TurnResult{Output: json.RawMessage(`{"complete":true,"findings":[]}`)}, nil
	}
	f.mu.Lock()
	dir := f.sessions[id]
	f.mu.Unlock()
	name := filepath.Base(dir)
	pkg := filepath.Join(dir, "internal/pi", name)
	if err := os.MkdirAll(pkg, 0755); err != nil {
		return gimbal.TurnResult{}, err
	}
	err := os.WriteFile(filepath.Join(pkg, "port.go"), []byte("package "+name+"\nfunc Value() int { return 1 }\n"), 0600)
	return gimbal.TurnResult{Output: json.RawMessage(`"implemented"`)}, err
}
func (f *fakeAdapter) Steer(context.Context, string, string) (bool, error) { return false, nil }
func (f *fakeAdapter) Fork(context.Context, string) (string, error)        { panic("unused") }
func (f *fakeAdapter) Close(context.Context, string) error                 { return nil }
