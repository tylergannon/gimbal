package execution

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/codex"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

const codexStartupTimeout = 30 * time.Second
const commandStartedHeartbeat = "started"

type commandHeartbeat struct {
	State string `json:"state"`
}

func RunWorker(ctx context.Context) error {
	environment := os.Getenv("GIMBAL_ENVIRONMENT")
	owner := os.Getenv("GIMBAL_WORKER_ID")
	dsn := os.Getenv("GIMBAL_POSTGRES_DSN")
	address := os.Getenv("GIMBAL_TEMPORAL_ADDRESS")
	if environment == "" || owner == "" || dsn == "" || address == "" {
		return errors.New("gimbal-worker: required environment configuration is missing")
	}
	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return err
	}
	defer db.Close()
	var row bootstrap
	var mounts []byte
	var workerConfig []byte
	if err := db.QueryRow(ctx, `SELECT name,task_queue,owner_id,container_name,mounts,worker_config FROM gimbal_environments WHERE name=$1`, environment).Scan(&row.Name, &row.Queue, &row.Owner, &row.Container, &mounts, &workerConfig); err != nil {
		return fmt.Errorf("gimbal-worker: load bootstrap: %w", err)
	}
	if row.Owner != owner {
		return fmt.Errorf("gimbal-worker: bootstrap owner mismatch for %q", environment)
	}
	if err := json.Unmarshal(mounts, &row.Mounts); err != nil {
		return err
	}
	if err := json.Unmarshal(workerConfig, &row); err != nil {
		return fmt.Errorf("gimbal-worker: decode worker configuration: %w", err)
	}
	var openAIAPIKey string
	for name, path := range row.Secrets {
		secret, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("gimbal-worker: read configured secret %s: %w", name, err)
		}
		if name == "OPENAI_API_KEY" {
			openAIAPIKey = strings.TrimSpace(string(secret))
			if openAIAPIKey == "" {
				return errors.New("gimbal-worker: configured OPENAI_API_KEY secret is empty")
			}
		}
	}
	if len(row.Roles) > 0 {
		codexHome := os.Getenv("CODEX_HOME")
		if codexHome == "" {
			return errors.New("gimbal-worker: CODEX_HOME is required for Codex role bindings")
		}
		if err := os.MkdirAll(codexHome, 0o700); err != nil {
			return fmt.Errorf("gimbal-worker: create dedicated Codex state directory: %w", err)
		}
		if err := ensureCodexAuth(ctx, codexHome, openAIAPIKey); err != nil {
			return err
		}
		startCtx, cancel := context.WithTimeout(ctx, codexStartupTimeout)
		defer cancel()
		cmd := exec.CommandContext(startCtx, "codex", "app-server", "daemon", "start")
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("gimbal-worker: start Codex app-server in its dedicated state directory: %w: %s", err, strings.TrimSpace(string(output)))
		}
	}
	temporal, err := client.Dial(client.Options{HostPort: address})
	if err != nil {
		return err
	}
	defer temporal.Close()
	w := worker.New(temporal, row.Queue, worker.Options{Identity: row.Owner, MaxConcurrentActivityExecutionSize: 32})
	w.RegisterActivityWithOptions(func(ctx context.Context, in commandInput) (commandResult, error) {
		return runCommandActivity(ctx, db, in)
	}, activity.RegisterOptions{Name: commandActivity})
	adapter := codex.New()
	w.RegisterActivityWithOptions(func(ctx context.Context, in harnessInput) (harnessResult, error) {
		heartbeatDone := make(chan struct{})
		go func() {
			ticker := time.NewTicker(3 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					activity.RecordHeartbeat(ctx, in.Operation)
				case <-heartbeatDone:
					return
				}
			}
		}()
		defer close(heartbeatDone)
		return runHarnessActivity(ctx, db, adapter, row.Roles, in)
	}, activity.RegisterOptions{Name: harnessActivity})
	errCh := make(chan error, 1)
	go func() { errCh <- w.Run(worker.InterruptCh()) }()
	select {
	case <-ctx.Done():
		w.Stop()
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}

// ensureCodexAuth turns an operator-provided API key into Codex's persisted
// auth record on first use. app-server does not opt into environment API-key
// auth, so merely inheriting OPENAI_API_KEY is insufficient. Once auth exists,
// leave it alone: app-server may have persisted refreshed OAuth tokens there.
func ensureCodexAuth(ctx context.Context, codexHome, apiKey string) error {
	authPath := filepath.Join(codexHome, "auth.json")
	if _, err := os.Stat(authPath); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("gimbal-worker: inspect Codex authentication state: %w", err)
	}
	if apiKey == "" {
		return errors.New("gimbal-worker: Codex authentication is missing and no OPENAI_API_KEY secret was configured")
	}
	loginCtx, cancel := context.WithTimeout(ctx, codexStartupTimeout)
	defer cancel()
	cmd := exec.CommandContext(loginCtx, "codex", "login", "--with-api-key")
	cmd.Stdin = strings.NewReader(apiKey + "\n")
	if output, err := cmd.CombinedOutput(); err != nil {
		return codexLoginFailure(err, output, apiKey)
	}
	if _, err := os.Stat(authPath); err != nil {
		return fmt.Errorf("gimbal-worker: Codex login succeeded without creating %s: %w", authPath, err)
	}
	return nil
}

