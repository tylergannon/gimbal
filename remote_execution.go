package gimbal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

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
	ctx, cancel := context.WithTimeout(context.Background(), closeTimeout+time.Second)
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
	err := effects.Call(ctx, effects.Endpoint(ctx), "process", effects.Process{Owner: effectOwner(s, id), Workdir: workdir, Command: command, Args: args, Stdout: out.file.Name(), Stderr: stderr.file.Name(), Service: service}, &reply)
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
	captureErr := errors.Join(out.close(), stderr.close())
	// The daemon writes directly to these explicitly shared capture files.
	for _, c := range []*commandCapture{out, stderr} {
		if info, e := os.Stat(c.file.Name()); e == nil {
			c.size = info.Size()
		} else {
			captureErr = errors.Join(captureErr, e)
		}
	}
	var e error
	ended.Stdout, e = out.result(s.run.dir, ended.StdoutFile)
	captureErr = errors.Join(captureErr, e)
	ended.Stderr, e = stderr.result(s.run.dir, ended.StderrFile)
	captureErr = errors.Join(captureErr, e)
	return errors.Join(err, captureErr)
}
func remoteService(ctx context.Context, s *scope, name, id, workdir, command string, out, stderr *commandCapture, ended CommandEnded, start time.Time) error {
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
