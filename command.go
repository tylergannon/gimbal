package gimble

import (
	"context"
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
	s.mu.Lock()
	id := s.next(name)
	s.mu.Unlock()
	dir, _ := filepath.Abs(workdir)
	s.run.event(s.key, "", "", CommandStarted{ID: id, Name: name, Command: command, Args: args, Workdir: dir})
	logf("%s: command started: %s", id, oneLine(strings.Join(append([]string{command}, args...), " ")))
	start := time.Now()
	ended := CommandEnded{ID: id, ExitCode: -1}

	openCapture := func(stream string) (*commandCapture, string, error) {
		relative := filepath.ToSlash(filepath.Join("commands", encodedPath(id), stream+".log"))
		file, stored, err := s.run.openArtifact(relative)
		if err != nil {
			return nil, "", err
		}
		return &commandCapture{file: file}, stored, nil
	}
	out, stdoutFile, captureErr := openCapture("stdout")
	if captureErr == nil {
		var errOut *commandCapture
		errOut, ended.StderrFile, captureErr = openCapture("stderr")
		if captureErr == nil {
			ended.StdoutFile = stdoutFile
			stdout, stderr, err = runCapturedCommand(ctx, id, workdir, command, args, s.run, out, errOut, &ended)
		} else {
			_ = out.close()
			ended.StdoutFile = stdoutFile
		}
	}
	if captureErr != nil {
		err = fmt.Errorf("gimble: %s: capture output: %w", id, captureErr)
		s.run.recordFailure("capture command output "+id, captureErr)
	}
	ended.Error, ended.Duration = errString(err), time.Since(start)
	s.run.event(s.key, "", "", ended)
	logf("%s: command ended after %s: exit %d: %v", id, ended.Duration.Round(time.Millisecond), ended.ExitCode, orNone(err))
	return ended.ExitCode, stdout, stderr, err
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
