package gimbal

import (
	"context"
	"errors"
	"fmt"
	"github.com/tylergannon/gimbal/internal/processdebug"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/tylergannon/gimbal/internal/effects"
	"github.com/tylergannon/gimbal/runstore"
)

var (
	serviceShutdownGrace = 10 * time.Second
	serviceKillWait      = 5 * time.Second
)

const servicePollInterval = 10 * time.Millisecond

var errServiceShutdownTimeout = errors.New("service process group did not stop before the deadline")

// Service starts command through zsh and makes the current scope own it. It
// returns after the service atomically publishes ENV_FILE, declared outputs
// validate, and the secrets backend confirms publication. An empty Yields
// declaration still requires an empty readiness file.
//
// A service is required for its scope's remaining lifetime. Any exit before
// that scope starts shutting down, including exit zero, fails and cancels the
// scope. When the scope ends or its ctx is cancelled, Gimbal sends SIGINT to
// the service's process group and escalates to SIGKILL when necessary.
// This owns descendants which remain in that group;
// a command must stay in the foreground and must not daemonize or escape it.
// Stdout, stderr, status, and failures are recorded with the scope's commands.
//
// name is the service's constant name for the graph and run record. An empty
// workdir uses the workflow process's working directory.
func Service(ctx context.Context, name, workdir, command string) (result error) {
	scope, err := current(ctx)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("gimbal: service %q: %w", name, err)
	}

	id, node := scope.nextNode(runstore.Service, name)
	var admitted struct {
		Error string `json:"error"`
	}
	if found, err := scope.run.saved(node, "service-admitted", &admitted); found {
		if err != nil {
			return err
		}
		return stringError(admitted.Error)
	}
	if scope.run.hasFact(node, "intent") {
		return fmt.Errorf("gimbal: service %s admission unresolved", id)
	}
	defer func() {
		result = errors.Join(result, scope.run.recordFact(node, scope.node, "", "service-admitted", struct {
			Error string `json:"error"`
		}{errString(result)}, runstore.Committed))
	}()

	identity := actionIdentity(scope.node, "", node, "process")
	if effects.Endpoint(ctx) == "" {
		identity.ExecutorOperation = ""
	}
	ctx = effects.WithExecutionIdentity(effectContext(ctx, scope, id), identity)
	dir, _ := filepath.Abs(workdir)
	if err := scope.run.prepareAction(ctx, scope.node, "", node, "service", map[string]any{"command": command, "workdir": dir, "requires": effects.Requirements(ctx)}, identity); err != nil {
		return err
	}
	started := CommandStarted{ID: id, Name: name, Command: "zsh", Args: []string{"-c", command}, Workdir: dir}
	if err := scope.run.eventResult(scope.key, "", "", started); err != nil {
		return fmt.Errorf("gimbal: service %s: record start: %w", id, err)
	}
	logf("%s: service starting through zsh: %s", id, oneLine(command))
	start := time.Now()
	ended := CommandEnded{ID: id, ExitCode: -1}

	out, stdoutFile, err := openCommandCapture(scope.run, id, "stdout")
	if err != nil {
		scope.run.recordFailure("capture service output "+id, err)
		return endServiceStartFailure(scope, start, ended, fmt.Errorf("gimbal: service %s: capture output: %w", id, err))
	}
	ended.StdoutFile = stdoutFile
	errOut, stderrFile, err := openCommandCapture(scope.run, id, "stderr")
	if err != nil {
		_ = out.close()
		scope.run.recordFailure("capture service output "+id, err)
		return endServiceStartFailure(scope, start, ended, fmt.Errorf("gimbal: service %s: capture output: %w", id, err))
	}
	ended.StderrFile = stderrFile

	if effects.Endpoint(ctx) != "" {
		return remoteService(ctx, scope, name, id, workdir, command, out, errOut, ended, start)
	}

	environment, envFile, cleanup, envErr := localProcessEnvironment(ctx, scope, true)
	if envErr != nil {
		_ = out.close()
		_ = errOut.close()
		return endServiceStartFailure(scope, start, ended, envErr)
	}
	startedNative := false
	defer func() {
		if !startedNative {
			cleanup()
		}
	}()
	cmd := exec.Command("zsh", "-c", command)
	cmd.Env = environment
	cmd.Dir = workdir
	cmd.Stdout, cmd.Stderr = out, errOut
	cmd.WaitDelay = commandWaitDelay
	if err := prepareServiceProcess(cmd); err != nil {
		_ = out.close()
		_ = errOut.close()
		return endServiceStartFailure(scope, start, ended, fmt.Errorf("gimbal: service %s: %w", id, err))
	}
	if err := cmd.Start(); err != nil {
		_ = out.close()
		_ = errOut.close()
		return endServiceStartFailure(scope, start, ended, fmt.Errorf("gimbal: service %s: %w", id, err))
	}

	service := &ownedService{
		scope: scope, ctx: scope.ctx, id: id, name: name, cmd: cmd,
		out: out, errOut: errOut, ended: ended, startedAt: start, cleanup: cleanup,
		done: make(chan struct{}), observation: processdebug.Start("service", id, cmd.Process.Pid),
	}
	startedNative = true
	if err := scope.adoptService(service); err != nil {
		go service.wait()
		service.beginStop()
		return errors.Join(err, service.stop())
	}
	go service.wait()
	if err := scope.run.nativeBinding(scope.node, "", node, fmt.Sprintf("pid:%d", cmd.Process.Pid)); err != nil {
		return err
	}
	ticker := time.NewTicker(servicePollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-service.done:
			return fmt.Errorf("service %s exited before readiness", id)
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if _, err := os.Stat(envFile); errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err := publishLocalOutputs(ctx, scope, envFile); err != nil {
				scope.failService(err)
				return err
			}
			select {
			case <-service.done:
				return fmt.Errorf("service %s exited during readiness", id)
			default:
			}
			scope.run.event(scope.key, "", "", ResourceReady{Name: name, ConsumerKind: "service", Consumer: id})
			return nil
		}
	}
}

