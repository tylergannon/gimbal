# Codex adapter: raise the descriptor limit before starting the shared daemon

URL: https://github.com/tylergannon/gimble/issues/163
State: closed
Updated: 2026-09-14T00:42:00Z

Follow-up to #143 / #160.

## Problem

When the Codex adapter starts the shared `codex app-server` daemon itself (`connect` runs `codex app-server daemon start` when `daemon version` reports none running), the daemon inherits the soft `RLIMIT_NOFILE` of the Gimble process as it was at startup. On macOS that is 256 by default. Go raises the soft limit to the hard limit for its own process but deliberately hands children the original value, so a daemon started by a Gimble workflow gets 256 regardless of the user's shell configuration.

Each loaded thread costs the daemon about 12 descriptors (MCP child pipes), and it idles at about 140 with a handful of Codex Desktop threads. A `Group` of eight parallel Codex sessions sits near 240 while running. On 2026-09-12 the daemon logged 413 "Too many open files" errors after reaching 261 open descriptors; the measurements are in `ephemeral/attest/codex-daemon/proof.txt`. With `Close` archiving threads this is no longer a leak, but it remains a ceiling on fan-out.

## Change

In `codex/rpc.go`, immediately before running `daemon start`, call `syscall.Setrlimit(RLIMIT_NOFILE, ...)` with the soft limit raised to the hard limit. Once a Go program calls `Setrlimit` itself, the raised value is what children inherit. Nothing else in the adapter changes; a daemon already running is untouched.

## Also document

For a daemon started by Codex Desktop's SSH bootstrap, Gimble has no lever; the daemon inherits the login shell's limit. A one-line `ulimit -n 65536` in `~/.zshenv` covers that path on the next daemon start. Record that in the adapter's package doc or README as the operator's step, not something Gimble does.

## Acceptance

- A fake `codex` on PATH that reports the daemon stopped and, on `daemon start`, prints its own `ulimit -n` shows the raised value (the pattern in `codex/codex_test.go`'s `TestCloseNeverStartsAStoppedDaemon`).
- Live, on a machine where no daemon is running: after Gimble starts it, `lsof -p <pid>` climbing past 256 during a wide `Group` does not produce "Too many open files" in `~/.codex/app-server-control/app-server.log`. Do not stop or restart an existing daemon to produce this condition on a shared machine; note where the live proof ran.

