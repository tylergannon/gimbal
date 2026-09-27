package execution

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tylergannon/gimbal"
)

func TestResolveWorkerReadinessIntegration(t *testing.T) {
	if os.Getenv("GIMBAL_EXECUTION_INTEGRATION") != "1" {
		t.Skip("set GIMBAL_EXECUTION_INTEGRATION=1 to use local Docker, Postgres, and Temporal")
	}
	postgresDSN := requiredEnv(t, "GIMBAL_TEST_POSTGRES_DSN")
	temporalAddress := requiredEnv(t, "GIMBAL_TEST_TEMPORAL_ADDRESS")
	image := os.Getenv("GIMBAL_TEST_WORKER_IMAGE")
	if image == "" {
		image = "gimbal-command-worker:local"
	}
	name := "readiness-" + uuid.NewString()
	workerDSN := "postgres://gimbal:gimbal@host.docker.internal:65534/gimbal?sslmode=disable&connect_timeout=2"

	badConfig := Config{DockerImage: image, TemporalAddress: temporalAddress, PostgresDSN: postgresDSN, WorkerPostgresDSN: workerDSN, Mounts: []string{t.TempDir()}}
	badBackend, err := New(context.Background(), badConfig)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256([]byte(name))
	failedContainer := "gimbal-worker-" + hex.EncodeToString(hash[:8]) + "-" + badBackend.owner[:8]
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
	if err := db.QueryRow(checkCtx, `SELECT count(*) FROM gimbal_environments WHERE name=$1`, name).Scan(&rows); err != nil {
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
	goodConfig := Config{DockerImage: image, TemporalAddress: temporalAddress, PostgresDSN: postgresDSN,
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
	image := os.Getenv("GIMBAL_TEST_WORKER_IMAGE")
	if image == "" {
		image = "gimbal-temporal-codex-worker:local"
	}
	workdir := t.TempDir()
	secretFiles := map[string]string{"OPENAI_API_KEY": keyFile}
	backend, err := New(context.Background(), Config{
		DockerImage: image, TemporalAddress: temporalAddress, PostgresDSN: postgresDSN,
		WorkerPostgresDSN: "postgres://gimbal:gimbal@host.docker.internal:5433/gimbal?sslmode=disable",
		Mounts:            []string{workdir}, Roles: map[gimbal.WorkflowRole]RoleBinding{
			"coder": {Harness: "codex", Model: "gpt-5.6-luna", Effort: "low"},
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