func endServiceStartFailure(scope *scope, start time.Time, ended CommandEnded, serviceErr error) error {
	ended.Error = serviceErr.Error()
	ended.Duration = time.Since(start)
	recordErr := errors.Join(scope.run.commandOutcome(scope, ended), scope.run.eventResult(scope.key, "", "", ended))
	logf("%s: service failed to start after %s: %v", ended.ID, ended.Duration.Round(time.Millisecond), serviceErr)
	return errors.Join(serviceErr, recordErr)
}

type ownedService struct {
	observation *processdebug.Owner
	scope       *scope
	ctx         context.Context
	id          string
	name        string
	cmd         *exec.Cmd
	out         *commandCapture
	errOut      *commandCapture
	ended       CommandEnded

	startedAt    time.Time
	done         chan struct{}
	cleanup      func()
	stopOnce     sync.Once
	resultCancel context.CancelFunc
	remote       bool

	mu          sync.Mutex
	stopping    bool
	terminalErr error
	stopErr     error
}

func (s *ownedService) beginStop() {
	s.observation.End()
	s.mu.Lock()
	s.stopping = true
	s.mu.Unlock()
}

func (s *ownedService) wait() {
	defer s.cleanup()
	waitErr := s.cmd.Wait()
	s.observation.Reaped()
	captureErr := errors.Join(s.out.close(), s.errOut.close())
	stdout, stdoutErr := s.out.result(s.scope.run.dir, s.ended.StdoutFile)
	stderr, stderrErr := s.errOut.result(s.scope.run.dir, s.ended.StderrFile)
	captureErr = errors.Join(captureErr, stdoutErr, stderrErr)
	if captureErr != nil {
		s.scope.run.recordFailure("capture service output "+s.id, captureErr)
	}

	s.mu.Lock()
	stopping := s.stopping
	s.ended.Stdout, s.ended.Stderr = stdout, stderr
	s.ended.Duration = time.Since(s.startedAt)
	if s.cmd.ProcessState != nil {
		s.ended.ExitCode = s.cmd.ProcessState.ExitCode()
	}
	var serviceErr error
	if stopping {
		s.ended.Interrupted = true
		if captureErr != nil {
			serviceErr = fmt.Errorf("gimbal: service %s: capture output: %w", s.id, captureErr)
		} else if errors.Is(waitErr, exec.ErrWaitDelay) {
			serviceErr = fmt.Errorf("gimbal: service %s: output remained open after its process exited: %w", s.id, waitErr)
		}
	} else {
		status := "without a process status"
		if s.cmd.ProcessState != nil {
			status = s.cmd.ProcessState.String()
		} else if waitErr != nil {
			status = waitErr.Error()
		}
		serviceErr = fmt.Errorf("gimbal: required service %s exited unexpectedly: %s", s.id, status)
		if captureErr != nil {
			serviceErr = errors.Join(serviceErr, fmt.Errorf("capture output: %w", captureErr))
		}
	}
	if stopping {
		s.terminalErr = serviceErr
	}
	s.ended.Error = errString(serviceErr)
	ended := s.ended
	s.mu.Unlock()

	if !stopping {
		// Install the already-classified failure before persistence, which
		// may block while the owning scope begins to close. Once wait observed
		// the exit first, shutdown cannot relabel it as intentional.
		s.scope.failService(serviceErr)
	}
	if err := errors.Join(s.scope.run.commandOutcome(s.scope, ended), s.scope.run.eventResult(s.scope.key, "", "", ended)); err != nil {
		s.scope.run.recordFailure("record service end "+s.id, err)
	}
	logf("%s: service ended after %s: exit %d: %v", s.id, ended.Duration.Round(time.Millisecond), ended.ExitCode, orNone(serviceErr))
	s.observation.End()
	close(s.done)
}

