package gimbal

import (
	"context"
	"io"
	"strings"
	"testing"
)

type commandTestBackend struct {
	env    *commandTestEnvironment
	closed bool
}

func (b *commandTestBackend) Resolve(context.Context, string) (ExecutionEnvironment, error) {
	return b.env, nil
}
func (b *commandTestBackend) Close() error { b.closed = true; return nil }

type commandTestEnvironment struct {
	command ExecutionCommand
	code    int
	stop    int
	output  string
}

func (e *commandTestEnvironment) Harness(WorkflowRole) HarnessAdapter { return nil }
func (e *commandTestEnvironment) Start(_ context.Context, command ExecutionCommand, stdout, stderr io.Writer) (ExecutionProcess, error) {
	e.command = command
	output := e.output
	if output == "" {
		output = "remote out"
	}
	_, _ = io.WriteString(stdout, output)
	_, _ = io.WriteString(stderr, "remote err")
	return &commandTestProcess{env: e, workdir: command.Workdir}, nil
}

type commandTestProcess struct {
	env     *commandTestEnvironment
	workdir string
}

func (p *commandTestProcess) Workdir() string    { return p.workdir }
func (p *commandTestProcess) Wait() (int, error) { return p.env.code, nil }
func (p *commandTestProcess) Stop() error        { p.env.stop++; return nil }

func TestRunCommandDispatchesThroughSelectedExecutionEnvironment(t *testing.T) {
	project, workdir := t.TempDir(), t.TempDir()
	env := &commandTestEnvironment{code: 7}
	backend := &commandTestBackend{env: env}
	var code int
	var stdout, stderr string
	var commandErr error
	runErr := Run(Project(t.Context(), project), "execution", nil, func(ctx context.Context) error {
		ctx = InEnvironment(ctx, "named")
		code, stdout, stderr, commandErr = RunCommand(ctx, "mutate", workdir, "sh", "-c", "exit 7")
		return nil
	}, WithExecution(backend))
	if runErr != nil {
		t.Fatal(runErr)
	}
	if code != 7 || stdout != "remote out" || stderr != "remote err" || commandErr != nil {
		t.Fatalf("remote RunCommand=(%d,%q,%q,%v)", code, stdout, stderr, commandErr)
	}
	if env.command.Command != "sh" || !strings.HasSuffix(env.command.Operation, "/mutate.1") || env.command.Role != "." || env.command.Workdir != workdir || env.stop != 1 {
		t.Fatalf("execution request/process cleanup = %+v, stops=%d", env.command, env.stop)
	}
	if !backend.closed {
		t.Fatal("Run did not close its execution backend")
	}
}

func TestRunClosesExecutionBackendWhenProjectSetupFails(t *testing.T) {
	backend := &commandTestBackend{env: &commandTestEnvironment{}}
	err := Run(context.Background(), "missing-project", nil, func(context.Context) error { return nil }, WithExecution(backend))
	if err == nil || !strings.Contains(err.Error(), "gimbal.Project") {
		t.Fatalf("Run error = %v, want missing project setup failure", err)
	}
	if !backend.closed {
		t.Fatal("Run did not close its backend after setup failed before options were previously applied")
	}
}

func TestRemoteCaptureUsesGimbalOutputLimit(t *testing.T) {
	project, workdir := t.TempDir(), t.TempDir()
	env := &commandTestEnvironment{code: 0, output: strings.Repeat("x", commandOutputLimit*3)}
	backend := &commandTestBackend{env: env}
	var stdout string
	if err := Run(Project(t.Context(), project), "large", nil, func(ctx context.Context) error {
		ctx = InEnvironment(ctx, "named")
		_, stdout, _, _ = RunCommand(ctx, "large-output", workdir, "ignored")
		return nil
	}, WithExecution(backend)); err != nil {
		t.Fatal(err)
	}
	if len(stdout) > commandOutputLimit+256 {
		t.Fatalf("captured output was not bounded: %d bytes", len(stdout))
	}
}
