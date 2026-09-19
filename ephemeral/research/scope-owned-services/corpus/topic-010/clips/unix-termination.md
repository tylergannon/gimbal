# Unix termination excerpt

Sources: [`linux-setpgid.md`](../sources/linux-setpgid.md), [`linux-setsid.md`](../sources/linux-setsid.md), [`linux-kill.md`](../sources/linux-kill.md), [`macos-setpgid.md`](../sources/macos-setpgid.md), [`macos-setsid.md`](../sources/macos-setsid.md), [`macos-kill.md`](../sources/macos-kill.md), and [`macos-sigaction.md`](../sources/macos-sigaction.md).

- Linux `setsid()` succeeds only when the caller is not a process-group leader; it makes the caller a new session and process-group leader. Linux `kill(-pgid, sig)` targets the process group whose ID is `pgid`.
- Linux documents `EPERM` for an invalid session/group transition or insufficient signal permission, and `ESRCH` when the target process/group no longer exists. A group signal therefore needs explicit error handling and cannot be treated as proof that every intended descendant received the signal.
- Apple documents `setpgid()` and `setsid()` with the same broad session/process-group model, and `kill()` explicitly sends a negative PID to a process group. Errors include `EACCES`, `EPERM`, and `ESRCH`; its signal documentation identifies `SIGTERM` as the software-termination signal and `SIGKILL` as non-catchable termination.

Contract implication: the owned unit should be a deliberately established process group/session where supported; shutdown sends `SIGTERM`, waits a finite bound, escalates to `SIGKILL`, then observes the direct child and reports cleanup failure rather than silently claiming the tree is gone.
