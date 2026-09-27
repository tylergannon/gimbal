package host_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/internal/execution"
	"github.com/tylergannon/gimbal/internal/runlog"
	"github.com/tylergannon/gimbal/internal/workflows/implementation"
	"github.com/tylergannon/gimbal/web"
)

func TestHostedBuiltInUsesConfiguredEnvironmentAndSelectedWorkdir(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	projectDir := t.TempDir()
	workdir := t.TempDir()
	if err := os.WriteFile(filepath.Join(workdir, "outcomes.json"), []byte(`["run the selected implementation outcome"]`), 0o644); err != nil {
		t.Fatal(err)
	}
	instance, err := web.NewInstance(ctx, filepath.Join(projectDir, ".gimbal"), []string{projectDir}, web.WithNoWeb(), web.WithExecutionBackend(execution.Config{Environment: "named-test-environment"}))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		cancel()
		instance.Wait()
	}()
	project, err := instance.Owner.AdmitProject(projectDir)
	if err != nil {
		t.Fatal(err)
	}
	harness := &hostedImplementationHarness{}
	backend := newHostedFakeBackend(harness)
	var factoryWorktree, factoryWorkdir, factoryArtifacts string
	var factoryModels map[gimbal.WorkflowRole]gimbal.ModelBinding
	instance.Owner.SetExecutionBackendFactory(func(_ context.Context, worktree, selectedWorkdir, artifacts string, models map[gimbal.WorkflowRole]gimbal.ModelBinding) (gimbal.ExecutionBackend, string, error) {
		factoryWorktree, factoryWorkdir, factoryArtifacts, factoryModels = worktree, selectedWorkdir, artifacts, models
		return backend, "named-test-environment", nil
	})
	models := map[gimbal.WorkflowRole]gimbal.ModelBinding{
		gimbal.RoleSprintPlanning:        {Adapter: harness, Harness: "codex", Model: "planner-model", Effort: "high"},
		"coding":                         {Adapter: harness, Harness: "codex", Model: "coder-model", Effort: "medium"},
		gimbal.RoleArchitecturalCritique: {Adapter: harness, Harness: "codex", Model: "coach-model", Effort: "low"},
		gimbal.RoleQAOrchestration:       {Adapter: harness, Harness: "codex", Model: "validator-model", Effort: "high"},
	}
	runID, err := project.Start("implement", workdir, "", models, func(runCtx context.Context) error {
		return implementation.Implement(runCtx, gimbal.Env{WorkDir: workdir}, implementation.Params{OutcomesFile: "outcomes.json", MaxTasksPerOutcome: 1})
	})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-backend.closed:
	case <-time.After(5 * time.Second):
		t.Fatal("hosted run did not close its backend")
	}
	if factoryWorktree != project.Path() || factoryWorkdir != workdir || factoryArtifacts != project.Dir() {
		t.Fatalf("factory paths = project %q, workdir %q, artifacts %q; want %q, %q, %q", factoryWorktree, factoryWorkdir, factoryArtifacts, project.Path(), workdir, project.Dir())
	}
	if backend.environmentName != "named-test-environment" {
		t.Fatalf("resolved environment = %q, want named-test-environment", backend.environmentName)
	}
	if !backend.commandStarted || backend.commandWorkdir != workdir {
		t.Fatalf("remote command started=%v workdir=%q, want true and %q", backend.commandStarted, backend.commandWorkdir, workdir)
	}
	wantModels := map[gimbal.WorkflowRole]string{
		gimbal.RoleSprintPlanning: "planner-model", "coding": "coder-model", gimbal.RoleQAOrchestration: "validator-model",
	}
	for role, model := range wantModels {
		if factoryModels[role].Model != model || harness.modelFor(role) != model {
			t.Errorf("role %q model in factory/session = %q/%q, want %q", role, factoryModels[role].Model, harness.modelFor(role), model)
		}
	}
	if !harness.plannerTurn || !harness.coderTurn || !harness.validatorTurn {
		t.Fatalf("built-in turns planner/coder/validator = %v/%v/%v", harness.plannerTurn, harness.coderTurn, harness.validatorTurn)
	}
	runDir := filepath.Join(project.Dir(), "runs", runID)
	var ended bool
	if err := runlog.Read[gimbal.LifecycleRecord](context.Background(), runDir, func(record gimbal.LifecycleRecord) error {
		if event, ok := record.Event.(gimbal.RunEnded); ok {
			ended = true
			if event.Error != "" {
				return errors.New(event.Error)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !ended {
		t.Fatal("compiled implementation workflow did not record RunEnded")
	}
}

type hostedFakeBackend struct {
	env             *hostedFakeEnvironment
	closed          chan struct{}
	environmentName string
	commandStarted  bool
	commandWorkdir  string
	mu              sync.Mutex
}

func newHostedFakeBackend(harness *hostedImplementationHarness) *hostedFakeBackend {
	backend := &hostedFakeBackend{closed: make(chan struct{})}
	backend.env = &hostedFakeEnvironment{backend: backend, harness: harness}
	return backend
}

func (b *hostedFakeBackend) Resolve(_ context.Context, name string) (gimbal.ExecutionEnvironment, error) {
	b.environmentName = name
	return b.env, nil
}
func (b *hostedFakeBackend) Close() error { close(b.closed); return nil }
func (b *hostedFakeBackend) start(command gimbal.ExecutionCommand) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.commandStarted, b.commandWorkdir = true, command.Workdir
}

type hostedFakeEnvironment struct {
	backend *hostedFakeBackend
	harness *hostedImplementationHarness
}

func (e *hostedFakeEnvironment) Harness(role gimbal.WorkflowRole) gimbal.HarnessAdapter {
	return hostedRoleHarness{harness: e.harness, role: role}
}
func (e *hostedFakeEnvironment) Start(ctx context.Context, command gimbal.ExecutionCommand, stdout, stderr io.Writer) (gimbal.ExecutionProcess, error) {
	e.backend.start(command)
	cmd := exec.CommandContext(ctx, command.Command, command.Args...)
	cmd.Dir, cmd.Stdout, cmd.Stderr = command.Workdir, stdout, stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return hostedFakeProcess{cmd: cmd}, nil
}

type hostedFakeProcess struct{ cmd *exec.Cmd }

func (p hostedFakeProcess) Workdir() string { return p.cmd.Dir }
func (p hostedFakeProcess) Wait() (int, error) {
	err := p.cmd.Wait()
	if err == nil {
		return 0, nil
	}
	if exit, ok := errors.AsType[*exec.ExitError](err); ok {
		return exit.ExitCode(), nil
	}
	return -1, err
}
func (p hostedFakeProcess) Stop() error {
	if p.cmd.Process == nil {
		return nil
	}
	if err := p.cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	return nil
}

type hostedImplementationHarness struct {
	mu                                    sync.Mutex
	models                                map[gimbal.WorkflowRole]string
	plannerTurn, coderTurn, validatorTurn bool
}

func (*hostedImplementationHarness) CreateSession(context.Context, string, string, string) (string, error) {
	return "unused", nil
}
func (*hostedImplementationHarness) Fork(context.Context, string) (string, error) { return "fork", nil }
func (*hostedImplementationHarness) Steer(context.Context, string, string) (bool, error) {
	return true, nil
}
func (*hostedImplementationHarness) Close(context.Context, string) error { return nil }

type hostedRoleHarness struct {
	harness *hostedImplementationHarness
	role    gimbal.WorkflowRole
}

func (r hostedRoleHarness) CreateSession(ctx context.Context, model, effort, workdir string) (string, error) {
	r.harness.mu.Lock()
	defer r.harness.mu.Unlock()
	if r.harness.models == nil {
		r.harness.models = make(map[gimbal.WorkflowRole]string)
	}
	r.harness.models[r.role] = model
	return model + ":" + effort + ":" + workdir, nil
}
func (hostedRoleHarness) Fork(context.Context, string) (string, error)        { return "fork", nil }
func (hostedRoleHarness) Steer(context.Context, string, string) (bool, error) { return true, nil }
func (hostedRoleHarness) Close(context.Context, string) error                 { return nil }
func (r hostedRoleHarness) RunTurn(ctx context.Context, session, prompt string, schema json.RawMessage, emit func(gimbal.AgentEvent) error) (gimbal.TurnResult, error) {
	return r.harness.runTurn(ctx, r.role, session, prompt, schema, emit)
}

func (h *hostedImplementationHarness) RunTurn(ctx context.Context, session, prompt string, schema json.RawMessage, emit func(gimbal.AgentEvent) error) (gimbal.TurnResult, error) {
	return h.runTurn(ctx, "", session, prompt, schema, emit)
}

func (h *hostedImplementationHarness) runTurn(_ context.Context, _ gimbal.WorkflowRole, _ string, prompt string, _ json.RawMessage, _ func(gimbal.AgentEvent) error) (gimbal.TurnResult, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	switch {
	case strings.Contains(prompt, "You plan the loop"):
		h.plannerTurn = true
		return gimbal.TurnResult{Output: json.RawMessage(`{"tasks":[{"name":"implement","description":"implement the outcome","definition_of_done":"the command and agent turn ran","validation":{"command":"printf backend-command","query":"Observe that it worked"}}],"next":0}`)}, nil
	case strings.Contains(prompt, "Independently validate the selected task and current outcome"):
		h.validatorTurn = true
		return gimbal.TurnResult{Output: json.RawMessage(`{"validation_passed":true,"observed":"the built-in ran","substantial_gaps":[],"small_gaps":[]}`)}, nil
	case strings.Contains(prompt, "Read the selected outcome and task"):
		h.coderTurn = true
		return gimbal.TurnResult{Output: json.RawMessage(`"implemented"`)}, nil
	default:
		return gimbal.TurnResult{Output: json.RawMessage(`"no objection"`)}, nil
	}
}
func (h *hostedImplementationHarness) modelFor(role gimbal.WorkflowRole) string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.models[role]
}
