package shell

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// ErrShellAborted is what a shell operation returns when the context ended the
// command. It is pi's `throw new Error("aborted")` from ops.exec (bash.ts),
// rendered as a sentinel so the tool can classify it with errors.Is.
var ErrShellAborted = errors.New("aborted")

// ShellTimeoutError is what a shell operation returns when the command outran
// its timeout — pi's `throw new Error("timeout:<seconds>")` — rendered as a
// typed error so the tool can recover the seconds with errors.As instead of
// parsing a message.
type ShellTimeoutError struct{ Seconds float64 }

func (e *ShellTimeoutError) Error() string {
	return "timeout:" + strconv.FormatFloat(e.Seconds, 'g', -1, 64)
}

// BashExecOptions are the per-command controls pi passes to
// BashOperations.exec.
type BashExecOptions struct {
	// OnData receives interleaved stdout/stderr as it is produced.
	OnData func(data []byte)
	// TimeoutSeconds bounds the command; zero or less means no timeout.
	TimeoutSeconds float64
	// Env is the child environment, already assembled.
	Env []string
}

// BashOperations is the process execution the bash tool performs. pi shares one
// interface between its shell tools; the port keeps the seam so a host can
// delegate execution to a remote system (SSH, a container).
type BashOperations struct {
	// Exec runs a command, streaming output through OnData, and reports the
	// exit code. A signal termination is reported as 128 + the signal number,
	// pi's shell convention. A NIL exit code with a nil error is pi's
	// `exitCode: null`, which only a custom implementation can produce and which
	// the tool treats as a FAILED command. Abort and timeout come back as
	// ErrShellAborted and *ShellTimeoutError.
	Exec func(ctx context.Context, command, cwd string, options BashExecOptions) (exitCode *int, err error)
}

// maxTimeoutMillis is pi's MAX_TIMEOUT_MS (2^31 - 1).
const maxTimeoutMillis = 2147483647

// maxTimeoutSecondsText is how pi prints MAX_TIMEOUT_MS/1000 in its error.
const maxTimeoutSecondsText = "2147483.647"

// CreateLocalBashOperations returns bash operations that run commands on the
// local machine through the shell resolved by GetShellConfig, pi's
// createLocalBashOperations.
func CreateLocalBashOperations(shellPath string) BashOperations {
	return localShellOperations("bash", func() (ShellConfig, error) { return GetShellConfig(shellPath) })
}

// localShellOperations builds the shared local exec used by the built-in shell
// tools, pi's createLocalShellOperations.
func localShellOperations(shellName string, resolveShell func() (ShellConfig, error)) BashOperations {
	return BashOperations{
		Exec: func(ctx context.Context, command, cwd string, options BashExecOptions) (*int, error) {
			timeout, hasTimeout, err := resolveTimeout(options.TimeoutSeconds)
			if err != nil {
				return nil, err
			}
			if ctx.Err() != nil {
				return nil, ErrShellAborted
			}
			shellConfig, err := resolveShell()
			if err != nil {
				return nil, err
			}
			if _, err := os.Stat(cwd); err != nil {
				return nil, fmt.Errorf("Working directory does not exist: %s\nCannot execute %s commands.", cwd, shellName) //nolint:staticcheck // byte-exact pi error string
			}

			runCtx := ctx
			if hasTimeout {
				var cancel context.CancelFunc
				runCtx, cancel = context.WithTimeout(ctx, timeout)
				defer cancel()
			}

			commandFromStdin := shellConfig.CommandTransport == shellTransportStdin
			var cmd *exec.Cmd
			if commandFromStdin {
				cmd = exec.CommandContext(runCtx, shellConfig.Shell, shellConfig.Args...)
				cmd.Stdin = strings.NewReader(command)
			} else {
				cmd = exec.CommandContext(runCtx, shellConfig.Shell, append(shellConfig.Args, command)...)
			}
			cmd.Dir = cwd
			cmd.Env = options.Env
			// Own process group, and on cancel/timeout kill the whole tree so
			// backgrounded grandchildren do not survive.
			setProcessGroup(cmd)
			cmd.Cancel = func() error { return killProcessTree(cmd) }

			runErr := runBashCommand(cmd, onDataWriter(options.OnData))
			return classifyShellRun(ctx.Err(), runCtx.Err(), runErr, options.TimeoutSeconds)
		},
	}
}

// resolveTimeout validates pi's resolveTimeoutMs and returns the duration. A
// zero or negative value means no timeout; a value above the 32-bit millisecond
// cap is an error.
func resolveTimeout(timeoutSeconds float64) (time.Duration, bool, error) {
	if timeoutSeconds <= 0 {
		return 0, false, nil
	}
	millis := timeoutSeconds * 1000
	if millis > maxTimeoutMillis {
		return 0, false, fmt.Errorf("Invalid timeout: maximum is %s seconds", maxTimeoutSecondsText) //nolint:staticcheck // byte-exact pi error string
	}
	return time.Duration(millis) * time.Millisecond, true, nil
}

// classifyShellRun turns a finished command into pi's `{exitCode}` / thrown
// error pair. Its order is the contract:
//
//   - Abort wins over timeout when both fired.
//   - Both are checked BEFORE the exit status, as pi's try/catch is.
//   - A signal-killed child takes 128 + the signal number so the termination is
//     never mistaken for a clean exit.
func classifyShellRun(ctxErr, runCtxErr, runErr error, timeoutSeconds float64) (*int, error) {
	if ctxErr != nil {
		return nil, ErrShellAborted
	}
	if errors.Is(runCtxErr, context.DeadlineExceeded) {
		return nil, &ShellTimeoutError{Seconds: timeoutSeconds}
	}
	if runErr != nil {
		var exitErr *exec.ExitError
		if !errors.As(runErr, &exitErr) {
			return nil, runErr
		}
		code := exitCode(exitErr.ProcessState)
		return &code, nil
	}
	zero := 0
	return &zero, nil
}

// exitCode renders a finished process's status the way pi intends:
// `code ?? (signal ? 128 + signal : 1)`. Go reports -1 when the process was
// signalled, so the signal is read explicitly.
func exitCode(state *os.ProcessState) int {
	if code := state.ExitCode(); code >= 0 {
		return code
	}
	if signal, ok := exitSignalNumber(state); ok {
		return 128 + signal
	}
	return 1
}

// onDataWriter adapts pi's onData callback to the io.Writer runBashCommand
// feeds. A nil callback discards, which keeps Exec usable without streaming.
func onDataWriter(onData func([]byte)) io.Writer {
	if onData == nil {
		return io.Discard
	}
	return writerFunc(func(p []byte) (int, error) { onData(p); return len(p), nil })
}

type writerFunc func(p []byte) (int, error)

func (f writerFunc) Write(p []byte) (int, error) { return f(p) }
