package taskdaemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/tylergannon/gimbal/internal/commandexecutor"
	"github.com/tylergannon/gimbal/internal/effects"
	"github.com/tylergannon/gimbal/internal/envfile"
	"github.com/tylergannon/gimbal/internal/executor"
	"path/filepath"
)

func (d *Daemon) process(req effects.Process) error {
	if len(req.Yields) > 0 && d.Secrets == nil {
		return fmt.Errorf("producer requires a secrets backend")
	}
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
	executable := req.Command
	if strings.ContainsRune(executable, os.PathSeparator) && !filepath.IsAbs(executable) {
		executable = filepath.Join(d.workdir(req.Workdir), executable)
	}
	if _, err := exec.LookPath(executable); err != nil {
		return err
	}
	environment := os.Environ()
	environment = append(environment, "GIMBAL_DESKTOP_FIXED_WORKDIR="+d.Workdir)
	for name, value := range req.Environment {
		environment = append(environment, name+"="+value)
	}
	for name, value := range values {
		environment = append(environment, name+"="+value)
	}
	if producer {
		environment = append(environment, "ENV_FILE="+envPath)
	}
	root, err := executor.Root("")
	if err != nil {
		return err
	}
	request := commandexecutor.Request{ID: executor.NewCommandID(), Command: req.Command, Args: req.Args, Workdir: d.workdir(req.Workdir), Environment: environment, Timeout: 20 * time.Minute, Grace: 10 * time.Second}
	// The bridge records host-local identity; it never sends these paths to callers.
	if err = os.MkdirAll(filepath.Join(root, "submissions"), 0700); err != nil {
		return err
	}
	if err = executor.Write(filepath.Join(root, "submissions", string(request.ID)+".json"), request.ID); err != nil {
		return err
	}
	descriptor, err := commandexecutor.Submit(root, request)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	t := &task{owner: req.Owner, service: req.Service, started: time.Now(), cancel: cancel, done: make(chan struct{}), captures: map[string]string{"stdout": descriptor.Stdout, "stderr": descriptor.Stderr}}
	started = true
	t.stop = func(ctx context.Context) error {
		err := commandexecutor.Cancel(ctx, descriptor)
		result, waitErr := commandexecutor.Wait(ctx, descriptor)
		if result.Forced {
			waitErr = errors.Join(waitErr, errors.New(result.Error))
		}
		return errors.Join(err, waitErr)
	}

	d.tasks[id] = t
	go func() {
		saved, waitErr := commandexecutor.Wait(context.Background(), descriptor)
		result := effects.ProcessResult{ExitCode: saved.ExitCode, Interrupted: saved.Cancelled || saved.TimedOut, Error: saved.Error}
		if waitErr != nil {
			result.Error = waitErr.Error()
		}
		if result.Interrupted && !saved.Forced {
			result.Error = ""
		}

		if req.Service && !result.Interrupted {
			result.Error = fmt.Sprintf("required service %s exited unexpectedly: %s", req.Task, fmt.Sprintf("exit %d", result.ExitCode))
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
