package gimble

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const commandOutputLimit = 64 << 10

const commandWaitDelay = 5 * time.Second

type commandCapture struct {
	mu   sync.Mutex
	file *os.File
	err  error
	size int64
}

func (c *commandCapture) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err != nil {
		return 0, c.err
	}
	n, err := c.file.Write(p)
	c.size += int64(n)
	if err != nil {
		c.err = err
	}
	return n, err
}

func (c *commandCapture) close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	err := c.file.Close()
	if c.err == nil {
		c.err = err
	}
	return err
}

func (c *commandCapture) result(runDir, relative string) (string, error) {
	c.mu.Lock()
	size, writeErr := c.size, c.err
	c.mu.Unlock()
	if writeErr != nil {
		return "", writeErr
	}
	name := filepath.Join(runDir, filepath.FromSlash(relative))
	if size <= commandOutputLimit {
		raw, err := os.ReadFile(name)
		return string(raw), err
	}
	keep := int64(commandOutputLimit)
	headSize := keep / 2
	tailSize := keep - headSize
	head := make([]byte, headSize)
	tail := make([]byte, tailSize)
	file, err := os.Open(name)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()
	if _, err := io.ReadFull(file, head); err != nil {
		return "", err
	}
	if _, err := file.ReadAt(tail, size-tailSize); err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	marker := fmt.Sprintf("\n\n[... %d bytes omitted ...]\n\nComplete output: %s\n\n", size-keep, name)
	return string(head) + marker + string(tail), nil
}

func openCommandCapture(r *run, id, stream string) (*commandCapture, string, error) {
	relative := filepath.ToSlash(filepath.Join("commands", encodedPath(id), stream+".log"))
	file, stored, err := r.openArtifact(relative)
	if err != nil {
		return nil, "", err
	}
	return &commandCapture{file: file}, stored, nil
}

// RunCommand runs command with args in workdir and blocks until it exits.
// name is the command's name for the graph, a constant at the call site:
// the command's id is the scope's key and the name with an ordinal, as in
// lap.3/check.2, so every run of one call site is its own record. An empty
// workdir is the process's working directory.
//
// It returns the command's exit code and stdout and stderr as scalars. Small
// streams are returned exactly. A stream over 64 KiB is a bounded head/tail
// excerpt with an absolute path to the complete run-owned file. A nonzero exit
// is not an error. err is for a command that could not start, its ctx
// cancelled, or output could not be captured; then the exit code is -1 except
// when the process's own exit status is known.
//
// Every command that starts streams stdout and stderr into separate files
// under the run while it is running. Partial output remains there after a
// nonzero exit, cancellation, or capture failure.
func RunCommand(ctx context.Context, name, workdir, command string, args ...string) (exitCode int, stdout, stderr string, err error) {
	s, err := current(ctx)
	if err != nil {
		return -1, "", "", err
	}
	_, ended, err, _ := runCommand(ctx, s, name, workdir, command, args)
	return ended.ExitCode, ended.Stdout, ended.Stderr, err
}

// Check runs command and records its result under key in the current scope's
// context. Like Set and SetJSON, key is explicit and may be written only once
// in a scope. Repeated checks therefore use distinct keys written at their call
// sites, such as tests.1 and tests.2. The result records the command,
// arguments, absolute working directory, exit code, stdout, stderr, and any
// execution error. Generate sees it through the usual scope context, and a
// completed PromiseLoop task carries it to the planner through the usual task
// feedback.
//
// A nonzero exit is evidence and returns nil. An error is returned when the
// command could not start, its ctx was cancelled, its output could not be
// captured, or its result could not be recorded. Large streams use the same
// bounded excerpt and complete-output file reference as RunCommand.
func Check(ctx context.Context, key, workdir, command string, args ...string) error {
	s, err := current(ctx)
	if err != nil {
		return err
	}
	if err := checkKeyAvailable(s, key); err != nil {
		return err
	}
	started, ended, commandErr, commandRecordErr := runCommand(ctx, s, key, workdir, command, args)
	result := struct {
		Command  string   `json:"command"`
		Args     []string `json:"args"`
		Workdir  string   `json:"workdir"`
		ExitCode int      `json:"exit_code"`
		Stdout   string   `json:"stdout"`
		Stderr   string   `json:"stderr"`
		Error    string   `json:"error,omitempty"`
	}{
		Command:  started.Command,
		Args:     append([]string(nil), started.Args...),
		Workdir:  started.Workdir,
		ExitCode: ended.ExitCode,
		Stdout:   ended.Stdout,
		Stderr:   ended.Stderr,
		Error:    ended.Error,
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return errors.Join(commandErr, commandRecordErr, fmt.Errorf("gimble: check %q: encode result: %w", key, err))
	}
	return errors.Join(commandErr, commandRecordErr, recordCheckResult(s, key, raw))
}

func checkKeyAvailable(s *scope, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended {
		return fmt.Errorf("gimble: check %q: scope %q ended", key, s.key)
	}
	if _, ok := s.values[key]; ok {
		return fmt.Errorf("gimble: check result %q is already recorded in scope %q", key, s.key)
	}
	return nil
}

