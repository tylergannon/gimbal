package gimbal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/tylergannon/gimbal/backend"

	"github.com/tylergannon/gimbal/internal/effects"
)

func effectOwner(s *scope, task string) effects.Owner {
	return effects.Owner{Run: filepath.Base(s.run.dir), Scope: s.key, Task: task}
}
func effectContext(ctx context.Context, s *scope, task string) context.Context {
	ctx = effects.WithOwner(ctx, effectOwner(s, task))
	return effects.WithResourceObserver(ctx, func(use backend.ResourceFact) {
		if s == nil || s.run == nil {
			return
		}
		event := ResourceReady{Name: use.Name, ConsumerKind: use.Kind, Consumer: use.Consumer}
		if err := s.run.eventResult(use.Scope.Scope, "", "", event); err != nil {
			logf("%s: resource recording unavailable: %v", task, err)
		}
	})
}
func (s *scope) remoteCleanup() error {
	if s.run.scopeCompleted(s.node) {
		return nil
	}
	endpoint := effects.Endpoint(s.ctx)
	if endpoint == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(s.ctx), 30*time.Second)
	defer cancel()
	return effects.Cleanup(ctx, endpoint, filepath.Base(s.run.dir), s.key, backend.NewExecutionIdentity("control").RequestID)
}
func startRemoteProcess(ctx context.Context, s *scope, id, workdir, command string, args []string, out, stderr *commandCapture, service bool) error {
	store, yields, produces := effects.Yields(ctx)
	var reply effects.Reply
	err := effects.Call(ctx, effects.Endpoint(ctx), "identified-process", effects.Process{ExecutionIdentity: effects.ExecutionIdentityFromContext(ctx), Deadline: operationDeadline(ctx), Owner: effectOwner(s, id), Workdir: workdir, Command: command, Args: args, Service: service, Requires: effects.Requirements(ctx), Store: store, Yields: yields, Produces: produces}, &reply)
	if err != nil {
		return effects.ConfirmStarted(ctx, effects.Endpoint(ctx), effects.Target{Run: filepath.Base(s.run.dir), ID: id}, err)
	}
	if err == nil && reply.Error != "" {
		err = errors.New(reply.Error)
	}
	return err
}
func remoteProcessResult(ctx context.Context, s *scope, id string, out, stderr *commandCapture, ended *CommandEnded) error {
	raw, err := effects.Wait(ctx, effects.Endpoint(ctx), effects.Target{Run: filepath.Base(s.run.dir), ID: id}, nil)
	var result effects.ProcessResult
	if err == nil {
		err = json.Unmarshal(raw, &result)
		ended.ExitCode = result.ExitCode
		ended.Interrupted = result.Interrupted
		if err == nil && result.Error != "" {
			err = errors.New(result.Error)
		}
	}
	if ctx.Err() != nil {
		ended.Interrupted = true
	}
	var captureErr error
	for stream, c := range map[string]*commandCapture{"stdout": out, "stderr": stderr} {
		reader, e := executionFor(ctx).ReadBytes(ctx, backend.Artifact{Owner: effectOwner(s, id), Name: stream})
		if e == nil {
			_, e = io.Copy(c, reader)
			e = errors.Join(e, reader.Close())
		}
		captureErr = errors.Join(captureErr, e, c.close())
	}
	var e error
	ended.Stdout, e = out.result(s.run.dir, ended.StdoutFile)
	captureErr = errors.Join(captureErr, e)
	ended.Stderr, e = stderr.result(s.run.dir, ended.StderrFile)
	captureErr = errors.Join(captureErr, e)
	return errors.Join(err, captureErr)
}
func remoteService(ctx context.Context, s *scope, name, id, workdir, command string, out, stderr *commandCapture, ended CommandEnded, start time.Time) error {
	if effects.Messages(ctx) != nil {
		return messageService(ctx, s, name, id, workdir, command, out, stderr, ended, start)
	}
	if err := startRemoteProcess(ctx, s, id, workdir, "zsh", []string{"-c", command}, out, stderr, true); err != nil {
		_ = out.close()
		_ = stderr.close()
		return endServiceStartFailure(s, start, ended, err)
	}
	return s.collectRemoteService(ctx, name, id, out, stderr, ended, start, func(ctx context.Context, ended *CommandEnded) error {
		return remoteProcessResult(effects.ObserveDetached(ctx), s, id, out, stderr, ended)
	})
}

