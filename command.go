package gimbal

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
	"time"

	"github.com/tylergannon/gimbal/internal/commandexecutor"
	"github.com/tylergannon/gimbal/internal/effects"
	"github.com/tylergannon/gimbal/internal/executor"
	"github.com/tylergannon/gimbal/runstore"
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
	writeErr := c.err
	c.mu.Unlock()
	if writeErr != nil {
		return "", writeErr
	}
	raw, err := os.ReadFile(filepath.Join(runDir, filepath.FromSlash(relative)))
	return string(raw), err
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
// It returns the command's exit code and stdout and stderr as scalars. Complete
// captured streams are returned exactly; observation excerpts remain bounded. A nonzero exit
// is not an error. err is for a command that could not start, its ctx
// cancelled, or output could not be captured or recorded; then the exit code is -1 except
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
	_, ended, commandErr, recordErr := runCommand(ctx, s, name, workdir, command, args)
	return ended.ExitCode, ended.Stdout, ended.Stderr, errors.Join(commandErr, recordErr)
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
	result, commandErr := executeCheck(ctx, s, key, workdir, command, args)
	raw, err := json.Marshal(result)
	return errors.Join(commandErr, err, recordCheckResult(s, key, raw))
}

type checkResult struct {
	Command  string   `json:"command"`
	Args     []string `json:"args"`
	Workdir  string   `json:"workdir"`
	ExitCode int      `json:"exit_code"`
	Stdout   string   `json:"stdout"`
	Stderr   string   `json:"stderr"`
	Error    string   `json:"error,omitempty"`
}

// executeCheck constructs the record from the exact command observations for
// Check, including resolved paths and execution errors.
func executeCheck(ctx context.Context, s *scope, key, workdir, command string, args []string) (checkResult, error) {
	started, ended, commandErr, recordErr := runCommand(ctx, s, key, workdir, command, args)
	return checkResult{Command: started.Command, Args: append([]string(nil), started.Args...), Workdir: started.Workdir, ExitCode: ended.ExitCode, Stdout: ended.Stdout, Stderr: ended.Stderr, Error: ended.Error}, errors.Join(commandErr, recordErr)
}

func checkKeyAvailable(s *scope, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended {
		return fmt.Errorf("gimbal: check %q: scope %q ended", key, s.key)
	}
	if _, ok := s.values[key]; ok {
		return fmt.Errorf("gimbal: check result %q is already recorded in scope %q", key, s.key)
	}
	return nil
}

