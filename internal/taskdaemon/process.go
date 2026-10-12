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
	"github.com/tylergannon/gimbal/internal/envfile"
)

func (d *Daemon) process(req effects.Process) error {
	values, finish, err := d.acquire(context.Background(), req.Owner, req.Requires)
	var finishOnce sync.Once
	finishAdmission := func() { finishOnce.Do(finish) }
	defer finishAdmission()
	if err != nil {
		return err
	}
	producer := req.Service || req.Produces || len(req.Yields) > 0
	envPath := ""
	cleanEnv := func() {}
	if producer {
		envPath, cleanEnv, err = envfile.Prepare()
		if err != nil {
			return err
		}
	}
	started := false
	defer func() {
		if !started {
			cleanEnv()
		}
	}()
	d.mu.Lock()
	locked := true
	defer func() {
		if locked {
			d.mu.Unlock()
		}
	}()
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
	for name, value := range req.Environment {
		cmd.Env = append(cmd.Env, name+"="+value)
	}
	for name, value := range values {
		cmd.Env = append(cmd.Env, name+"="+value)
	}
	if producer {
		cmd.Env = append(cmd.Env, "ENV_FILE="+envPath)
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
	t := &task{owner: req.Owner, service: req.Service, started: time.Now(), cancel: cancel, done: make(chan struct{}), captures: map[string]string{"stdout": out.Name(), "stderr": stderr.Name()}}
	started = true
	nativeDone := make(chan struct{})
	var stopMu sync.Mutex
	var stopFailure error
	t.stop = func(ctx context.Context) error {
		stopMu.Lock()
		defer stopMu.Unlock()
		if stopFailure != nil {
			return stopFailure
		}
		signal := func(sig syscall.Signal) error {
			err := syscall.Kill(-cmd.Process.Pid, sig)
			if errors.Is(err, syscall.ESRCH) {
				return nil
			}
			return err
		}
		if err := signal(syscall.SIGINT); err != nil {
			return err
		}
		grace := time.NewTimer(10 * time.Second)
		defer grace.Stop()
		ticker := time.NewTicker(20 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-nativeDone:
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
				stopFailure = fmt.Errorf("process exceeded SIGINT cleanup grace; external resource cleanup unconfirmed")
				return stopFailure
			case <-ticker.C:
			}
		}
	}
	d.tasks[id] = t
	go func() {
		waitErr := cmd.Wait()
		close(nativeDone)
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
		if producer && !req.Service && result.ExitCode == 0 && !result.Interrupted && result.Error == "" {
			if err := d.publishProducer(ctx, req, envPath); err != nil {
				result.Error = err.Error()
			}
		}
		cleanEnv()
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
			stopCtx, c := context.WithTimeout(context.Background(), 12*time.Second)
			defer c()
			_ = t.stop(stopCtx)
		case <-t.done:
		}
	}()
	d.mu.Unlock()
	locked = false
	finishAdmission()
	if !req.Service {
		return nil
	}
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.done:
			return fmt.Errorf("service exited before readiness")
		case <-ticker.C:
			if _, err := os.Stat(envPath); os.IsNotExist(err) {
				continue
			} else if err != nil {
				cancel()
				return err
			}
			err := d.publishProducer(ctx, req, envPath)
			if err != nil {
				cancel()
				return fmt.Errorf("service readiness publication: %w", err)
			}
			select {
			case <-t.done:
				return fmt.Errorf("service exited during readiness")
			default:
			}
			return nil
		}
	}
}
