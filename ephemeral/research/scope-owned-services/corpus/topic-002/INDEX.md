# zsh-launched process ownership

Goal: identify the narrow ownership and status boundary introduced by executing a supplied command string through zsh.

Primary sources downloaded locally:

- [zsh invocation](sources/zsh-invocation.html) — official zsh manual; `-c` command-string execution.
- [zsh builtins](sources/zsh-builtins.html) — official zsh manual; `exec`, `exit`, and `wait` semantics.
- [zsh jobs and signals](sources/zsh-jobs-signals.html) — official zsh manual; job status, background/disown behavior, shell exit, and signals.
- [POSIX shell command language](sources/posix-shell-command-language.html) — POSIX specification; utility execution, last-command status, pipelines, asynchronous lists, and signal-derived status.

Synthesis:

1. `zsh -c` interprets the supplied string as shell code. For an external utility the shell uses a separate execution environment; unless the string explicitly invokes zsh `exec`, the shell remains the wrapper whose PID and exit status Go observes. `exec` replaces the shell, so it is a meaningful but caller-controlled exception.
2. For ordinary foreground syntax, the shell waits and reports the command's result. zsh `exit` defaults to the last command's status and `wait` reports the waited job/process status; POSIX records 126/127 for command lookup failures and signal termination as a value above 128. These are status observations, not readiness guarantees.
3. Shell syntax can make the boundary misleading: pipelines generally report the last command; asynchronous lists return without waiting and have a zero status under POSIX; zsh also documents disowned jobs and cases where shell exit does not wait for asynchronous helper processes. A command that backgrounds or delegates work can therefore make the shell look successful or finished while descendants remain.
4. Signal behavior is not implicit tree ownership. zsh documents inherited signal dispositions, special QUIT handling, and interactive job-control/HUP behavior, but the sources do not guarantee that a non-interactive `zsh -c` forwards a later SIGTERM to every descendant. Gimble should own only the launched workload boundary and must not claim ownership of resources that the supplied tool deliberately manages externally.

The longer status/ownership caveats are in [the zsh ownership clip](clips/zsh-ownership.md).

Question coverage: both assigned questions are addressed above. Unresolved: these sources do not settle whether the eventual macOS and Linux launch setup creates the same process group/session boundary or how every arbitrary delegated tool responds to group signaling; that must remain an explicit implementation-dependent constraint.
