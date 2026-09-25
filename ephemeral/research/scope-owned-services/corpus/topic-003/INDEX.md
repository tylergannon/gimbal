# Process groups, sessions, and tree termination

## Scope

This topic answers which Unix primitives can stop a zsh-launched workload and
what a bounded, observable cleanup contract can honestly guarantee. It does
not propose an API or manage Docker/Overmind/etc. resources.

## Synthesis

Use one dedicated process group as the direct ownership boundary. POSIX defines
a process group specifically as a signalable collection of related processes,
and newly created children inherit the creator's group. Linux can signal the
group with `kill(-pgid, SIGTERM)`; macOS exposes the equivalent `killpg(pgid,
SIGTERM)`. Thus a shell and ordinary descendants that retain the group receive
the shutdown signal together. ([Linux kill(2)](sources/linux-kill.2.html),
[current macOS SDK killpg(2)](sources/macos-sdk-killpg.2))

The group must be established before the launched program can `execve()` or
reorganize itself. Linux documents `setpgid()` failing with `EACCES` after a
child has execed and with `EPERM` for cross-session, session-leader, or missing
group cases. `setsid()` creates both a session and a new process group, but its
caller must not already be a process-group leader; macOS documents the same
`EPERM` precondition. A session is not itself a single kill target: it can
contain multiple process groups. ([Linux setpgid(2)](sources/linux-setpgid.2.html),
[Linux setsid(2)](sources/linux-setsid.2.html),
[current macOS SDK setsid(2)](sources/macos-sdk-setsid.2),
[current macOS SDK setpgid(2)](sources/macos-sdk-setpgid.2),
[macOS source provenance](sources/macos-sdk-provenance.md))

The group boundary is deliberately narrow. A descendant that calls `setpgid()`
or `setsid()` can leave it, so “all descendants” is not a portable promise.
The supplied tool remains responsible for any processes it deliberately moves
outside the group; Gimbal can only report that its owned group was cleaned up
or that cleanup remained uncertain.

Shutdown should be bounded and status-bearing: send `SIGTERM`, wait for the
direct child and its exit status, then, before the deadline, send `SIGKILL` to
the still-owned group and wait again. Linux documents that `SIGKILL` cannot be
caught, blocked, or ignored, but signal delivery is not synchronous termination:
Linux `kill()` succeeds when at least one group member received the signal, and
`wait()` is required both to observe status and to reap the direct child.
([Linux signal(7)](sources/linux-signal.7.html),
[Linux kill(2)](sources/linux-kill.2.html),
[Linux wait(2)](sources/linux-wait.2.html))

Failure to signal (`EPERM`, `ESRCH`, or the macOS all-or-nothing permission
case) and expiry of the final wait deadline are observable cleanup failures,
not success. A longer evidence table is in
[termination-evidence.md](clips/termination-evidence.md).

## Actionable constraints for issue 276

1. Treat the dedicated process group, not the shell PID, as the owned shutdown
   target.
2. Make group/session setup a startup precondition and surface setup errors;
   do not assume a post-`execve()` repair is race-free.
3. Define cleanup as `SIGTERM` plus a fixed wait deadline and `SIGKILL` fallback,
   followed by direct-child wait/status observation.
4. Report incomplete cleanup when signaling or final observation cannot prove
   the owned group is gone; do not promise escaped-descendant cleanup.

## Questions addressed

- Group/session mechanisms, prerequisites, and failure modes: answered above
  from five Unix primary sources.
- Bounded escalation and avoiding silent descendants: answered above; the
  remaining portability limit is made explicit in topic-004.

## Unresolved

The sources do not establish a portable mechanism that captures descendants
after they intentionally leave the owned process group/session. That remains an
implementation-dependent boundary, not a reason to add a general process
orchestrator.
