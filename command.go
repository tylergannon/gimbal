package gimble

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
)

// commandOutputLimit bounds how much of each output stream a command's
// record holds. Longer output is written whole to a file in the run's
// directory, and the record keeps its tail and the file's name.
const commandOutputLimit = 64 << 10

// commandWaitDelay bounds the wait for a command's output once the command
// has exited or its ctx has ended, so a process it left behind holding the
// output open cannot hold RunCommand too.
const commandWaitDelay = 5 * time.Second

// RunCommand runs command with args in workdir and blocks until it exits.
// name is the command's name for the graph, a constant at the call site:
// the command's id is the scope's key and the name with an ordinal, as in
// lap.3/check.2, so every run of one call site is its own record. An empty
// workdir is the process's working directory.
//
// It returns the command's exit code and what it wrote to stdout and
// stderr. A nonzero exit is not an error. err is for a command that could
// not start, or that its ctx cancelled, and then the exit code is -1.
//
// The run records the command in the ctx's scope: what ran where, when it
// started and ended, how it ended, and its output.
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

	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = workdir
	cmd.WaitDelay = commandWaitDelay
	// The ctx interrupted the command only if exec had to kill it for the
	// ctx: a command that exited first keeps its own exit status.
	var killed atomic.Bool
	cmd.Cancel = func() error {
		err := cmd.Process.Kill()
		killed.Store(err == nil)
		return err
	}
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	runErr := cmd.Run()
	stdout, stderr = out.String(), errOut.String()

	ended := CommandEnded{ID: id, ExitCode: -1}
	switch {
	case killed.Load() || cmd.ProcessState == nil && errors.Is(runErr, ctx.Err()):
		// Its ctx ended it, or had ended before it could start. Keep
		// errors.Is(err, context.Canceled) true, and errors.As for a Killed
		// cause, as a turn's error does.
		cause := context.Cause(ctx)
		if cause != ctx.Err() {
			cause = fmt.Errorf("%w: %w", ctx.Err(), cause)
		}
		err = fmt.Errorf("gimble: %s: %w", id, cause)
		ended.Interrupted = true
	case cmd.ProcessState == nil:
		err = fmt.Errorf("gimble: %s: %w", id, runErr)
	default:
		ended.ExitCode = cmd.ProcessState.ExitCode()
	}
	ended.Error, ended.Duration = errString(err), time.Since(start)
	ended.Stdout, ended.StdoutFile = s.run.commandOutput(id, "stdout", stdout)
	ended.Stderr, ended.StderrFile = s.run.commandOutput(id, "stderr", stderr)
	s.run.event(s.key, "", "", ended)
	logf("%s: command ended after %s: exit %d: %v", id, ended.Duration.Round(time.Millisecond), ended.ExitCode, orNone(err))
	return ended.ExitCode, stdout, stderr, err
}

// commandOutput is what a command's record holds of one output stream: all
// of it, or, past commandOutputLimit, its tail and the name of the file in
// the run's directory that holds all of it.
func (r *run) commandOutput(id, stream, output string) (kept, file string) {
	if len(output) <= commandOutputLimit {
		return output, ""
	}
	kept = output[len(output)-commandOutputLimit:]
	file = path.Join("commands", id+"."+stream)
	name := filepath.Join(r.dir, filepath.FromSlash(file))
	err := os.MkdirAll(filepath.Dir(name), 0o755)
	if err == nil {
		err = os.WriteFile(name, []byte(output), 0o644)
	}
	if err != nil {
		r.recordFailure("write command output "+file, err)
		return kept, ""
	}
	return kept, file
}