func recordCheckResult(s *scope, key string, raw []byte) error {
	if s == nil {
		return errors.New("gimbal: check: no scope in the ctx; it must come from gimbal.Run")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended {
		return fmt.Errorf("gimbal: check %q: scope %q ended", key, s.key)
	}
	if _, ok := s.values[key]; ok {
		return fmt.Errorf("gimbal: check result %q is already recorded in scope %q", key, s.key)
	}
	if s.values == nil {
		s.values = make(map[string]*scopeValue)
	}
	value := &scopeValue{owner: s, raw: append([]byte(nil), raw...)}
	if tokenCount("## "+key+"\n\n"+render(raw)) > contextEntryTokenLimit {
		if err := s.spillValueLocked(key, value); err != nil {
			return fmt.Errorf("gimbal: check %q: record result: %w", key, err)
		}
	}
	s.values[key] = value
	s.keys = append(s.keys, key)
	if err := s.run.eventResult(s.key, "", "", valueEvent(key, value)); err != nil {
		return fmt.Errorf("gimbal: check %q: record result: %w", key, err)
	}
	return nil
}

func runCommand(ctx context.Context, s *scope, name, workdir, command string, args []string) (CommandStarted, CommandEnded, error, error) {
	id, node := s.nextNode(runstore.Action, name)
	identity := actionIdentity(s.node, "", node, "process")
	ctx = effects.WithReconstruction(ctx, s.run.hasFact(node, "intent"))
	ctx, cancelAccepted := s.run.acceptedContext(ctx, node)
	defer cancelAccepted()
	ctx = effects.WithExecutionIdentity(effectContext(ctx, s, id), identity)
	dir, _ := filepath.Abs(workdir)
	started := CommandStarted{ID: id, Name: name, Command: command, Args: args, Workdir: dir}
	var accepted struct {
		Input struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
			Workdir string   `json:"workdir"`
		} `json:"input"`
	}
	if found, e := s.run.saved(node, "intent", &accepted); found && e == nil {
		command, args, workdir = accepted.Input.Command, accepted.Input.Args, accepted.Input.Workdir
		started.Command, started.Args, started.Workdir = command, args, workdir
	}

	var saved struct {
		ExitCode    int                         `json:"exit_code"`
		Interrupted bool                        `json:"interrupted"`
		Output      map[string]runstore.BlobRef `json:"output"`
	}
	if found, err := s.run.outcome(node, &saved); found {
		ended := CommandEnded{ID: id, ExitCode: saved.ExitCode, Interrupted: saved.Interrupted}
		for stream, ref := range saved.Output {
			reader, e := s.run.records.OpenBlob(context.Background(), ref)
			if e != nil {
				return started, ended, e, e
			}
			raw, e := io.ReadAll(reader)
			_ = reader.Close()
			if e != nil {
				return started, ended, e, e
			}
			if stream == "stdout" {
				ended.Stdout = string(raw)
			} else {
				ended.Stderr = string(raw)
			}
		}
		return started, ended, err, nil
	}
	intentErr := s.run.prepareAction(ctx, s.node, "", node, "process", map[string]any{"command": command, "args": args, "workdir": dir, "deadline": operationDeadline(ctx), "requires": effects.Requirements(ctx)}, identity)
	if intentErr != nil {
		return started, CommandEnded{ID: id, ExitCode: -1}, intentErr, intentErr
	}
	startRecordErr := s.run.eventResult(s.key, "", "", started)
	if startRecordErr != nil {
		return started, CommandEnded{ID: id, ExitCode: -1}, startRecordErr, startRecordErr
	}
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
			if effects.Messages(ctx) != nil {
				request, e := startMessageProcess(ctx, s, id, workdir, command, args, false)
				commandErr = e
				if e == nil {
					commandErr = messageProcessResult(ctx, s, request, out, errOut, &ended)
				} else {
					_ = out.close()
					_ = errOut.close()
				}
			} else if effects.Endpoint(ctx) != "" {
				commandErr = startRemoteProcess(ctx, s, id, workdir, command, args, out, errOut, false)
				if commandErr == nil {
					commandErr = remoteProcessResult(ctx, s, id, out, errOut, &ended)
				} else {
					_ = out.close()
					_ = errOut.close()
				}
			} else {
				_, _, commandErr = runCapturedCommand(ctx, id, workdir, command, args, s.run, out, errOut, &ended)
			}
		} else {
			_ = out.close()
			ended.StdoutFile = stdoutFile
		}
	}
	if captureErr != nil {
		commandErr = fmt.Errorf("gimbal: %s: capture output: %w", id, captureErr)
		s.run.recordFailure("capture command output "+id, captureErr)
	}
	ended.Error, ended.Duration = errString(commandErr), time.Since(start)
	outcomeErr := s.run.commandOutcome(s, ended)
	endRecordErr := errors.Join(outcomeErr, s.run.eventResult(s.key, "", "", ended))
	logf("%s: command ended after %s: exit %d: %v", id, ended.Duration.Round(time.Millisecond), ended.ExitCode, orNone(commandErr))
	return started, ended, commandErr, errors.Join(startRecordErr, endRecordErr)
}

