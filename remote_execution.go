package gimbal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/tylergannon/gimbal/backend"

	"github.com/tylergannon/gimbal/internal/effects"
)

func effectOwner(s *scope, task string) effects.Owner {
	return effects.Owner{Run: filepath.Base(s.run.dir), Scope: s.key, Task: task}
}
func effectContext(ctx context.Context, s *scope, task string) context.Context {
	return effects.WithOwner(ctx, effectOwner(s, task))
}
func (s *scope) remoteCleanup() error {
	endpoint := effects.Endpoint(s.ctx)
	if endpoint == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(s.ctx), closeTimeout+time.Second)
	defer cancel()
	err := effects.Cleanup(ctx, endpoint, filepath.Base(s.run.dir), s.key)
	s.run.closeMu.Lock()
	if err != nil {
		if s.run.pendingScopes == nil {
			s.run.pendingScopes = map[string]*scope{}
		}
		s.run.pendingScopes[s.key] = s
	} else {
		for key := range s.run.pendingScopes {
			if s.key == "" || key == s.key || strings.HasPrefix(key, s.key+"/") {
				delete(s.run.pendingScopes, key)
			}
		}
	}
	s.run.closeMu.Unlock()
	return err
}
func startRemoteProcess(ctx context.Context, s *scope, id, workdir, command string, args []string, out, stderr *commandCapture, service bool) error {
	var reply effects.Reply
	err := effects.Call(ctx, effects.Endpoint(ctx), "process", effects.Process{Owner: effectOwner(s, id), Workdir: workdir, Command: command, Args: args, Service: service}, &reply)
	if err != nil {
		return effects.ConfirmStarted(ctx, effects.Endpoint(ctx), effects.Target{Run: filepath.Base(s.run.dir), ID: id})
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
	var ack effects.Reply
	captureErr = errors.Join(captureErr, effects.Call(ctx, effects.Endpoint(ctx), "ack", effects.Target{Run: filepath.Base(s.run.dir), ID: id}, &ack))
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
	service := &ownedService{scope: s, ctx: s.ctx, id: id, name: name, done: make(chan struct{}), remote: true}
	if err := s.adoptService(service); err != nil {
		return errors.Join(err, s.remoteCleanup())
	}
	go func() {
		// Poll independently of the scope cancellation: cleanup owns the stop and
		// the final result must remain observable while the scope unwinds.
		err := remoteProcessResult(context.WithoutCancel(ctx), s, id, out, stderr, &ended)
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
		s.run.event(s.key, "", "", ended)
		close(service.done)
	}()
	return nil
}

func executionFor(ctx context.Context) backend.Execution {
	if c := effects.Connection(ctx); c.Execution != nil {
		return c.Execution
	}
	return backend.HTTP{Endpoint: effects.Endpoint(ctx)}
}

func startMessageProcess(ctx context.Context, s *scope, id, workdir, command string, args []string, service bool) (backend.MessageRequest, error) {
	r := effects.Request("process", effectOwner(s, id))
	r.Process = &backend.Process{Owner: r.Owner, Workdir: workdir, Command: command, Args: args, Service: service}
	return r, effects.Submit(ctx, r)
}
func messageProcessResult(ctx context.Context, s *scope, r backend.MessageRequest, out, stderr *commandCapture, ended *CommandEnded) error {
	event, err := effects.Await(ctx, r, "completed", nil)
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
		_, err = effects.Await(ctx, r, "established", nil)
	}
	if err != nil {
		_ = out.close()
		_ = stderr.close()
		return endServiceStartFailure(s, start, ended, err)
	}
	service := &ownedService{scope: s, ctx: s.ctx, id: id, name: name, done: make(chan struct{}), remote: true}
	if err = s.adoptService(service); err != nil {
		return errors.Join(err, s.remoteCleanup())
	}
	go func() {
		err := messageProcessResult(context.WithoutCancel(ctx), s, r, out, stderr, &ended)
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
		s.run.event(s.key, "", "", ended)
		close(service.done)
	}()
	return nil
}
