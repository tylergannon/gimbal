# macOS/Linux portability evidence

## Shared POSIX baseline

POSIX defines process groups as signalable collections, says new processes join
their creator's group/session, and explicitly allows process-group lifetime to
end when the last member exits or leaves via `setpgid()`/`setsid()`.
([POSIX definitions](../sources/posix-base-definitions.html))

The current macOS SDK's `setsid(2)` and `killpg(2)` document the same core model: `setsid()`
creates a one-process session and group, while `killpg()` signals one group and
can fail with `EPERM` or `ESRCH`. ([macOS SDK setsid(2)](../sources/macos-sdk-setsid.2),
[macOS SDK killpg(2)](../sources/macos-sdk-killpg.2),
[macOS source provenance](../sources/macos-sdk-provenance.md))

## Material differences

- Linux documents the negative-PID form of `kill()` and says group-call success
  means at least one signal was delivered. The current macOS SDK documents
  `killpg()` and says a permission mismatch in one or more targets fails the
  call without sending a signal. Cleanup code cannot treat the two return
  conventions as identical.
  ([Linux kill(2)](../sources/linux-kill.2.html),
  [macOS SDK killpg(2)](../sources/macos-sdk-killpg.2))
- Linux has non-POSIX descendant-adoption mechanisms: a child subreaper gets
  orphaned descendants reparented to it and can wait for them; cgroup v2's
  `cgroup.kill` kills an entire cgroup subtree with `SIGKILL`, handles
  concurrent forks, and can fail with `EOPNOTSUPP` for threaded cgroups.
  ([Linux child subreaper](../sources/linux-child-subreaper.2const.html),
  [Linux cgroup v2](../sources/linux-cgroup-v2.html))
- The reviewed current macOS SDK primary sources document no equivalent
  subreaper or cgroup tree-kill primitive. Their absence here is evidence against promising one
  portable implementation, not evidence that no private mechanism exists.

## Single unresolved portability point

Can the implementation guarantee cleanup of every descendant if the supplied
command deliberately daemonizes or calls `setsid()`/`setpgid()` to leave the
dedicated group? The answer is unresolved and must remain implementation-
dependent: POSIX permits group/session departure; Linux has extra mechanisms,
but the reviewed current macOS SDK sources provide no corresponding portable
primitive.
The initial contract should therefore guarantee only the dedicated group,
surface an incomplete final wait, and avoid Linux-only subreapers/cgroups in a
cross-platform promise.
