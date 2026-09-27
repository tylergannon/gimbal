// Package execution implements Gimbal's first command backend slice. For a
// local run, start Temporal on the host with `temporal server start-dev --ip
// 0.0.0.0`, start Postgres with `docker compose -f
// docker-compose.temporal.yaml up -d postgres`, and build the worker image with
// `docker build -f Dockerfile.gimbal-worker -t gimbal-command-worker:local .`.
// New takes the host addresses; WorkerTemporalAddress and WorkerPostgresDSN
// override them for addresses reachable from Docker.
package execution

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tylergannon/gimbal"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
)

const commandActivity = "gimbal.execute-command"

const (
	workerReadyTimeout  = 45 * time.Second
	workerReadyInterval = 250 * time.Millisecond
)

type Config struct {
	DockerImage           string
	TemporalAddress       string // host-side Temporal address
	PostgresDSN           string // host-side Postgres DSN
	WorkerTemporalAddress string
	WorkerPostgresDSN     string
	Mounts                []string
	DockerExecutable      string
}

type Backend struct {
	cfg          Config
	db           *pgxpool.Pool
	temporal     client.Client
	owner        string
	mu           sync.Mutex
	environments map[string]*Environment
	containers   []string
	closed       bool
}

type bootstrap struct {
	Name      string   `json:"name"`
	Queue     string   `json:"queue"`
	Owner     string   `json:"owner"`
	Container string   `json:"container"`
	Mounts    []string `json:"mounts"`
}

type commandInput struct {
	Environment string   `json:"environment"`
	Operation   string   `json:"operation"`
	Session     string   `json:"session"`
	Role        string   `json:"role"`
	Workdir     string   `json:"workdir"`
	Command     string   `json:"command"`
	Args        []string `json:"args"`
	StdoutPath  string   `json:"stdout_path"`
	StderrPath  string   `json:"stderr_path"`
}

type commandResult struct {
	ExitCode       int    `json:"exit_code"`
	RecordingError string `json:"recording_error,omitempty"`
}

type workerReadinessProbe interface {
	checkWorkerReady(context.Context, bootstrap) (bool, string, error)
}

func New(ctx context.Context, cfg Config) (*Backend, error) {
	if cfg.DockerImage == "" || cfg.TemporalAddress == "" || cfg.PostgresDSN == "" {
		return nil, errors.New("execution: Docker image, Temporal address, and Postgres DSN are required")
	}
	if cfg.DockerExecutable == "" {
		cfg.DockerExecutable = "docker"
	}
	if cfg.WorkerTemporalAddress == "" {
		cfg.WorkerTemporalAddress = "host.docker.internal:7233"
	}
	if cfg.WorkerPostgresDSN == "" {
		cfg.WorkerPostgresDSN = strings.Replace(cfg.PostgresDSN, "localhost", "host.docker.internal", 1)
	}
	for i, mount := range cfg.Mounts {
		if !filepath.IsAbs(mount) {
			abs, err := filepath.Abs(mount)
			if err != nil {
				return nil, err
			}
			cfg.Mounts[i] = abs
		}
	}
	db, err := pgxpool.New(ctx, cfg.PostgresDSN)
	if err != nil {
		return nil, fmt.Errorf("execution: connect Postgres: %w", err)
	}
	if err := db.Ping(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("execution: ping Postgres: %w", err)
	}
	if _, err := db.Exec(ctx, `CREATE TABLE IF NOT EXISTS gimbal_environments (
		name text PRIMARY KEY, task_queue text NOT NULL UNIQUE, owner_id text NOT NULL,
		container_name text NOT NULL UNIQUE, mounts jsonb NOT NULL, created_at timestamptz NOT NULL DEFAULT now()
	)`); err != nil {
		db.Close()
		return nil, fmt.Errorf("execution: create bootstrap table: %w", err)
	}
	if _, err := db.Exec(ctx, `CREATE TABLE IF NOT EXISTS gimbal_command_events (
		id bigserial PRIMARY KEY, environment_name text NOT NULL, operation_id text NOT NULL,
		session_id text NOT NULL, role text NOT NULL, command text NOT NULL, args jsonb NOT NULL,
		workdir text NOT NULL, exit_code integer NOT NULL, error_text text NOT NULL DEFAULT '',
		stdout text NOT NULL DEFAULT '', stderr text NOT NULL DEFAULT '', created_at timestamptz NOT NULL DEFAULT now()
	)`); err != nil {
		db.Close()
		return nil, fmt.Errorf("execution: create event table: %w", err)
	}
	temporal, err := client.Dial(client.Options{HostPort: cfg.TemporalAddress})
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("execution: connect Temporal: %w", err)
	}
	return &Backend{cfg: cfg, db: db, temporal: temporal, owner: uuid.NewString(), environments: make(map[string]*Environment)}, nil
}

