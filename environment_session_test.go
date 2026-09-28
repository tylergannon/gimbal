package gimbal

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
)

type sessionTestBackend struct {
	environments map[string]*sessionTestEnvironment
	resolveErr   error
}

func (b *sessionTestBackend) Resolve(_ context.Context, name string) (ExecutionEnvironment, error) {
	if b.resolveErr != nil {
		return nil, b.resolveErr
	}
	return b.environments[name], nil
}
func (*sessionTestBackend) Close() error { return nil }

type sessionTestEnvironment struct {
	adapter HarnessAdapter
	roles   []WorkflowRole
}

func (e *sessionTestEnvironment) Harness(role WorkflowRole) HarnessAdapter {
	e.roles = append(e.roles, role)
	return e.adapter
}
func (*sessionTestEnvironment) Start(context.Context, ExecutionCommand, io.Writer, io.Writer) (ExecutionProcess, error) {
	return nil, errors.New("not used")
}

type sessionTestHarness struct {
	*fake
	created []struct {
		model, effort, workdir string
	}
	forks int
}

func (h *sessionTestHarness) CreateSession(_ context.Context, model, effort, workdir string) (string, error) {
	h.created = append(h.created, struct {
		model, effort, workdir string
	}{model, effort, workdir})
	return h.fake.CreateSession(context.Background(), model, effort, workdir)
}

func (h *sessionTestHarness) Fork(ctx context.Context, sessionID string) (string, error) {
	h.forks++
	return h.fake.Fork(ctx, sessionID)
}

func newSessionTestHarness() *sessionTestHarness {
	return &sessionTestHarness{fake: &fake{answer: func(context.Context, string, string, json.RawMessage, func(AgentEvent) error) (string, error) {
		return "ok", nil
	}}}
}

func TestNewSessionUsesEnvironmentHarnessAndForkKeepsBinding(t *testing.T) {
	project, workdir := t.TempDir(), t.TempDir()
	static := newSessionTestHarness()
	environmentHarness := newSessionTestHarness()
	environment := &sessionTestEnvironment{adapter: environmentHarness}
	backend := &sessionTestBackend{environments: map[string]*sessionTestEnvironment{"docker": environment}}
	models := bind(static, "static-model", "planner")
	models["planner"] = ModelBinding{Adapter: static, Model: "environment-model", Effort: "high"}
	var fork *Session
	err := Run(Project(t.Context(), project), "environment-session", models, func(ctx context.Context) error {
		ctx = InEnvironment(ctx, "docker")
		planner := NewSession(ctx, "planner", workdir)
		if _, err := planner.Generate[Text](ctx, "first turn"); err != nil {
			return err
		}
		var err error
		fork, err = planner.Fork(ctx, "coder")
		if err != nil {
			return err
		}
		_, err = fork.Generate[Text](ctx, "fork turn")
		return err
	}, WithExecution(backend))
	if err != nil {
		t.Fatal(err)
	}
	if len(environment.roles) != 1 || environment.roles[0] != "planner" {
		t.Fatalf("environment harness roles=%v, want [planner] (fork should retain its parent's adapter)", environment.roles)
	}
	if len(environmentHarness.created) != 1 || environmentHarness.forks != 1 || len(static.created) != 0 {
		t.Fatalf("environment adapter calls: creates=%d forks=%d static creates=%d, want creates=1 forks=1 static creates=0", len(environmentHarness.created), environmentHarness.forks, len(static.created))
	}
	for _, created := range environmentHarness.created {
		if created.model != "environment-model" || created.effort != "high" || created.workdir != workdir {
			t.Fatalf("environment session binding=%+v", created)
		}
	}
	if fork == nil || fork.environment != "docker" || fork.workdir != workdir || fork.adapter != environmentHarness {
		t.Fatalf("fork binding=%+v; want docker environment, shared workdir, and environment adapter", fork)
	}
}

func TestNewSessionFailsClearlyWhenEnvironmentCannotSupplyHarness(t *testing.T) {
	tests := []struct {
		name    string
		backend ExecutionBackend
		want    string
	}{
		{name: "no backend", want: `environment "docker" selected without an execution backend`},
		{name: "nil role harness", backend: &sessionTestBackend{environments: map[string]*sessionTestEnvironment{"docker": {}}}, want: `role "planner": environment "docker" has no compatible harness adapter`},
		{name: "resolve failure", backend: &sessionTestBackend{resolveErr: errors.New("worker unavailable")}, want: `resolve environment "docker": worker unavailable`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var sessionErr error
			err := Run(Project(t.Context(), t.TempDir()), "environment-session-failure", bind(newSessionTestHarness(), "model", "planner"), func(ctx context.Context) error {
				ctx = InEnvironment(ctx, "docker")
				session := NewSession(ctx, "planner", t.TempDir())
				_, sessionErr = session.Generate[Text](ctx, "turn")
				return nil
			}, WithExecution(test.backend))
			if err != nil {
				t.Fatal(err)
			}
			if sessionErr == nil || !strings.Contains(sessionErr.Error(), test.want) {
				t.Fatalf("Generate error=%v, want operational error containing %q", sessionErr, test.want)
			}
		})
	}
}

func TestSupervisorMustShareEnvironmentAndWorkdir(t *testing.T) {
	for _, test := range []struct {
		name          string
		supervisorEnv string
		supervisorDir string
	}{
		{name: "no environment", supervisorDir: "workspace"},
		{name: "different environment", supervisorEnv: "other", supervisorDir: "workspace"},
		{name: "different workdir", supervisorEnv: "docker", supervisorDir: "other-workspace"},
	} {
		t.Run(test.name, func(t *testing.T) {
			adapter := newSessionTestHarness()
			backend := &sessionTestBackend{environments: map[string]*sessionTestEnvironment{
				"docker": {adapter: adapter}, "other": {adapter: adapter},
			}}
			models := bind(adapter, "model", "worker", "reviewer")
			var gotErr error
			err := Run(Project(t.Context(), t.TempDir()), "supervisor-binding", models, func(ctx context.Context) error {
				workerCtx := InEnvironment(ctx, "docker")
				worker := NewSession(workerCtx, "worker", "workspace")
				supervisorCtx := ctx
				if test.supervisorEnv != "" {
					supervisorCtx = InEnvironment(ctx, test.supervisorEnv)
				}
				supervisor := NewSession(supervisorCtx, "reviewer", test.supervisorDir)
				_, gotErr = worker.Generate[Text](workerCtx, "work", WithSupervisor(supervisor, "watch"))
				return nil
			}, WithExecution(backend))
			if err != nil {
				t.Fatal(err)
			}
			if gotErr == nil || !strings.Contains(gotErr.Error(), `supervisor role "reviewer"`) || !strings.Contains(gotErr.Error(), `session role "worker"`) || !strings.Contains(gotErr.Error(), `environment "docker"`) {
				t.Fatalf("supervisor binding error=%v, want named supervisor, worker, and environment", gotErr)
			}
			if len(adapter.created) != 0 {
				t.Fatalf("adapter created %d sessions before incompatible binding was rejected", len(adapter.created))
			}
		})
	}
}
