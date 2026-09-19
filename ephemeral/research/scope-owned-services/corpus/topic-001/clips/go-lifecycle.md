# Go lifecycle details

Source: [Go `os/exec` implementation](../sources/go-os-exec-exec.go.txt), especially lines 250-314, 486-504, 626-652, 785-850, 916-1055.

- `Start` returns after `os.StartProcess`; successful return sets `Cmd.Process` and does not wait for the child. A start error covers lookup/preparation/spawn failure. `Cancel` is not called when `Start` fails.
- `Wait` is the required observation and cleanup point. It waits for the OS process and any Go I/O-copy goroutines, converts an unsuccessful process state to `*ExitError`, must not be concurrent, and releases `Cmd` resources.
- `CommandContext` installs a cancellation watcher whose default action is `Process.Kill`, i.e. the one tracked process. A custom `Cancel` may signal or otherwise interrupt it. If cancellation races with an already-finished process, `ErrProcessDone` avoids injecting a needless cancellation error; a successful exit after a real cancellation can instead return the context error.
- `WaitDelay` bounds a stuck child or inherited output pipe, but its forced kill and pipe closure are still about the tracked process and descriptors. The Go tests explicitly leave a grandchild behind in a `SIGKILL` case and show that a successful child exit can still yield `ErrWaitDelay` when a descendant retains output pipes.
- `Output` and `CombinedOutput` call `Run`/`Wait`; default `Output` captures stdout and only a bounded diagnostic subset of stderr on `ExitError`. Live or complete output requires explicitly owned writers/pipes.

Source: [Go `os/exec` tests](../sources/go-os-exec-exec_test.go.txt), especially lines 954-1015, 1296-1455, 1490-1585. These tests are the strongest local evidence for cancellation races, handled signals, forced kill, inherited pipes, and cancellation-vs-exit ordering.