func (b *Backend) Resolve(ctx context.Context, name string) (gimbal.ExecutionEnvironment, error) {
	if name == "" {
		return nil, errors.New("execution: environment name is empty")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil, errors.New("execution: backend is closed")
	}
	if env := b.environments[name]; env != nil {
		return env, nil
	}
	hash := sha256.Sum256([]byte(name))
	queue := "gimbal-env-" + hex.EncodeToString(hash[:12])
	containerName := "gimbal-worker-" + hex.EncodeToString(hash[:8]) + "-" + b.owner[:8]
	mounts, _ := json.Marshal(b.cfg.Mounts)
	row := bootstrap{Name: name, Queue: queue, Owner: b.owner, Container: containerName, Mounts: b.cfg.Mounts}
	_, err := b.db.Exec(ctx, `INSERT INTO gimbal_environments(name,task_queue,owner_id,container_name,mounts) VALUES($1,$2,$3,$4,$5)`, name, queue, b.owner, containerName, mounts)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, fmt.Errorf("execution: environment %q already has an owner", name)
		}
		return nil, fmt.Errorf("execution: persist bootstrap for %q: %w", name, err)
	}
	if err := b.startWorker(ctx, row); err != nil {
		deleteCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_, _ = b.db.Exec(deleteCtx, `DELETE FROM gimbal_environments WHERE name=$1 AND owner_id=$2`, name, b.owner)
		cancel()
		return nil, err
	}
	env := &Environment{backend: b, bootstrap: row}
	b.environments[name] = env
	b.containers = append(b.containers, containerName)
	return env, nil
}

func (b *Backend) startWorker(ctx context.Context, row bootstrap) error {
	mounts, _ := json.Marshal(row.Mounts)
	args := []string{"run", "-d", "--name", row.Container, "--add-host", "host.docker.internal:host-gateway",
		"-e", "GIMBAL_ENVIRONMENT=" + row.Name, "-e", "GIMBAL_WORKER_ID=" + row.Owner,
		"-e", "GIMBAL_POSTGRES_DSN=" + b.cfg.WorkerPostgresDSN, "-e", "GIMBAL_TEMPORAL_ADDRESS=" + b.cfg.WorkerTemporalAddress,
		"-e", "GIMBAL_MOUNTS=" + string(mounts)}
	for _, mount := range row.Mounts {
		args = append(args, "-v", mount+":"+mount)
	}
	args = append(args, b.cfg.DockerImage, "worker")
	cmd := exec.CommandContext(ctx, b.cfg.DockerExecutable, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return b.workerStartupFailure(row, fmt.Errorf("docker run failed: %w: %s", err, strings.TrimSpace(string(output))))
	}
	readyCtx, cancel := context.WithTimeout(ctx, workerReadyTimeout)
	defer cancel()
	if err := waitForWorkerReady(readyCtx, workerReadyInterval, b, row); err != nil {
		return b.workerStartupFailure(row, err)
	}
	return nil
}

