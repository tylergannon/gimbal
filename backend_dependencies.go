package gimbal

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sync"
	"syscall"
	"time"

	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/backend/secretfiles"
)

// LocalBackends configures private shared stores and named daemon commands.
// Commands run gimbal message-worker; readiness and secrets flags are appended.
// Each environment's Route is its messaging configuration, not an execution URL.
type LocalBackends struct {
	Directory    string
	Environments map[string]EnvironmentCommand
}
type EnvironmentCommand struct {
	Command []string
	Route   backend.Route
}
type environmentUse struct {
	PID       int
	ReadyFile string
	Owners    []backend.Owner
}
type localOrchestrator struct {
	config LocalBackends
	mu     sync.Mutex
}

var localOrchestrators sync.Map

func (r *Registry) constructLocalBackends(raw json.RawMessage) (backend.Dependencies, error) {
	var c LocalBackends
	if err := json.Unmarshal(raw, &c); err != nil {
		return backend.Dependencies{}, err
	}
	if !filepath.IsAbs(c.Directory) {
		return backend.Dependencies{}, fmt.Errorf("backend Directory must be absolute")
	}
	secrets, err := secretfiles.Open(filepath.Join(c.Directory, "secrets"))
	if err != nil {
		return backend.Dependencies{}, err
	}
	value, _ := localOrchestrators.LoadOrStore(c.Directory, &localOrchestrator{config: c})
	controller := value.(*localOrchestrator)
	dependencies := backend.Dependencies{Orchestrator: controller, Secrets: secrets}
	dependencies.WrapMessaging = func(ctx context.Context, base backend.Messaging) (backend.Messaging, error) {
		return r.connectEnvironments(ctx, base, controller, dependencies)
	}
	return dependencies, nil
}
func (o *localOrchestrator) file(name string) string {
	sum := sha256.Sum256([]byte(name))
	return filepath.Join(o.config.Directory, "environment-"+hex.EncodeToString(sum[:8])+".json")
}
func (o *localOrchestrator) load(name string) (environmentUse, error) {
	var use environmentUse
	raw, err := os.ReadFile(o.file(name))
	if os.IsNotExist(err) {
		return use, nil
	}
	if err != nil {
		return use, err
	}
	err = json.Unmarshal(raw, &use)
	return use, err
}
func (o *localOrchestrator) save(name string, use environmentUse) error {
	raw, err := json.Marshal(use)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(o.config.Directory, ".environment-")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err = tmp.Write(raw); err == nil {
		err = tmp.Sync()
	}
	closeErr := tmp.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(tmp.Name(), o.file(name))
}

