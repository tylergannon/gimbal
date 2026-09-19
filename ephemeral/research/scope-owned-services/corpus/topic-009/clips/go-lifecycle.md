# Go lifecycle excerpt

Source: [`go-exec-source.txt`](../sources/go-exec-source.txt), the downloaded Go 1.27.1 standard-library source; see `Cmd.Start`, `Cmd.Wait`, `CommandContext`, `Cancel`, `WaitDelay`, and `CombinedOutput`.

- `Start` returns before the command exits; after success, `Process` is set and `Wait` must be called to release resources.
- `CommandContext` installs cancellation that calls `Process.Kill`; it leaves `WaitDelay` unset. `Cancel` is not called when `Start` fails.
- `Wait` requires a started command, is not safe concurrently, waits for the process and configured I/O copying, records `ProcessState`, and reports the exit/I/O/context result.
- `os.Process.Kill` is immediate and non-waiting, and explicitly affects only that process, not children it started.
- `WaitDelay` bounds a child that does not exit after context cancellation and pipes that remain open, but its fallback is `os.Process.Kill`; it is not descendant cleanup.

Contract implication: a background service needs a separate, single owner of `Wait`, a status path distinct from startup, and process-tree cleanup beyond default `CommandContext` cancellation.