func (b *Backend) checkWorkerReady(ctx context.Context, row bootstrap) (bool, string, error) {
	inspect := exec.CommandContext(ctx, b.cfg.DockerExecutable, "inspect", "--format", "{{.State.Status}}", row.Container)
	state, err := inspect.CombinedOutput()
	if err != nil {
		return false, "", fmt.Errorf("inspect container: %w: %s", err, strings.TrimSpace(string(state)))
	}
	status := strings.TrimSpace(string(state))
	if status != "running" {
		return false, "", fmt.Errorf("container state is %q", status)
	}
	response, err := b.temporal.DescribeTaskQueue(ctx, row.Queue, enums.TASK_QUEUE_TYPE_ACTIVITY)
	if err != nil {
		return false, "container is running, but Temporal task queue lookup failed: " + err.Error(), nil
	}
	for _, poller := range response.GetPollers() {
		if poller.GetIdentity() == row.Owner {
			return true, "", nil
		}
	}
	return false, "container is running, waiting for its Temporal activity poller", nil
}

func waitForWorkerReady(ctx context.Context, interval time.Duration, probe workerReadinessProbe, row bootstrap) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	lastStatus := "worker has not reported readiness"
	for {
		ready, status, err := probe.checkWorkerReady(ctx, row)
		if status != "" {
			lastStatus = status
		}
		if err != nil {
			return fmt.Errorf("worker readiness check failed: %w", err)
		}
		if ready {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("worker readiness timed out after %s: %s: %w", workerReadyTimeout, lastStatus, ctx.Err())
		case <-ticker.C:
		}
	}
}

func (b *Backend) workerStartupFailure(row bootstrap, cause error) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	logsCmd := exec.CommandContext(ctx, b.cfg.DockerExecutable, "logs", row.Container)
	logs, logsErr := logsCmd.CombinedOutput()
	removeCtx, cancelRemove := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelRemove()
	removeCmd := exec.CommandContext(removeCtx, b.cfg.DockerExecutable, "rm", "-f", row.Container)
	removeOutput, removeErr := removeCmd.CombinedOutput()
	message := fmt.Sprintf("execution: worker for environment %q failed to become ready: %v", row.Name, cause)
	if len(strings.TrimSpace(string(logs))) > 0 {
		message += "; docker logs: " + strings.TrimSpace(string(logs))
	} else if logsErr != nil {
		message += "; docker logs unavailable: " + logsErr.Error()
	}
	if removeErr != nil {
		message += fmt.Sprintf("; remove container failed: %v: %s", removeErr, strings.TrimSpace(string(removeOutput)))
	}
	return errors.New(message)
}

func (b *Backend) Close() error {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return nil
	}
	b.closed = true
	containers := append([]string(nil), b.containers...)
	b.mu.Unlock()
	var errs []error
	for _, name := range containers {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		cmd := exec.CommandContext(ctx, b.cfg.DockerExecutable, "rm", "-f", name)
		if output, err := cmd.CombinedOutput(); err != nil {
			errs = append(errs, fmt.Errorf("remove worker %s: %w: %s", name, err, strings.TrimSpace(string(output))))
			cancel()
			continue
		}
		cancel()
		deleteCtx, cancelDelete := context.WithTimeout(context.Background(), 2*time.Second)
		_, err := b.db.Exec(deleteCtx, `DELETE FROM gimbal_environments WHERE container_name=$1 AND owner_id=$2`, name, b.owner)
		cancelDelete()
		if err != nil {
			errs = append(errs, fmt.Errorf("remove bootstrap for worker %s: %w", name, err))
		}
	}
	b.temporal.Close()
	b.db.Close()
	return errors.Join(errs...)
}

type Environment struct {
	backend   *Backend
	bootstrap bootstrap
}

func (*Environment) Harness(gimbal.WorkflowRole) gimbal.HarnessAdapter { return nil }

