# Go command lifecycle semantics

Goal: establish what Go itself guarantees when a scope owns a foreground command.

Primary sources downloaded locally:

- [Go `os/exec` implementation](sources/go-os-exec-exec.go.txt) — official Go source; `Start`, `Wait`, `CommandContext`, `Cancel`, `WaitDelay`, and output methods.
- [Go `os/exec` tests](sources/go-os-exec-exec_test.go.txt) — official Go tests covering cancellation, signal handling, inherited pipes, and races.

Synthesis:

1. `Start` is synchronous only for command preparation and process creation. A nil error means `Cmd.Process` exists; it does not mean the command is ready, healthy, or still running. Lookup, I/O setup, and spawn failures are returned synchronously. `Cancel` is not invoked after a failed `Start`.
2. `Wait` is the runtime authority. It observes process exit, maps a non-successful state to `*exec.ExitError`, waits for non-file I/O copying, and releases resources. It must be called after successful `Start`, exactly once, and not concurrently. `Run` is simply `Start` followed by `Wait`.
3. `CommandContext` starts a watcher after `Start`; its default cancellation action is `Cmd.Process.Kill`, not a process-tree signal. A custom cancel action can send another signal. Cancellation races with exit: an already-finished process is not given a needless cancellation error, while a successful exit after an actual cancellation can still return the context error. `WaitDelay` bounds a stuck process or open inherited pipes, but does not promise descendant cleanup.
4. Output is only observable according to the chosen wiring. `Output` captures stdout and a bounded diagnostic stderr subset on failure; `CombinedOutput` captures both but waits for completion. For a live service, the owning scope must attach writers/pipes and retain the `Wait` result; a start event alone is insufficient.

The detailed race and grandchild-pipe evidence is in [the lifecycle clip](clips/go-lifecycle.md).

Question coverage: both assigned questions are addressed above. The source-backed unresolved point is that Go's default cancellation targets one process and does not define a portable descendant-tree cleanup mechanism.
