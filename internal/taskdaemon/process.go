package taskdaemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/tylergannon/gimbal/internal/effects"
)

func (d *Daemon) process(req effects.Process) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.admit(req.Owner); err != nil {
		return err
	}
	id := key(req.Run, req.Task)
	if pending := d.admitting[id]; pending != nil && pending.admissionCtx.Err() != nil {
		return pending.admissionCtx.Err()
	}
	if d.tasks[id] != nil {
		return errors.New("command already started; read its result")
	}
	out, err := os.CreateTemp("", "gimbal-command-out-*")
	if err != nil {
		return err
	}
	stderr, err := os.CreateTemp("", "gimbal-command-err-*")
	if err != nil {
		_ = out.Close()
		_ = os.Remove(out.Name())
		return err
	}
	cmd := exec.Command(req.Command, req.Args...)
	cmd.Dir = d.workdir(req.Workdir)
	cmd.Env = os.Environ()
	for _, binding := range req.Bindings {
		cmd.Env = append(cmd.Env, binding.EnvironmentName()+"="+binding.URL)
	}
	cmd.Stdout = out
	cmd.Stderr = stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		_ = out.Close()
		_ = stderr.Close()
		_ = os.Remove(out.Name())
		_ = os.Remove(stderr.Name())
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	t := &task{owner: req.Owner, cancel: cancel, done: make(chan struct{}), captures: map[string]string{"stdout": out.Name(), "stderr": stderr.Name()}}
	var stopMu sync.Mutex
	t.stop = func(ctx context.Context) error {
		stopMu.Lock()
		defer stopMu.Unlock()
		signal := func(sig syscall.Signal) error {
			err := syscall.Kill(-cmd.Process.Pid, sig)
			if errors.Is(err, syscall.ESRCH) {
				return nil
			}
			return err
		}
		if err := signal(syscall.SIGTERM); err != nil {
			return err
		}
		grace := time.NewTimer(5 * time.Second)
		defer grace.Stop()
		ticker := time.NewTicker(20 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-t.done:
				if errors.Is(syscall.Kill(-cmd.Process.Pid, 0), syscall.ESRCH) {
					return nil
				}
			default:
			}
			select {
			case <-ctx.Done():
				return fmt.Errorf("process group cleanup unconfirmed: %w", ctx.Err())
			case <-grace.C:
				if err := signal(syscall.SIGKILL); err != nil {
					return err
				}
			case <-ticker.C:
			}
		}
	}
	d.tasks[id] = t
	go func() {
		waitErr := cmd.Wait()
		captureErr := errors.Join(out.Close(), stderr.Close())
		result := effects.ProcessResult{ExitCode: cmd.ProcessState.ExitCode(), Interrupted: ctx.Err() != nil}
		if captureErr != nil {
			result.Error = captureErr.Error()
		} else if waitErr != nil {
			if _, ok := errors.AsType[*exec.ExitError](waitErr); !ok {
				result.Error = waitErr.Error()
			}
		}
		if req.Service && !result.Interrupted {
			result.Error = fmt.Sprintf("required service %s exited unexpectedly: %s", req.Task, cmd.ProcessState.String())
		}
		t.mu.Lock()
		t.result, _ = json.Marshal(result)
		t.mu.Unlock()
		t.mu.Lock()
		t.completed = time.Now()
		t.mu.Unlock()
		close(t.done)
	}()
	go func() {
		select {
		case <-ctx.Done():
			stopCtx, c := context.WithTimeout(context.Background(), 10*time.Second)
			defer c()
			_ = t.stop(stopCtx)
		case <-t.done:
		}
	}()
	return nil
}