func (e *Environment) Start(ctx context.Context, command gimbal.ExecutionCommand, stdout, stderr io.Writer) (gimbal.ExecutionProcess, error) {
	if !filepath.IsAbs(command.Workdir) {
		return nil, fmt.Errorf("execution: command workdir must be absolute: %q", command.Workdir)
	}
	if !mounted(e.bootstrap.Mounts, command.Workdir) {
		return nil, fmt.Errorf("execution: workdir %q is not in the configured mounts", command.Workdir)
	}
	if !mounted(e.bootstrap.Mounts, command.StdoutPath) || !mounted(e.bootstrap.Mounts, command.StderrPath) {
		return nil, errors.New("execution: command output files are outside the configured mounts")
	}
	input := commandInput{Environment: e.bootstrap.Name, Operation: command.Operation, Session: command.Session, Role: command.Role, Workdir: command.Workdir, Command: command.Command, Args: command.Args, StdoutPath: command.StdoutPath, StderrPath: command.StderrPath}
	activityID := command.Operation
	if activityID == "" {
		return nil, errors.New("execution: command operation ID is required")
	}
	handle, err := e.backend.temporal.ExecuteActivity(ctx, client.StartActivityOptions{ID: activityID, TaskQueue: e.bootstrap.Queue, ScheduleToStartTimeout: 30 * time.Second, StartToCloseTimeout: 24 * time.Hour, HeartbeatTimeout: 15 * time.Second, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 1}}, commandActivity, input)
	if err != nil {
		return nil, fmt.Errorf("execution: schedule command activity: %w", err)
	}
	p := &Process{ctx: ctx, handle: handle, workdir: command.Workdir, operation: command.Operation}
	return p, nil
}

func mounted(mounts []string, workdir string) bool {
	for _, root := range mounts {
		rel, err := filepath.Rel(root, workdir)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

type Process struct {
	ctx        context.Context
	handle     activityHandle
	workdir    string
	operation  string
	mu         sync.Mutex
	waitDone   chan struct{}
	waited     bool
	code       int
	waitErr    error
	cancelOnce sync.Once
	cancelErr  error
	stopOnce   sync.Once
	stopErr    error
}

type activityHandle interface {
	Get(context.Context, any) error
	Cancel(context.Context, client.CancelActivityOptions) error
}

func (p *Process) Workdir() string { return p.workdir }
func (p *Process) Wait() (int, error) {
	waitDone := p.ensureWait()
	select {
	case <-waitDone:
	case <-p.ctx.Done():
		p.requestCancel()
		select {
		case <-p.waitDone:
		case <-time.After(10 * time.Second):
			return -1, errors.Join(p.ctx.Err(), p.cancelErr, errors.New("execution: activity cancellation was not confirmed before timeout"))
		}
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.code, errors.Join(p.waitErr, p.ctx.Err(), p.cancelErr)
}

func (p *Process) ensureWait() <-chan struct{} {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.waitDone == nil {
		p.waitDone = make(chan struct{})
		go p.collect()
	}
	return p.waitDone
}

func (p *Process) collect() {
	var result commandResult
	err := p.handle.Get(context.Background(), &result)
	if result.RecordingError != "" {
		log.Printf("gimbal: command event recording degraded for %s: %s", p.operation, result.RecordingError)
	}
	p.mu.Lock()
	p.code, p.waitErr, p.waited = result.ExitCode, err, true
	if err != nil {
		p.code = -1
	}
	close(p.waitDone)
	p.mu.Unlock()
}

func (p *Process) requestCancel() {
	p.cancelOnce.Do(func() {
		reason := "execution process stopped"
		if p.ctx.Err() != nil {
			reason = p.ctx.Err().Error()
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		p.cancelErr = p.handle.Cancel(ctx, client.CancelActivityOptions{Reason: reason})
	})
}

func (p *Process) Stop() error {
	p.stopOnce.Do(func() {
		p.mu.Lock()
		waited := p.waited
		p.mu.Unlock()
		if !waited {
			p.requestCancel()
			waitDone := p.ensureWait()
			select {
			case <-waitDone:
			case <-time.After(10 * time.Second):
				p.stopErr = errors.Join(p.cancelErr, errors.New("execution: stopped activity did not finish before timeout"))
			}
		}
	})
	return errors.Join(p.stopErr, p.cancelErr)
}
