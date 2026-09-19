# Process-group termination evidence

This clip preserves the contract-relevant details behind the synthesis in
`../INDEX.md`.

## Signal target and membership

- POSIX defines a process group as a collection that permits signaling related
  processes; a newly created process joins its creator's group. Its process
  group can later change through `setpgid()` or `setsid()`.
  ([POSIX definitions](../sources/posix-base-definitions.html) is not copied
  here; the topic-004 copy is the same downloaded primary source.)
- Linux `kill(pid, sig)` targets every member when `pid < -1`, with `-pid` as
  the process-group ID. Success means at least one signal was delivered, not
  that every member exited. `EPERM` means no target was signalable and `ESRCH`
  means the group does not exist.
  ([Linux kill(2)](../sources/linux-kill.2.html))
- macOS `killpg(pgrp, sig)` targets one process group. It returns `ESRCH` for
  an absent group and, unlike Linux's documented partial-success wording,
  says a mismatched target identity makes the call fail without sending a
  signal.
  ([current macOS SDK killpg(2)](../sources/macos-sdk-killpg.2))

## Formation and escape

`setpgid()` must establish the group before the child has completed `execve()`;
Linux documents `EACCES` after exec and `EPERM` for cross-session, session-leader,
or nonexistent-group cases. `setsid()` creates a new session and one new
process group, but fails for a process-group leader; the documented fork-then-
`setsid()` pattern exists to avoid that precondition. The same process-group-
leader failure is documented by macOS. ([Linux setpgid(2)](../sources/linux-setpgid.2.html),
[Linux setsid(2)](../sources/linux-setsid.2.html),
[current macOS SDK setsid(2)](../sources/macos-sdk-setsid.2))

The group is the bounded ownership unit: ordinary descendants inherit it and
are reachable by group signaling, but a descendant that calls `setpgid()` or
`setsid()` leaves that unit. A session is a collection of process groups;
creating a session alone does not make one signal address cover every group.

## Bounded shutdown

The narrow defensible sequence is: signal the owned group with `SIGTERM`, wait
for the launched child and observe its status, and within a fixed deadline
escalate the still-owned group with `SIGKILL`, then wait again. Linux documents
that `SIGKILL` cannot be caught, blocked, or ignored. `wait()`/`waitpid()` is
still required: without a wait, a terminated direct child remains a zombie,
and wait status distinguishes normal exit from signal termination.
([Linux signal(7)](../sources/linux-signal.7.html),
[Linux wait(2)](../sources/linux-wait.2.html))

An `ESRCH` group or a permission error must be reported as cleanup uncertainty,
not treated as proof that all descendants are gone. If the final wait deadline
expires, the owning scope must surface that cleanup was incomplete; the
process-group mechanism cannot prove cleanup of descendants that escaped the
group.
