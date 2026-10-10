# First risk: can Gimbal attach to the running desktop's app server?

**The documented public attachment path was not available in the inspected running setup.** This narrows the current deployment; it does not establish that all configurations or future builds prohibit attachment.

On October 9, 2026, at approximately 23:51 UTC, the installed desktop remained version `26.1007.21159`, build `20052`. Its main process, PID 5198, had two direct bundled Codex children, PIDs 5290 and 5553. Their file descriptors 0 and 1 were connected Unix socket pairs to the desktop process. Neither child had a TCP listener or a named public app-server Unix socket in the `lsof` inventory. This is consistent with private stdio hosting.

The documented default app-server socket path existed, but resolved through a symlink to a temporary daemon socket. A connection attempt returned `ECONNREFUSED` (61). The probe sent no bytes or RPC requests and then closed. Merely finding the socket file would have produced the wrong conclusion.

The desktop did own other named sockets, including its internal IPC router and native tool pipes. Those are different protocols and authorization boundaries; their existence is not evidence that public app-server JSON-RPC can attach there. No requests were made to those private endpoints.

This check did not create a session, change configuration, start a daemon, restart the application, or perform a computer-use action. The second question—whether an externally created thread retains the desktop's native tool wiring—therefore remains untested. Starting an independent server would test a different process and would not resolve it.

## Saved observations

- [Process/descriptor inventory](/Users/tyler/Documents/Codex/2026-10-09/gimbal-session-control-research/validation/installed-transport.json), including exact read-only commands and version metadata.
- [Default socket ownership lookup](/Users/tyler/Documents/Codex/2026-10-09/gimbal-session-control-research/validation/public-socket-ownership.json), including symlink resolution and no matching owner returned by `lsof`.
- [Public socket connect-only result](/Users/tyler/Documents/Codex/2026-10-09/gimbal-session-control-research/validation/public-socket-reachability.json), the direct negative observation.
- [Source/observation hashes](/Users/tyler/Documents/Codex/2026-10-09/gimbal-session-control-research/validation/manifest.json).

The public transport contract is separately described in the [official app-server documentation](https://learn.chatgpt.com/docs/app-server) and verified against pinned source in the [Codex lane](codex-report.md). A listener supporting that protocol is a prerequisite to testing native-tool retention through it.
