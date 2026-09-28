package execution

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/codex"
	"github.com/tylergannon/gimbal/internal/runlog"
)

func TestResolveWorkerReadinessIntegration(t *testing.T) {
	if os.Getenv("GIMBAL_EXECUTION_INTEGRATION") != "1" {
		t.Skip("set GIMBAL_EXECUTION_INTEGRATION=1 to use local Docker, Postgres, and Temporal")
	}
	postgresDSN := requiredEnv(t, "GIMBAL_TEST_POSTGRES_DSN")
	temporalAddress := requiredEnv(t, "GIMBAL_TEST_TEMPORAL_ADDRESS")
	image := requiredEnv(t, "GIMBAL_TEST_WORKER_IMAGE")
	workerBinary := requiredEnv(t, "GIMBAL_TEST_WORKER_BINARY")
	name := "readiness-" + uuid.NewString()
	workerDSN := "postgres://gimbal:gimbal@host.docker.internal:65534/gimbal?sslmode=disable&connect_timeout=2"

	badConfig := Config{Environment: "readiness-bad", DockerImage: image, WorkerBinary: workerBinary, TemporalAddress: temporalAddress, PostgresDSN: postgresDSN, WorkerPostgresDSN: workerDSN, Mounts: []string{t.TempDir()}}
	badBackend, err := New(context.Background(), badConfig)
	if err != nil {
		t.Fatal(err)
	}
	scopedName, _, failedContainer, _ := environmentIdentity(badBackend.owner, name)
	started := time.Now()
	_, resolveErr := badBackend.Resolve(context.Background(), name)
	if resolveErr == nil || !strings.Contains(resolveErr.Error(), name) || !strings.Contains(resolveErr.Error(), "65534") {
		_ = badBackend.Close()
		t.Fatalf("Resolve with bad worker DSN error=%v, want environment and Postgres startup cause", resolveErr)
	}
	if elapsed := time.Since(started); elapsed >= workerReadyTimeout {
		_ = badBackend.Close()
		t.Fatalf("bad worker resolution took %s, want failure within %s", elapsed, workerReadyTimeout)
	}
	if err := badBackend.Close(); err != nil {
		t.Fatal(err)
	}

	checkCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	db, err := pgxpool.New(checkCtx, postgresDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var rows int
	if err := db.QueryRow(checkCtx, `SELECT count(*) FROM gimbal_environments WHERE name=$1`, scopedName).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 0 {
		t.Fatalf("failed resolution left %d bootstrap rows for %q", rows, name)
	}
	inspect := exec.CommandContext(checkCtx, "docker", "inspect", failedContainer)
	if output, err := inspect.CombinedOutput(); err == nil {
		t.Fatalf("failed worker container %q still exists: %s", failedContainer, output)
	}

	goodDir := t.TempDir()
	goodConfig := Config{Environment: "readiness-good", DockerImage: image, WorkerBinary: workerBinary, TemporalAddress: temporalAddress, PostgresDSN: postgresDSN,
		WorkerPostgresDSN: "postgres://gimbal:gimbal@host.docker.internal:5433/gimbal?sslmode=disable",
		Mounts:            []string{goodDir}}
	goodBackend, err := New(context.Background(), goodConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = goodBackend.Close() }()
	ctx, cancelWait := context.WithTimeout(gimbal.Project(context.Background(), goodDir), 15*time.Second)
	defer cancelWait()
	var commandCode int
	var stdout, stderr string
	err = gimbal.Run(ctx, "worker-readiness-integration", nil, func(runCtx context.Context) error {
		runCtx = gimbal.InEnvironment(runCtx, name)
		var commandErr error
		commandCode, stdout, stderr, commandErr = gimbal.RunCommand(runCtx, "readiness-command", goodDir, "sh", "-c", "printf ready")
		return commandErr
	}, gimbal.WithExecution(goodBackend))
	if err != nil || commandCode != 0 || stdout != "ready" || stderr != "" {
		t.Fatalf("RunCommand result=(%d,%q,%q,%v), want (0,ready,empty,nil)", commandCode, stdout, stderr, err)
	}
}

func TestCodexSessionLifecycleIntegration(t *testing.T) {
	if os.Getenv("GIMBAL_EXECUTION_INTEGRATION") != "1" {
		t.Skip("set GIMBAL_EXECUTION_INTEGRATION=1 to use local Docker, Postgres, and Temporal")
	}
	keyFile := os.Getenv("GIMBAL_TEST_OPENAI_API_KEY_FILE")
	if keyFile == "" {
		t.Skip("set GIMBAL_TEST_OPENAI_API_KEY_FILE to an operator-provided Codex API-key file to run this lifecycle test")
	}
	postgresDSN := requiredEnv(t, "GIMBAL_TEST_POSTGRES_DSN")
	temporalAddress := requiredEnv(t, "GIMBAL_TEST_TEMPORAL_ADDRESS")
	image := requiredEnv(t, "GIMBAL_TEST_WORKER_IMAGE")
	workerBinary := requiredEnv(t, "GIMBAL_TEST_WORKER_BINARY")
	workdir := t.TempDir()
	secretFiles := map[string]string{"OPENAI_API_KEY": keyFile}
	backend, err := New(context.Background(), Config{
		Environment: "codex-lifecycle", DockerImage: image, WorkerBinary: workerBinary, TemporalAddress: temporalAddress, PostgresDSN: postgresDSN,
		WorkerPostgresDSN: "postgres://gimbal:gimbal@host.docker.internal:5433/gimbal?sslmode=disable",
		Mounts:            []string{workdir}, Models: map[gimbal.WorkflowRole]gimbal.ModelBinding{
			"coder": {Adapter: codex.New(), Harness: "codex", Model: "gpt-5.6-luna", Effort: "low"},
		}, SecretFiles: secretFiles,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := backend.Close(); err != nil {
			t.Errorf("close backend: %v", err)
		}
	}()
	envName := "codex-lifecycle-" + uuid.NewString()
	environment, err := backend.Resolve(context.Background(), envName)
	if err != nil {
		t.Fatal(err)
	}
	adapter := environment.Harness("coder")
	if adapter == nil {
		t.Fatal("environment did not supply its configured Codex adapter")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	session, err := adapter.CreateSession(ctx, "gpt-5.6-luna", "low", workdir)
	if err != nil {
		t.Fatalf("stock Codex CreateSession through Temporal proxy: %v", err)
	}
	if session == "" {
		t.Fatal("Codex CreateSession returned an empty native thread ID")
	}
	if err := adapter.Close(ctx, session); err != nil {
		t.Fatalf("stock Codex Close through Temporal proxy: %v", err)
	}
	var stored int
	if err := backend.db.QueryRow(ctx, `SELECT count(*) FROM gimbal_harness_events WHERE environment_name=$1`, environment.(*Environment).bootstrap.Name).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored < 2 {
		t.Fatalf("Postgres recorded %d session lifecycle events, want create and close", stored)
	}
	t.Logf("worker %s created and closed Codex thread %s; Postgres retained %d lifecycle rows; operator API-key file configured=%t", environment.(*Environment).bootstrap.Container, session, stored, len(secretFiles) != 0)
}

// TestCommandCancellationIntegration cancels a running command through real
// Temporal, then checks that the worker ended it and that the environment
// still runs the next command. It needs no model credentials.
func TestCommandCancellationIntegration(t *testing.T) {
	if os.Getenv("GIMBAL_EXECUTION_INTEGRATION") != "1" {
		t.Skip("set GIMBAL_EXECUTION_INTEGRATION=1 to use local Docker, Postgres, and Temporal")
	}
	workdir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	backend, err := New(context.Background(), Config{
		Environment: "command-cancellation", DockerImage: requiredEnv(t, "GIMBAL_TEST_WORKER_IMAGE"), WorkerBinary: requiredEnv(t, "GIMBAL_TEST_WORKER_BINARY"),
		TemporalAddress: requiredEnv(t, "GIMBAL_TEST_TEMPORAL_ADDRESS"), PostgresDSN: requiredEnv(t, "GIMBAL_TEST_POSTGRES_DSN"),
		WorkerPostgresDSN: "postgres://gimbal:gimbal@host.docker.internal:5433/gimbal?sslmode=disable",
		Mounts:            []string{workdir},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := backend.Close(); err != nil {
			t.Errorf("close backend: %v", err)
		}
	}()
	resolved, err := backend.Resolve(context.Background(), "command-cancellation-"+uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	environment := resolved.(*Environment)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Start returns once the worker's heartbeat reports the process started.
	process, err := environment.Start(ctx, gimbal.ExecutionCommand{Operation: uuid.NewString(), Workdir: workdir,
		Command: "sh", Args: []string{"-c", "echo $$ > sleeper.pid; exec sleep 300"}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	began := time.Now()
	_, waitErr := process.Wait()
	if !errors.Is(waitErr, context.Canceled) || environment.removal() != nil {
		t.Fatalf("Wait after cancel = %v (removal %v), want a confirmed cancellation", waitErr, environment.removal())
	}
	t.Logf("cancellation confirmed in %s: %v", time.Since(began).Round(time.Millisecond), waitErr)

	var stdout, stderr strings.Builder
	next, err := environment.Start(context.Background(), gimbal.ExecutionCommand{Operation: uuid.NewString(), Workdir: workdir,
		Command: "sh", Args: []string{"-c", `if kill -0 "$(cat sleeper.pid)" 2>/dev/null; then echo "sleeper still running"; exit 1; fi; printf after`}}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("Start after a confirmed cancellation: %v", err)
	}
	code, err := next.Wait()
	if code != 0 || err != nil || stdout.String() != "after" {
		t.Fatalf("next command = (%d, %q, %q, %v), want (0, after) with the cancelled process gone", code, stdout.String(), stderr.String(), err)
	}
}

// TestSupervisedTurnIntegration runs a principal and its supervisor, both
// Codex gpt-5.6-luna sessions, in one environment and workspace. The
// supervisor must see the principal's live work and steer it before the
// principal finishes. It needs an operator-provided API-key file.
func TestSupervisedTurnIntegration(t *testing.T) {
	if os.Getenv("GIMBAL_EXECUTION_INTEGRATION") != "1" {
		t.Skip("set GIMBAL_EXECUTION_INTEGRATION=1 to use local Docker, Postgres, and Temporal")
	}
	keyFile := os.Getenv("GIMBAL_TEST_OPENAI_API_KEY_FILE")
	if keyFile == "" {
		t.Skip("set GIMBAL_TEST_OPENAI_API_KEY_FILE to an operator-provided Codex API-key file; this test proves nothing without it")
	}
	project, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	workspace := filepath.Join(project, "workspace")
	if err := os.Mkdir(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	adapter := codex.New()
	models := map[gimbal.WorkflowRole]gimbal.ModelBinding{
		"principal":  {Adapter: adapter, Harness: "codex", Model: "gpt-5.6-luna", Effort: "low"},
		"supervisor": {Adapter: adapter, Harness: "codex", Model: "gpt-5.6-luna", Effort: "low"},
	}
	backend, err := New(context.Background(), Config{
		Environment: "supervised-turn", DockerImage: requiredEnv(t, "GIMBAL_TEST_WORKER_IMAGE"), WorkerBinary: requiredEnv(t, "GIMBAL_TEST_WORKER_BINARY"),
		TemporalAddress: requiredEnv(t, "GIMBAL_TEST_TEMPORAL_ADDRESS"), PostgresDSN: requiredEnv(t, "GIMBAL_TEST_POSTGRES_DSN"),
		WorkerPostgresDSN: "postgres://gimbal:gimbal@host.docker.internal:5433/gimbal?sslmode=disable",
		Mounts:            []string{project}, Models: models, SecretFiles: map[string]string{"OPENAI_API_KEY": keyFile},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(gimbal.Project(context.Background(), project), 5*time.Minute)
	defer cancel()
	envName := "supervised-turn-" + uuid.NewString()
	var answer gimbal.Text
	err = gimbal.Run(ctx, "supervised-turn-integration", models, func(ctx context.Context) error {
		ctx = gimbal.InEnvironment(ctx, envName)
		principal := gimbal.NewSession(ctx, "principal", workspace)
		supervisor := gimbal.NewSession(ctx, "supervisor", workspace)
		var err error
		answer, err = principal.Generate[gimbal.Text](ctx, "This is a supervision integration test. Write the word original to note.txt in the current directory. Then run the shell command sleep 45 and wait for it to finish. Follow any instruction you receive while you work. Finally, reply with the exact contents of note.txt and nothing else.",
			gimbal.WithSupervisor(supervisor, "This is a supervision integration test. On your first look, read note.txt in the worker's directory. If it contains original, object with exactly this instruction: Replace the contents of note.txt with supervisor-corrected before replying. On later looks return no objections. Do not edit files.", gimbal.WithInterval(10*time.Second)))
		return err
	}, gimbal.WithExecution(backend))
	if err != nil {
		t.Fatal(err)
	}
	note, readErr := os.ReadFile(filepath.Join(workspace, "note.txt"))
	t.Logf("principal answered %q; note.txt on the host = %q (%v)", answer, note, readErr)

	entries, err := os.ReadDir(filepath.Join(project, "runs"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("runs = %v, %v", entries, err)
	}
	var principalStart, principalEnd, lookStart time.Time
	var lookPrompt string
	workdirs := map[string]string{}
	attached, steered := false, false
	if err := runlog.Read[gimbal.LifecycleRecord](ctx, filepath.Join(project, "runs", entries[0].Name()), func(record gimbal.LifecycleRecord) error {
		session := record.Session.Value
		switch event := record.Event.(type) {
		case gimbal.SessionCreated:
			workdirs[event.Name] = event.Workdir
		case gimbal.SuperviseAttached:
			attached = attached || strings.HasPrefix(event.Worker, "principal") && strings.HasPrefix(event.Reviewer, "supervisor")
		case gimbal.TurnStarted:
			if strings.HasPrefix(session, "principal") && principalStart.IsZero() {
				principalStart = record.Time
			}
			if strings.HasPrefix(session, "supervisor") && lookStart.IsZero() {
				lookStart, lookPrompt = record.Time, event.Prompt
			}
		case gimbal.TurnEnded:
			if strings.HasPrefix(session, "principal") {
				principalEnd = record.Time
			}
		case gimbal.Steer:
			t.Logf("steer %s -> %s landed=%t: %s", event.Source, event.Target, event.Landed, event.Message)
			steered = steered || strings.HasPrefix(event.Target, "principal") && strings.HasPrefix(event.Source, "supervisor") && event.Landed && strings.Contains(event.Message, "supervisor-corrected")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	t.Logf("sessions %v; principal turn %s..%s; first look %s", workdirs, principalStart.Format(time.StampMilli), principalEnd.Format(time.StampMilli), lookStart.Format(time.StampMilli))
	if !attached {
		t.Error("no supervisor was attached to the principal's turn")
	}
	if lookStart.IsZero() || !lookStart.After(principalStart) || !lookStart.Before(principalEnd) {
		t.Error("the supervisor's look did not overlap the principal's turn")
	}
	// The first look also quotes the principal's task; its activity is the
	// part after this heading, which a look is only sent with.
	if _, activity, _ := strings.Cut(lookPrompt, "What the agent did since your last look:"); strings.TrimSpace(activity) == "" {
		t.Errorf("the look did not carry the principal's activity; its prompt was %q", lookPrompt)
	}
	for name, workdir := range workdirs {
		if workdir != workspace {
			t.Errorf("session %s workdir = %q, want the shared workspace %q", name, workdir, workspace)
		}
	}
	if len(backend.environments) != 1 {
		t.Errorf("backend resolved %d environments, want the one shared by both sessions", len(backend.environments))
	}
	if !steered {
		t.Error("no supervisor objection landed in the principal's turn")
	}
	if strings.TrimSpace(string(answer)) != "supervisor-corrected" || strings.TrimSpace(string(note)) != "supervisor-corrected" {
		t.Errorf("principal answered %q with note.txt %q, want the steered supervisor-corrected", answer, note)
	}
}

func TestCodexLoginFailureRedactsAPIKey(t *testing.T) {
	const apiKey = "sk-test-secret-value"
	err := codexLoginFailure(os.ErrInvalid, []byte("codex echoed "+apiKey+" while failing"), apiKey)
	if strings.Contains(err.Error(), apiKey) {
		t.Fatalf("Codex login error leaked API key: %s", err)
	}
	if !strings.Contains(err.Error(), "[REDACTED]") {
		t.Fatalf("Codex login error did not retain a redaction marker: %s", err)
	}
}

func requiredEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required when GIMBAL_EXECUTION_INTEGRATION=1", name)
	}
	return value
}