func (s *ownedService) stop() error {
	if s.remote {
		select {
		case <-s.done:
			s.mu.Lock()
			defer s.mu.Unlock()
			return s.terminalErr
		case <-time.After(10 * time.Second):
			if s.resultCancel != nil {
				s.resultCancel()
			}
			return fmt.Errorf("service %s final result unavailable", s.id)
		}
	}
	s.stopOnce.Do(func() {
		s.beginStop()
		termErr := terminateServiceGroup(s.cmd.Process.Pid)
		waitErr := s.awaitBoundary(serviceShutdownGrace)
		if waitErr != nil {
			killErr := killServiceGroup(s.cmd.Process.Pid)
			finalErr := s.awaitBoundary(serviceKillWait)
			s.stopErr = errors.Join(termErr, waitErr, killErr, finalErr)
		} else {
			s.stopErr = termErr
		}
		s.mu.Lock()
		s.stopErr = errors.Join(s.stopErr, s.terminalErr)
		s.mu.Unlock()
		if s.stopErr != nil {
			s.stopErr = fmt.Errorf("gimbal: stop service %s: %w", s.id, s.stopErr)
		}
	})
	return s.stopErr
}

func (s *ownedService) awaitBoundary(timeout time.Duration) error {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	ticker := time.NewTicker(servicePollInterval)
	defer ticker.Stop()
	done := s.done
	directExited := false
	var observationErr error
	for {
		if !directExited {
			select {
			case <-done:
				directExited = true
				done = nil
			default:
			}
		}
		alive, err := serviceGroupAlive(s.cmd.Process.Pid)
		if err != nil {
			observationErr = err
		}
		if directExited && !alive && err == nil {
			return nil
		}
		select {
		case <-done:
			directExited = true
			done = nil
		case <-ticker.C:
		case <-timer.C:
			parts := []string{}
			if !directExited {
				parts = append(parts, "direct process was not reaped")
			}
			if alive {
				parts = append(parts, "process group still exists")
			}
			return errors.Join(fmt.Errorf("%w: %s", errServiceShutdownTimeout, strings.Join(parts, "; ")), observationErr)
		}
	}
}