func codexLoginFailure(err error, output []byte, apiKey string) error {
	redact := func(value string) string {
		if apiKey != "" {
			value = strings.ReplaceAll(value, apiKey, "[REDACTED]")
		}
		return value
	}
	return fmt.Errorf("gimbal-worker: persist Codex API-key authentication: %s: %s", redact(err.Error()), redact(strings.TrimSpace(string(output))))
}

func runHarnessActivity(ctx context.Context, db *pgxpool.Pool, adapter gimbal.HarnessAdapter, roles map[gimbal.WorkflowRole]RoleBinding, in harnessInput) (harnessResult, error) {
	if in.Environment == "" || in.Role == "" || in.Operation == "" {
		return harnessResult{}, errors.New("gimbal-worker: harness operation identity is incomplete")
	}
	binding, ok := roles[gimbal.WorkflowRole(in.Role)]
	if !ok || binding.Harness != "codex" || binding.Model == "" {
		return harnessResult{}, fmt.Errorf("gimbal-worker: no compatible Codex binding for role %q", in.Role)
	}
	if in.Model != "" && (in.Model != binding.Model || in.Effort != binding.Effort) {
		return harnessResult{}, fmt.Errorf("gimbal-worker: operation model %q effort %q does not match role %q binding %q effort %q", in.Model, in.Effort, in.Role, binding.Model, binding.Effort)
	}
	persist := func(session string, event gimbal.AgentEvent) error {
		ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
		defer cancel()
		_, err := db.Exec(ctx, `INSERT INTO gimbal_harness_events(environment_name,operation_id,session_id,role,event) VALUES($1,$2,$3,$4,$5)`, in.Environment, in.Operation, session, in.Role, mustJSON(event))
		return err
	}
	switch in.Action {
	case "create":
		id, err := adapter.CreateSession(ctx, binding.Model, binding.Effort, in.Workdir)
		if err != nil {
			return harnessResult{}, err
		}
		err = persist(id, gimbal.AgentEvent{Type: "gimbal.session.created", Data: mustJSON(map[string]any{"sessionID": id, "workdir": in.Workdir, "model": binding.Model})})
		return harnessResult{Session: id, RecordingError: recordingError(err)}, nil
	case "fork":
		id, err := adapter.Fork(ctx, in.Session)
		if err != nil {
			return harnessResult{}, err
		}
		err = persist(id, gimbal.AgentEvent{Type: "gimbal.session.forked", Data: mustJSON(map[string]any{"sessionID": id, "parentSessionID": in.Session})})
		return harnessResult{Session: id, RecordingError: recordingError(err)}, nil
	case "turn":
		result, degraded, err := runHarnessTurn(ctx, adapter, in.Session, in.Prompt, in.Schema, func(event gimbal.AgentEvent) error { return persist(in.Session, event) })
		return harnessResult{Turn: result, RecordingError: degraded}, err
	case "steer":
		landed, err := adapter.Steer(ctx, in.Session, in.Message)
		if err != nil {
			return harnessResult{}, err
		}
		recordErr := persist(in.Session, gimbal.AgentEvent{Type: "gimbal.session.steer", Data: mustJSON(map[string]any{"sessionID": in.Session, "landed": landed})})
		return harnessResult{Landed: landed, RecordingError: recordingError(recordErr)}, nil
	case "close":
		err := adapter.Close(ctx, in.Session)
		if err != nil {
			return harnessResult{}, err
		}
		recordErr := persist(in.Session, gimbal.AgentEvent{Type: "gimbal.session.closed", Data: mustJSON(map[string]any{"sessionID": in.Session})})
		return harnessResult{RecordingError: recordingError(recordErr)}, nil
	default:
		return harnessResult{}, fmt.Errorf("gimbal-worker: unknown harness action %q", in.Action)
	}
}

func runHarnessTurn(ctx context.Context, adapter gimbal.HarnessAdapter, session, prompt string, schema json.RawMessage, persist func(gimbal.AgentEvent) error) (gimbal.TurnResult, string, error) {
	var mu sync.Mutex
	var recordingErr error
	result, runErr := adapter.RunTurn(ctx, session, prompt, schema, func(event gimbal.AgentEvent) error {
		if err := persist(event); err != nil {
			mu.Lock()
			if recordingErr == nil {
				recordingErr = err
			}
			mu.Unlock()
		}
		return nil
	})
	mu.Lock()
	defer mu.Unlock()
	return result, recordingError(recordingErr), runErr
}