func executionFor(ctx context.Context) backend.Execution {
	if c := effects.Connection(ctx); c.Execution != nil {
		return c.Execution
	}
	return backend.HTTP{Endpoint: effects.Endpoint(ctx)}
}

func startMessageProcess(ctx context.Context, s *scope, id, workdir, command string, args []string, service bool) (backend.MessageRequest, error) {
	identity := effects.ExecutionIdentityFromContext(ctx)
	r := effects.Request(identity.RequestID, "identified-process", effectOwner(s, id))
	r.ExecutionIdentity = identity
	store, yields, produces := effects.Yields(ctx)
	r.Process = &backend.Process{ExecutionIdentity: identity, Deadline: operationDeadline(ctx), Owner: r.Owner, Workdir: workdir, Command: command, Args: args, Service: service, Requires: effects.Requirements(ctx), Store: store, Yields: yields, Produces: produces}
	return r, effects.Submit(ctx, r)
}
func messageProcessResult(ctx context.Context, s *scope, r backend.MessageRequest, out, stderr *commandCapture, ended *CommandEnded) error {
	event, err := effects.Await(effectContext(ctx, s, r.Task), r, "completed", nil)
	if ctx.Err() != nil {
		ended.Interrupted = true
		err = errors.Join(err, effects.InterruptMessage(ctx, r))
	}
	if event.Result != nil {
		result := event.Result
		if result.Process != nil {
			ended.ExitCode = result.Process.ExitCode
			ended.Interrupted = result.Process.Interrupted
			if result.Process.Error != "" {
				err = errors.Join(err, errors.New(result.Process.Error))
			}
		}
		_, e := out.Write(result.Stdout)
		err = errors.Join(err, e)
		_, e = stderr.Write(result.Stderr)
		err = errors.Join(err, e)
	}
	err = errors.Join(err, out.close(), stderr.close())
	var e error
	ended.Stdout, e = out.result(s.run.dir, ended.StdoutFile)
	err = errors.Join(err, e)
	ended.Stderr, e = stderr.result(s.run.dir, ended.StderrFile)
	return errors.Join(err, e)
}
func messageService(ctx context.Context, s *scope, name, id, workdir, command string, out, stderr *commandCapture, ended CommandEnded, start time.Time) error {
	r, err := startMessageProcess(ctx, s, id, workdir, "zsh", []string{"-c", command}, true)
	if err == nil {
		_, err = effects.Await(effectContext(ctx, s, r.Task), r, "established", nil)
	}
	if err != nil {
		_ = out.close()
		_ = stderr.close()
		return endServiceStartFailure(s, start, ended, err)
	}
	return s.collectRemoteService(ctx, name, id, out, stderr, ended, start, func(ctx context.Context, ended *CommandEnded) error {
		return messageProcessResult(ctx, s, r, out, stderr, ended)
	})
}

// collectRemoteService gives both transports the same local service owner.
// Its result reader survives scope cancellation so normal scope exit can collect
// the final command result after the remote cleanup request.
func (s *scope) collectRemoteService(ctx context.Context, name, id string, out, stderr *commandCapture, ended CommandEnded, start time.Time, result func(context.Context, *CommandEnded) error) error {
	resultCtx, resultCancel := context.WithCancel(context.WithoutCancel(ctx))
	service := &ownedService{scope: s, ctx: s.ctx, id: id, name: name, done: make(chan struct{}), remote: true, resultCancel: resultCancel}
	if err := s.adoptService(service); err != nil {
		resultCancel()
		_ = out.close()
		_ = stderr.close()
		return errors.Join(err, s.remoteCleanup())
	}
	go func() {
		defer resultCancel()
		err := result(resultCtx, &ended)
		ended.Duration = time.Since(start)
		ended.Error = errString(err)
		service.mu.Lock()
		stopping := service.stopping
		service.terminalErr = err
		service.mu.Unlock()
		if !stopping {
			if err == nil {
				err = fmt.Errorf("required service %s exited unexpectedly", id)
			}
			s.failService(err)
		}
		_ = s.run.commandOutcome(s, ended)
		s.run.event(s.key, "", "", ended)
		close(service.done)
	}()
	return nil
}

func operationDeadline(ctx context.Context) time.Time { deadline, _ := ctx.Deadline(); return deadline }
