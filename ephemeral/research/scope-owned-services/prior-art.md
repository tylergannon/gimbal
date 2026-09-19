# Prior art for issue 276: scope-owned commands through zsh

This report covers the assigned Go/zsh slice only. Sources are downloaded primary Go, zsh, and POSIX documents in the corpus.

## What Go guarantees

The Go `os/exec` implementation makes `Start` a process-creation observation: success sets `Cmd.Process`; it does not mean readiness or continued liveness. Lookup, descriptor setup, and spawn errors are synchronous. After success, `Wait` is mandatory and authoritative for process exit, `ProcessState`, nonzero exit (`*exec.ExitError`), I/O-copy completion, and resource release. It must not run concurrently or be called from a custom cancellation function. [`go-os-exec-exec.go`](corpus/topic-001/sources/go-os-exec-exec.go.txt) and the corresponding official tests [`go-os-exec-exec_test.go`](corpus/topic-001/sources/go-os-exec-exec_test.go.txt).

`CommandContext` installs a watcher whose default action is `Cmd.Process.Kill`; it does not signal a process group or descendants. A custom cancel action can send another signal, but `Wait` still decides the final result. The source explicitly handles cancellation racing with an already-exited process (`ErrProcessDone`) and can report the context error even when the child later exits zero. `WaitDelay` bounds a stuck child or open inherited pipe, not a general descendant tree. The official tests show a killed child with a grandchild still writing to pipes, and a successful child exit that returns `ErrWaitDelay` because a descendant kept output open. [`go-lifecycle.md`](corpus/topic-001/clips/go-lifecycle.md).

`Output`/`CombinedOutput` are completion-oriented capture helpers. Live service output needs explicitly wired writers/pipes, and the owning scope must retain and interpret `Wait`; a “started” event is not enough.

## What zsh changes

`zsh -c` makes the supplied string shell code. External utilities run in a separate utility environment, but absent an explicit zsh `exec`, the shell remains the wrapper whose process/status Go sees. `exec` replaces the shell and is caller-controlled. zsh reports the last command's status for `exit` without an argument; `wait` reports the waited process's status. POSIX supplies the same last-simple-command rule, 126/127 lookup failures, and signal-derived status above 128. [`zsh-invocation.html`](corpus/topic-002/sources/zsh-invocation.html), [`zsh-builtins.html`](corpus/topic-002/sources/zsh-builtins.html), [`posix-shell-command-language.html`](corpus/topic-002/sources/posix-shell-command-language.html).

The string may change the status boundary: a pipeline reports its last command; an asynchronous list does not wait and has zero status in POSIX; zsh documents disowning and asynchronous helper processes. Thus shell exit/status is not proof that all descendants ended. zsh documents inherited signals, special QUIT handling, and interactive job-control/HUP rules, but does not promise that non-interactive `zsh -c` forwards a later SIGTERM to every descendant. [`zsh-jobs-signals.html`](corpus/topic-002/sources/zsh-jobs-signals.html) and [`zsh-ownership.md`](corpus/topic-002/clips/zsh-ownership.md).

## Narrow contract constraints for issue 276

1. Treat successful start as “the wrapper process was spawned,” not readiness. Keep readiness in ordinary workflow checks, as the issue requests.
2. Make `Wait` (including its I/O result) the single runtime observation. Surface synchronous start failure immediately; surface unexpected wrapper/foreground-command exit as a required-service failure rather than silently continuing. Do not infer health from output or from shell status alone.
3. On normal scope close or cancellation, initiate shutdown with SIGTERM at the explicitly owned launch boundary, then use a bounded escalation deadline and final observation. A tracked PID alone is insufficient when zsh or the supplied command has descendants; descendant cleanup requires an explicit process-group/session choice.
4. Preserve command output/status as observable workflow evidence, while making clear that shell status represents the shell's chosen foreground/pipeline/background boundary. Do not take responsibility for Docker, Overmind, or other tools' external resources.

## Recommendation and unresolved portability point

Recommend one narrow initial behavior: a scope owns the launched zsh/workload boundary for its lifetime; startup errors fail immediately; the workflow continues only while the required command remains observed alive; an unexpected exit fails the owning scope with the recorded command status/output; normal close and cancellation issue SIGTERM, wait up to a fixed bound, escalate, then wait and report cleanup status. The service cannot intentionally outlive the global scope. No restart policy, readiness framework, persistent daemon, launchd integration, dependency graph, or public API shape follows from this recommendation.

Unresolved: the assigned primary sources do not establish a single identical macOS/Linux default for creating a dedicated process group/session around non-interactive `zsh -c`, nor do they establish how arbitrary delegated tools respond to group signals or daemonize. The contract should therefore require an explicit implementation-dependent launch boundary and live evidence on both supported OS families before claiming descendant cleanup. This report compares only two approaches: direct Go process ownership and shell-mediated ownership; it does not compare persistent/container managers.