func runCapturedCommand(ctx context.Context, id, workdir, command string, args []string, r *run, out, errOut *commandCapture, ended *CommandEnded) (stdout, stderr string, err error) {
	s, scopeErr := current(ctx)
	if scopeErr != nil {
		_ = out.close()
		_ = errOut.close()
		return "", "", scopeErr
	}
	environment, envFile, cleanup, envErr := localProcessEnvironment(ctx, s, false)
	if envErr != nil {
		_ = out.close()
		_ = errOut.close()
		return "", "", envErr
	}
	defer cleanup()
	if err = ctx.Err(); err != nil {
		_ = out.close()
		_ = errOut.close()
		return "", "", err
	}
	request := commandexecutor.Request{DiagnosticRun: filepath.Base(r.dir), Reconstructing: effects.Reconstructing(ctx), ID: executor.CommandID(effects.ExecutionIdentityFromContext(ctx).ExecutorOperation), Command: command, Args: args, Workdir: workdir, Environment: environment}
	if deadline, ok := ctx.Deadline(); ok {
		request.Deadline = deadline
	}
	descriptor, submitErr := commandexecutor.Submit(filepath.Join(r.dir, "executor-spool"), request)
	if submitErr != nil {
		_ = out.close()
		_ = errOut.close()
		return "", "", submitErr
	}
	stopProjection, projectionDone := projectCommandOutput(map[string]*commandCapture{descriptor.Stdout: out, descriptor.Stderr: errOut})
	result, waitErr := commandexecutor.Wait(ctx, descriptor)
	if ctx.Err() != nil {
		stopCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		stopErr := commandexecutor.Cancel(stopCtx, descriptor)
		result, waitErr = commandexecutor.Wait(stopCtx, descriptor)
		cancel()
		waitErr = errors.Join(ctx.Err(), stopErr, waitErr)
	}
	close(stopProjection)
	waitErr = errors.Join(waitErr, <-projectionDone, out.close(), errOut.close())
	excerpt := func(path string) (string, error) {
		info, e := os.Stat(path)
		if e != nil {
			return "", e
		}
		capture := commandCapture{size: info.Size()}
		relative, _ := filepath.Rel(r.dir, path)
		return capture.result(r.dir, relative)
	}
	stdout, stdoutErr := excerpt(out.file.Name())
	stderr, stderrErr := excerpt(errOut.file.Name())
	err = errors.Join(waitErr, stdoutErr, stderrErr)
	ended.ExitCode = result.ExitCode
	ended.Interrupted = result.Cancelled || result.TimedOut || ctx.Err() != nil
	if result.Error != "" && err == nil {
		err = errors.New(result.Error)
		if result.NotFound {
			err = errors.Join(exec.ErrNotFound, err)
		}
	}
	if err == nil && envFile != "" {
		if ended.ExitCode != 0 {
			err = fmt.Errorf("producer %s exited %d", id, ended.ExitCode)
		} else {
			err = publishLocalOutputs(ctx, s, envFile)
		}
		if err != nil {
			s.failService(err)
		}
	}
	ended.Stdout, ended.Stderr = stdout, stderr
	return stdout, stderr, err
}

// Output projections tail file-owned executor output with bounded buffers. The
// final drain completes before the authoritative outcome owns its blobs.
func projectCommandOutput(streams map[string]*commandCapture) (chan struct{}, chan error) {
	stop, done := make(chan struct{}), make(chan error, 1)
	go func() {
		ticker := time.NewTicker(20 * time.Millisecond)
		defer ticker.Stop()
		drain := func() error {
			var result error
			for path, capture := range streams {
				f, err := os.Open(path)
				if errors.Is(err, os.ErrNotExist) {
					continue
				}
				if err != nil {
					result = errors.Join(result, err)
					continue
				}
				capture.mu.Lock()
				offset := capture.size
				capture.mu.Unlock()
				info, err := f.Stat()
				if err == nil && info.Size() > offset {
					_, err = io.Copy(capture, io.NewSectionReader(f, offset, info.Size()-offset))
				}
				result = errors.Join(result, err, f.Close())
			}
			return result
		}
		var err error
		for {
			if e := drain(); err == nil {
				err = e
			}
			select {
			case <-stop:
				done <- errors.Join(err, drain())
				return
			case <-ticker.C:
			}
		}
	}()
	return stop, done
}
