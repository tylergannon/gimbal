// Package program provides small Go primitives for composing Gimbal programs.
package program

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/tylergannon/gimbal/harness"
	"github.com/tylergannon/gimbal/harness/agy"
	"github.com/tylergannon/gimbal/harness/claude"
	"github.com/tylergannon/gimbal/harness/codex"
)

const (
	defaultModel      = "gpt-5.6-terra"
	defaultJudgeModel = "gpt-5.6-luna"
	defaultEffort     = "high"
	defaultTimeout    = 20 * time.Minute
)

// Config initializes a Runtime. Adapters is keyed by harness name (codex,
// claude, agy); leave it nil to use Gimbal's native adapters.
type Config struct {
	Workdir           string
	RunDir            string
	Adapters          map[string]harness.HarnessAdapter
	DefaultModel      string
	DefaultJudgeModel string
	ReasoningEffort   string
	Timeout           time.Duration
}

// Runtime owns the workspace, artifacts, and harness adapters for a program.
type Runtime struct {
	Workdir           string
	RunDir            string
	Adapters          map[string]harness.HarnessAdapter
	DefaultModel      string
	DefaultJudgeModel string
	ReasoningEffort   string
	Timeout           time.Duration

	closers []interface{ Close() }
	logMu   sync.Mutex
	nextID  atomic.Uint64
}

// CommandResult reports a shell command's semantic result. A nonzero exit is
// data; failure to start or collect the command is returned as an error.
type CommandResult struct {
	ExitCode int    `json:"exit_code"`
	Output   string `json:"output"`
}

// NewRuntime prepares a program runtime and, unless Adapters is supplied,
// constructs Gimbal's native Codex, Claude, and agy adapters.
func NewRuntime(config Config) (*Runtime, error) {
	workdir := config.Workdir
	if strings.TrimSpace(workdir) == "" {
		workdir = "."
	}
	workdir, err := filepath.Abs(workdir)
	if err != nil {
		return nil, fmt.Errorf("resolve workdir: %w", err)
	}
	info, err := os.Stat(workdir)
	if err != nil || !info.IsDir() {
		if err == nil {
			err = errors.New("not a directory")
		}
		return nil, fmt.Errorf("workdir %s: %w", workdir, err)
	}
	runDir := config.RunDir
	if strings.TrimSpace(runDir) == "" {
		runDir, err = os.MkdirTemp("", "gimbal-program-")
	} else {
		runDir, err = filepath.Abs(runDir)
		if err == nil {
			if entries, readErr := os.ReadDir(runDir); readErr == nil && len(entries) > 0 {
				return nil, fmt.Errorf("run directory %s is not empty", runDir)
			} else if readErr != nil && !os.IsNotExist(readErr) {
				return nil, fmt.Errorf("inspect run directory: %w", readErr)
			}
			err = os.MkdirAll(runDir, 0o755)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("prepare run directory: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(runDir, "agents"), 0o755); err != nil {
		return nil, fmt.Errorf("prepare agent logs: %w", err)
	}
	r := &Runtime{Workdir: workdir, RunDir: runDir, Adapters: config.Adapters,
		DefaultModel: config.DefaultModel, DefaultJudgeModel: config.DefaultJudgeModel,
		ReasoningEffort: config.ReasoningEffort, Timeout: config.Timeout}
	if r.DefaultModel == "" {
		r.DefaultModel = defaultModel
	}
	if r.DefaultJudgeModel == "" {
		r.DefaultJudgeModel = defaultJudgeModel
	}
	if r.ReasoningEffort == "" {
		r.ReasoningEffort = defaultEffort
	}
	if r.Timeout == 0 {
		r.Timeout = defaultTimeout
	}
	if r.Adapters == nil {
		codexAdapter, claudeAdapter, agyAdapter := codex.New(), claude.New(), agy.New()
		r.Adapters = map[string]harness.HarnessAdapter{"codex": codexAdapter, "claude": claudeAdapter, "agy": agyAdapter}
		r.closers = []interface{ Close() }{codexAdapter, claudeAdapter, agyAdapter}
	}
	return r, nil
}

// Close releases native adapter resources created by NewRuntime.
func (r *Runtime) Close() {
	for _, closer := range r.closers {
		closer.Close()
	}
}

// Command executes command through /bin/sh in the runtime workspace.
func (r *Runtime) Command(ctx context.Context, command string) (CommandResult, error) {
	id, started, err := r.startOperation("command", "", map[string]any{"command": command})
	if err != nil {
		return CommandResult{}, err
	}
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", command)
	cmd.Dir = r.Workdir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	output, runErr := cmd.CombinedOutput()
	result := CommandResult{Output: string(output)}
	outputPath := filepath.Join(r.RunDir, id+".log")
	if writeErr := os.WriteFile(outputPath, output, 0o644); writeErr != nil {
		return result, fmt.Errorf("write command output: %w", writeErr)
	}
	var infraErr error
	if ctx.Err() != nil {
		infraErr = ctx.Err()
	} else if runErr != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](runErr); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			infraErr = fmt.Errorf("run command: %w", runErr)
		}
	}
	endErr := r.endOperation(id, "command", started, map[string]any{"exit_code": result.ExitCode, "error": errorString(infraErr), "output": outputPath})
	if infraErr != nil {
		return result, infraErr
	}
	return result, endErr
}

func (r *Runtime) operationLog() string { return filepath.Join(r.RunDir, "operations.jsonl") }

func (r *Runtime) startOperation(kind, parent string, fields map[string]any) (string, time.Time, error) {
	started := time.Now().UTC()
	id := fmt.Sprintf("op-%06d", r.nextID.Add(1))
	record := map[string]any{"type": "operation_start", "id": id, "operation": kind, "timestamp": started.Format(time.RFC3339Nano)}
	if parent != "" {
		record["parent_id"] = parent
	}
	maps.Copy(record, fields)
	return id, started, r.appendRecord(record)
}

func (r *Runtime) endOperation(id, kind string, started time.Time, fields map[string]any) error {
	now := time.Now().UTC()
	record := map[string]any{"type": "operation_end", "id": id, "operation": kind, "timestamp": now.Format(time.RFC3339Nano), "duration_ms": now.Sub(started).Milliseconds()}
	for key, value := range fields {
		if value != "" && value != nil {
			record[key] = value
		}
	}
	return r.appendRecord(record)
}

func (r *Runtime) appendRecord(record map[string]any) error {
	r.logMu.Lock()
	defer r.logMu.Unlock()
	file, err := os.OpenFile(r.operationLog(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open operation log: %w", err)
	}
	w := bufio.NewWriter(file)
	encodeErr := json.NewEncoder(w).Encode(record)
	flushErr := w.Flush()
	closeErr := file.Close()
	if encodeErr != nil {
		return fmt.Errorf("encode operation log: %w", encodeErr)
	}
	if flushErr != nil {
		return fmt.Errorf("flush operation log: %w", flushErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close operation log: %w", closeErr)
	}
	return nil
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
