# macOS/Linux portability boundary

## Synthesis

The portable core is stable: process groups are the Unix unit for signaling
related processes, new children inherit the creator's group/session, and
`setsid()` creates a new session plus process group only when the caller is not
already a group leader. POSIX also says group lifetime ends when the last member
exits or leaves through `setpgid()`/`setsid()`. ([POSIX definitions](sources/posix-base-definitions.html),
[current macOS SDK setsid(2)](sources/macos-sdk-setsid.2),
[macOS source provenance](sources/macos-sdk-provenance.md))

The first material difference is the signaling interface and error contract.
Linux documents `kill(-pgid, sig)` and defines group success as delivery to at
least one permitted member. macOS documents `killpg(pgid, sig)` and says a
permission mismatch in one or more targets fails without sending any signal.
Both document missing-group errors, so cleanup must observe and report the
platform-specific result rather than infer that a successful call means every
member exited. ([Linux kill(2)](sources/linux-kill.2.html),
[current macOS SDK killpg(2)](sources/macos-sdk-killpg.2))

The second difference is Linux-only machinery that can extend ownership beyond
one process group. `PR_SET_CHILD_SUBREAPER` reparents orphaned descendants to a
designated subreaper and lets it receive `SIGCHLD`/wait status. cgroup v2's
`cgroup.kill` kills a cgroup and descendants with `SIGKILL`, handles concurrent
forks, and reports `EOPNOTSUPP` for threaded cgroups. These are useful evidence
that a stronger tree guarantee needs an explicit platform-specific ownership
mechanism, not a stronger reading of `killpg()`. ([Linux child subreaper](sources/linux-child-subreaper.2const.html),
[Linux cgroup v2](sources/linux-cgroup-v2.html))

The reviewed current macOS SDK primary sources document no equivalent subreaper
or cgroup tree-kill primitive. That absence cannot prove none exists, but it prevents a
portable claim. A detailed evidence clip is in
[portability-evidence.md](clips/portability-evidence.md).

## Recommended portability boundary

Guarantee cleanup only for the dedicated process group/session established for
the launched workload. Send `SIGTERM`, wait a bounded interval, escalate the
owned group with `SIGKILL`, and report failure if the final wait or signaling
result cannot prove termination. Do not promise descendants that intentionally
escape with `setsid()`/`setpgid()`, and do not make Linux subreapers or cgroups
part of the initial cross-platform contract.

## Questions addressed

- Material macOS/Linux differences: signaling call/partial-success semantics
  and Linux-only subreaper/cgroup options are identified above.
- Single unresolved point: all-descendant cleanup after deliberate group/session
  escape remains implementation-dependent, based on POSIX, Linux, and current
  macOS SDK primary sources.

## Out of scope

Subreapers, cgroups, launchd, persistent ownership, restart policies, and
orchestration-specific resource management are comparison evidence only; none
is recommended for issue 276's narrow initial contract.