// Lock only receipt mutation: independent runner processes share ownership.
func (o *localOrchestrator) lock(ctx context.Context) (func(), error) {
	file, err := os.OpenFile(filepath.Join(o.config.Directory, "environments.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	for {
		err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return func() { _ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN); _ = file.Close() }, nil
		}
		if err != syscall.EWOULDBLOCK {
			_ = file.Close()
			return nil, err
		}
		select {
		case <-ctx.Done():
			_ = file.Close()
			return nil, ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
}
func (o *localOrchestrator) AcquireEnvironment(ctx context.Context, owner backend.Owner, name string) (backend.Environment, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	unlock, lockErr := o.lock(ctx)
	if lockErr != nil {
		return backend.Environment{}, lockErr
	}
	defer unlock()
	definition, ok := o.config.Environments[name]
	if !ok {
		return backend.Environment{}, fmt.Errorf("unknown environment %q", name)
	}
	use, err := o.load(name)
	if err != nil {
		return backend.Environment{}, err
	}
	if use.PID != 0 {
		if err := syscall.Kill(use.PID, 0); err != nil {
			return backend.Environment{}, fmt.Errorf("environment %q daemon stopped; cleanup required", name)
		}
	} else {
		if len(definition.Command) == 0 {
			return backend.Environment{}, fmt.Errorf("environment %q needs a daemon command", name)
		}
		ready, err := os.CreateTemp(o.config.Directory, ".ready-")
		if err != nil {
			return backend.Environment{}, err
		}
		use.ReadyFile = ready.Name()
		_ = ready.Close()
		_ = os.Remove(use.ReadyFile)
		args := append([]string{}, definition.Command[1:]...)
		args = append(args, "--ready-file", use.ReadyFile, "--secrets-directory", filepath.Join(o.config.Directory, "secrets"))
		cmd := exec.Command(definition.Command[0], args...)
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		log, err := os.OpenFile(o.file(name)+".log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			return backend.Environment{}, err
		}
		defer func() { _ = log.Close() }()
		cmd.Stdout = log
		cmd.Stderr = log
		if err := cmd.Start(); err != nil {
			return backend.Environment{}, err
		}
		use.PID = cmd.Process.Pid
		use.Owners = append(use.Owners, owner)
		if err := o.save(name, use); err != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			return backend.Environment{}, err
		}
		go func() { _ = cmd.Wait() }()
	}
	found := slices.Contains(use.Owners, owner)
	if !found {
		use.Owners = append(use.Owners, owner)
		if err := o.save(name, use); err != nil {
			return backend.Environment{}, err
		}
	}
	deadline := time.NewTimer(30 * time.Second)
	defer deadline.Stop()
	for {
		if _, err := os.Stat(use.ReadyFile); err == nil {
			return backend.Environment{ID: name}, nil
		}
		if err := syscall.Kill(use.PID, 0); err != nil {
			return backend.Environment{}, fmt.Errorf("environment %q daemon exited before readiness", name)
		}
		select {
		case <-ctx.Done():
			return backend.Environment{}, ctx.Err()
		case <-deadline.C:
			return backend.Environment{}, fmt.Errorf("environment %q daemon readiness timed out", name)
		case <-time.After(20 * time.Millisecond):
		}
	}
}
func (o *localOrchestrator) Release(ctx context.Context, scope backend.Scope) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	unlock, lockErr := o.lock(ctx)
	if lockErr != nil {
		return lockErr
	}
	defer unlock()
	var failures []error
	for name := range o.config.Environments {
		use, err := o.load(name)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		if use.PID == 0 {
			continue
		}
		remaining := []backend.Owner{}
		matched := false
		for _, owner := range use.Owners {
			if owner.Run == scope.Run && backend.WithinScope(scope.Scope, owner.Scope) {
				matched = true
			} else {
				remaining = append(remaining, owner)
			}
		}
		if !matched {
			continue
		}
		if len(remaining) > 0 {
			use.Owners = remaining
			failures = append(failures, o.save(name, use))
			continue
		}
		_ = syscall.Kill(-use.PID, syscall.SIGINT)
		stopped := false
		for !stopped {
			err := syscall.Kill(use.PID, 0)
			if errors.Is(err, syscall.ESRCH) {
				stopped = true
				break
			}
			select {
			case <-ctx.Done():
				failures = append(failures, fmt.Errorf("environment %q shutdown unconfirmed: %w", name, ctx.Err()))
				goto next
			case <-time.After(20 * time.Millisecond):
			}
		}
		_ = os.Remove(use.ReadyFile)
		failures = append(failures, os.Remove(o.file(name)))
	next:
	}
	return errors.Join(failures...)
}

// assigned resolves native session/process ownership before scope fallback.
func (o *localOrchestrator) assigned(r backend.MessageRequest) ([]string, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	var scopes []string
	best := -1
	for name := range o.config.Environments {
		use, err := o.load(name)
		if err != nil {
			return nil, err
		}
		for _, owner := range use.Owners {
			if owner.Run != r.Run {
				continue
			}
			if r.Kind == "cleanup" {
				if backend.WithinScope(r.Scope, owner.Scope) || backend.WithinScope(owner.Scope, r.Scope) {
					scopes = append(scopes, name)
					break
				}
				continue
			}
			target := r.Task
			if r.SessionID != "" {
				target = r.SessionID
			}
			if r.Target != nil {
				target = r.Target.ID
			}
			if owner.Task == target {
				return []string{name}, nil
			}
			if backend.WithinScope(owner.Scope, r.Scope) && len(owner.Scope) > best {
				best = len(owner.Scope)
				scopes = []string{name}
			}
		}
	}
	return scopes, nil
}