func recordCheckResult(s *scope, key string, raw []byte) error {
	if s == nil {
		return errors.New("gimble: check: no scope in the ctx; it must come from gimble.Run")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended {
		return fmt.Errorf("gimble: check %q: scope %q ended", key, s.key)
	}
	if _, ok := s.values[key]; ok {
		return fmt.Errorf("gimble: check result %q is already recorded in scope %q", key, s.key)
	}
	if s.values == nil {
		s.values = make(map[string]*scopeValue)
	}
	value := &scopeValue{owner: s, raw: append([]byte(nil), raw...)}
	if tokenCount("## "+key+"\n\n"+render(raw)) > contextEntryTokenLimit {
		if err := s.spillValueLocked(key, value); err != nil {
			return fmt.Errorf("gimble: check %q: record result: %w", key, err)
		}
	}
	s.values[key] = value
	s.keys = append(s.keys, key)
	if err := s.run.eventResult(s.key, "", "", valueEvent(key, value)); err != nil {
		return fmt.Errorf("gimble: check %q: record result: %w", key, err)
	}
	return nil
}

func runCommand(ctx context.Context, s *scope, name, workdir, command string, args []string) (CommandStarted, CommandEnded, error, error) {
	s.mu.Lock()
	id := s.next(name)
	s.mu.Unlock()
	dir, _ := filepath.Abs(workdir)
	started := CommandStarted{ID: id, Name: name, Command: command, Args: args, Workdir: dir}
	startRecordErr := s.run.eventResult(s.key, "", "", started)
	logf("%s: command started: %s", id, oneLine(strings.Join(append([]string{command}, args...), " ")))
	start := time.Now()
	ended := CommandEnded{ID: id, ExitCode: -1}
	var commandErr error

	out, stdoutFile, captureErr := openCommandCapture(s.run, id, "stdout")
	if captureErr == nil {
		var errOut *commandCapture
		errOut, ended.StderrFile, captureErr = openCommandCapture(s.run, id, "stderr")
		if captureErr == nil {
			ended.StdoutFile = stdoutFile
			_, _, commandErr = runCapturedCommand(ctx, id, workdir, command, args, s.run, out, errOut, &ended)
		} else {
			_ = out.close()
			ended.StdoutFile = stdoutFile
		}
	}
	if captureErr != nil {
		commandErr = fmt.Errorf("gimble: %s: capture output: %w", id, captureErr)
		s.run.recordFailure("capture command output "+id, captureErr)
	}
	ended.Error, ended.Duration = errString(commandErr), time.Since(start)
	endRecordErr := s.run.eventResult(s.key, "", "", ended)
	logf("%s: command ended after %s: exit %d: %v", id, ended.Duration.Round(time.Millisecond), ended.ExitCode, orNone(commandErr))
	return started, ended, commandErr, errors.Join(startRecordErr, endRecordErr)
}

func runCapturedCommand(ctx context.Context, id, workdir, command string, args []string, r *run, out, errOut *commandCapture, ended *CommandEnded) (stdout, stderr string, err error) {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = workdir
	cmd.WaitDelay = commandWaitDelay
	var killed atomic.Bool
	cmd.Cancel = func() error {
		err := cmd.Process.Kill()
		killed.Store(err == nil)
		return err
	}
	cmd.Stdout, cmd.Stderr = out, errOut
	startErr := cmd.Start()
	var runErr error
	if startErr == nil {
		runErr = cmd.Wait()
	}
	closeErr := errors.Join(out.close(), errOut.close())
	stdout, stdoutErr := out.result(r.dir, ended.StdoutFile)
	stderr, stderrErr := errOut.result(r.dir, ended.StderrFile)
	captureErr := errors.Join(closeErr, stdoutErr, stderrErr)
	if captureErr != nil {
		r.recordFailure("capture command output "+id, captureErr)
	}

	switch {
	case captureErr != nil:
		if cmd.ProcessState != nil {
			ended.ExitCode = cmd.ProcessState.ExitCode()
		}
		err = fmt.Errorf("gimble: %s: capture output: %w", id, captureErr)
	case killed.Load() || cmd.ProcessState == nil && errors.Is(startErr, ctx.Err()):
		cause := context.Cause(ctx)
		if cause != ctx.Err() {
			cause = fmt.Errorf("%w: %w", ctx.Err(), cause)
		}
		err = fmt.Errorf("gimble: %s: %w", id, cause)
		ended.Interrupted = true
	case cmd.ProcessState == nil:
		err = fmt.Errorf("gimble: %s: %w", id, startErr)
	default:
		ended.ExitCode = cmd.ProcessState.ExitCode()
		var exitErr *exec.ExitError
		if runErr != nil && !errors.As(runErr, &exitErr) && !errors.Is(runErr, exec.ErrWaitDelay) {
			err = fmt.Errorf("gimble: %s: %w", id, runErr)
		}
	}
	ended.Stdout, ended.Stderr = stdout, stderr
	return stdout, stderr, err
}
