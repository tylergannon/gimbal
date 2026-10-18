// Package commandexecutor owns identified commands independently of observers.
package commandexecutor

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/tylergannon/gimbal/internal/executor"
)

type Request struct {
	ID          executor.CommandID `json:"id"`
	Command     string             `json:"command"`
	Args        []string           `json:"args"`
	Workdir     string             `json:"workdir"`
	Environment []string           `json:"environment"`
	Timeout     time.Duration      `json:"timeout"`
}
type Result struct {
	NotFound  bool      `json:"not_found,omitempty"`
	ExitCode  int       `json:"exit_code"`
	Error     string    `json:"error,omitempty"`
	Cancelled bool      `json:"cancelled"`
	TimedOut  bool      `json:"timed_out"`
	Completed time.Time `json:"completed"`
}

const workerArg = "--gimbal-command-worker"

// Any Go workflow importing RunCommand can re-exec its own binary as the
// supervisor. Installed distributions prefer the small sibling gimbal-command.
func init() {
	if len(os.Args) == 3 && os.Args[1] == workerArg {
		var req Request
		if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if err := Run(os.Args[2], req); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
}
func Submit(root string, req Request) (executor.Descriptor, error) {
	var err error
	root, err = executor.Root(root)
	if err != nil {
		return executor.Descriptor{}, err
	}
	if err = executor.Validate(string(req.ID), "command_"); err != nil {
		return executor.Descriptor{}, err
	}
	if req.Timeout <= 0 {
		req.Timeout = 20 * time.Minute
	}
	if req.Workdir == "" {
		req.Workdir, err = os.Getwd()
	}
	if err != nil {
		return executor.Descriptor{}, err
	}
	req.Workdir, err = filepath.Abs(req.Workdir)
	if err != nil {
		return executor.Descriptor{}, err
	}
	binary, err := os.Executable()
	if err != nil {
		return executor.Descriptor{}, err
	}
	sibling := filepath.Join(filepath.Dir(binary), "gimbal-command")
	if info, e := os.Stat(sibling); e == nil && info.Mode().IsRegular() {
		binary = sibling
	}
	return executor.Launch(binary, []string{workerArg, root}, req, root)
}
func Run(root string, req Request) error {
	if err := executor.Validate(string(req.ID), "command_"); err != nil {
		return err
	}
	dir := filepath.Join(root, string(req.ID))
	accepted := struct {
		ID              executor.CommandID
		Command         string
		Args            []string
		Workdir         string
		EnvironmentHash string
		Timeout         time.Duration
	}{req.ID, req.Command, req.Args, req.Workdir, fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(req.Environment, "\x00")))), req.Timeout}
	lease, err := executor.Claim(dir, accepted)
	if err != nil {
		return err
	}
	if lease == nil {
		return executor.Startup(executor.Describe(dir, false))
	}
	defer lease.Close()
	stopVacuum := executor.PeriodicVacuum(root)
	defer stopVacuum()
	ctx, cancel := context.WithTimeout(context.Background(), req.Timeout)
	defer cancel()
	server, err := executor.Listen(dir, func(c executor.Control) executor.Reply {
		if c.Op != "cancel" {
			return executor.Reply{Error: "unsupported command control"}
		}
		cancel()
		return executor.Reply{Landed: true}
	})
	if err != nil {
		return err
	}
	defer server.Close()
	out, err := os.OpenFile(filepath.Join(dir, "stdout.log"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer out.Close()
	stderr, err := os.OpenFile(filepath.Join(dir, "stderr.log"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer stderr.Close()
	if err = executor.Accept(dir, accepted); err != nil {
		return err
	}
	// The accepted timeout starts here and is never changed by a lookup.
	if err = executor.Write(filepath.Join(dir, "deadline.json"), time.Now().Add(req.Timeout)); err != nil {
		panic(err)
	}
	_ = executor.Startup(executor.Describe(dir, false))
	cmd := exec.CommandContext(ctx, req.Command, req.Args...)
	cmd.Dir = req.Workdir
	cmd.Env = req.Environment
	cmd.Stdout = out
	cmd.Stderr = stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	err = cmd.Run()
	// A command owns its process group. Its ordinary children cannot outlive
	// terminal publication, including when the group leader exits first.
	if cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	result := Result{ExitCode: -1, Completed: time.Now(), NotFound: errors.Is(err, exec.ErrNotFound)}
	if cmd.ProcessState != nil {
		result.ExitCode = cmd.ProcessState.ExitCode()
	}
	if ctx.Err() != nil {
		result.Cancelled = errors.Is(ctx.Err(), context.Canceled)
		result.TimedOut = errors.Is(ctx.Err(), context.DeadlineExceeded)
		result.Error = ctx.Err().Error()
	} else if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			result.Error = err.Error()
		}
	}
	if e := errors.Join(out.Sync(), stderr.Sync()); e != nil {
		result.Error = errors.Join(err, e).Error()
	}
	if err = executor.Write(filepath.Join(dir, "result.json"), result); err != nil {
		panic(err)
	}
	server.Notify()
	return nil
}
func Wait(ctx context.Context, d executor.Descriptor) (Result, error) {
	dir := filepath.Dir(d.Result)
	for {
		var result Result
		err := executor.Read(d.Result, &result)
		if err == nil {
			return result, nil
		}
		if !os.IsNotExist(err) {
			return result, err
		}
		if err = executor.Wait(ctx, dir, 0); err != nil {
			return result, err
		}
	}
}
func Cancel(ctx context.Context, d executor.Descriptor) error {
	if executor.Exists(d.Result) {
		return nil
	}
	_, err := executor.Call(ctx, filepath.Dir(d.Result), executor.Control{Op: "cancel"})
	if err != nil {
		executor.AssertOwner(filepath.Dir(d.Result))
	}
	return err
}
func CLI(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("gimbal-command", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var id, root, workdir, lookup, cancel string
	var timeout time.Duration
	flags.StringVar(&id, "id", "", "stable command_ ULID (required)")
	flags.StringVar(&root, "state-dir", "", "executor state directory")
	flags.StringVar(&workdir, "workdir", ".", "command directory")
	flags.StringVar(&lookup, "lookup", "", "look up command identity")
	flags.StringVar(&cancel, "cancel", "", "cancel command identity")
	flags.DurationVar(&timeout, "timeout", 20*time.Minute, "accepted execution deadline")
	if err := flags.Parse(args); err != nil {
		return err
	}
	root, err := executor.Root(root)
	if err != nil {
		return err
	}
	if lookup != "" || cancel != "" {
		if cancel != "" {
			lookup = cancel
		}
		if err = executor.Validate(lookup, "command_"); err != nil {
			return err
		}
		dir := filepath.Join(root, lookup)
		if !executor.Exists(filepath.Join(dir, "request.json")) {
			return errors.New("command not found")
		}
		executor.AssertOwner(dir)
		d := executor.Describe(dir, false)
		if cancel != "" {
			if err = Cancel(context.Background(), d); err != nil {
				return err
			}
		}
		return json.NewEncoder(stdout).Encode(d)
	}
	if flags.NArg() == 0 {
		return errors.New("give a command after --")
	}
	d, err := Submit(root, Request{ID: executor.CommandID(id), Command: flags.Arg(0), Args: flags.Args()[1:], Workdir: workdir, Environment: os.Environ(), Timeout: timeout})
	if err != nil {
		return err
	}
	return json.NewEncoder(stdout).Encode(d)
}