func recordingError(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func runCommandActivity(ctx context.Context, db *pgxpool.Pool, in commandInput) (commandResult, error) {
	result := commandResult{ExitCode: -1}
	out, err := os.OpenFile(in.StdoutPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		runErr := fmt.Errorf("open stdout capture: %w", err)
		return recordCommandEvent(db, in, result, runErr)
	}
	errOut, err := os.OpenFile(in.StderrPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		_ = out.Close()
		runErr := fmt.Errorf("open stderr capture: %w", err)
		return recordCommandEvent(db, in, result, runErr)
	}
	cmd := exec.Command(in.Command, in.Args...)
	cmd.Dir = in.Workdir
	cmd.Stdout, cmd.Stderr = out, errOut
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	runErr := cmd.Start()
	var cleanupErr error
	if runErr == nil {
		heartbeat := commandHeartbeat{State: commandStartedHeartbeat}
		if activity.IsActivity(ctx) {
			activity.RecordHeartbeat(ctx, heartbeat)
		}
		heartbeatDone := make(chan struct{})
		go func() {
			ticker := time.NewTicker(3 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					if activity.IsActivity(ctx) {
						activity.RecordHeartbeat(ctx, heartbeat)
					}
				case <-heartbeatDone:
					return
				}
			}
		}()
		wait := make(chan error, 1)
		go func() { wait <- cmd.Wait() }()
		select {
		case runErr = <-wait:
		case <-ctx.Done():
			_ = signalCommandGroup(cmd.Process.Pid, syscall.SIGTERM)
			select {
			case runErr = <-wait:
			case <-time.After(time.Second):
				_ = signalCommandGroup(cmd.Process.Pid, syscall.SIGKILL)
				runErr = <-wait
			}
		}
		close(heartbeatDone)
		cleanupErr = cleanupCommandGroup(cmd.Process.Pid)
		if cmd.ProcessState != nil {
			result.ExitCode = cmd.ProcessState.ExitCode()
		}
		runErr = commandActivityError(ctx, runErr, cleanupErr)
	}
	if closeErr := errors.Join(out.Close(), errOut.Close()); runErr == nil && closeErr != nil {
		runErr = closeErr
	}
	return recordCommandEvent(db, in, result, runErr)
}

func commandActivityError(ctx context.Context, waitErr, cleanupErr error) error {
	if ctx.Err() != nil {
		if cleanupErr != nil {
			// Temporal treats any error wrapping context.Canceled as cancellation
			// and discards it, so keep a failed cleanup terminal and observable.
			return cleanupErr
		}
		return ctx.Err()
	}
	if _, ok := errors.AsType[*exec.ExitError](waitErr); ok {
		waitErr = nil
	}
	return errors.Join(waitErr, cleanupErr)
}

func signalCommandGroup(pid int, signal syscall.Signal) error {
	err := syscall.Kill(-pid, signal)
	if errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return err
}

func cleanupCommandGroup(pid int) error {
	termErr := signalCommandGroup(pid, syscall.SIGTERM)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		alive, err := commandGroupAlive(pid)
		if err != nil {
			return errors.Join(termErr, err)
		}
		if !alive {
			return termErr
		}
		time.Sleep(10 * time.Millisecond)
	}
	killErr := signalCommandGroup(pid, syscall.SIGKILL)
	deadline = time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		alive, err := commandGroupAlive(pid)
		if err != nil {
			return errors.Join(termErr, killErr, err)
		}
		if !alive {
			return errors.Join(termErr, killErr)
		}
		time.Sleep(10 * time.Millisecond)
	}
	return errors.Join(termErr, killErr, errors.New("worker command process group remains after SIGKILL"))
}

func commandGroupAlive(pid int) (bool, error) {
	err := syscall.Kill(-pid, 0)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, syscall.ESRCH):
		return false, nil
	default:
		return true, err
	}
}

func recordCommandEvent(db *pgxpool.Pool, in commandInput, result commandResult, runErr error) (commandResult, error) {
	// Recording is intentionally best effort and cannot change the process result.
	recordCtx, cancelRecord := context.WithTimeout(context.Background(), 2*time.Second)
	_, recordErr := db.Exec(recordCtx, `INSERT INTO gimbal_command_events(environment_name,operation_id,session_id,role,command,args,workdir,exit_code,error_text) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, in.Environment, in.Operation, in.Session, in.Role, in.Command, mustJSON(in.Args), in.Workdir, result.ExitCode, errorText(runErr))
	cancelRecord()
	if recordErr != nil {
		result.RecordingError = recordErr.Error()
		log.Printf("gimbal-worker: command event recording degraded for %s: %v", in.Operation, recordErr)
	}
	return result, runErr
}

func mustJSON(v any) []byte { raw, _ := json.Marshal(v); return raw }
func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
