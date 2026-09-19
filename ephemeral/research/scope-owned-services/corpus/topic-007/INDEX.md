# Select and study a small comparison set

## Selection

Two bounded test-workflow tools are enough for this comparison:

1. **Playwright Test `webServer`** — a test runner option that launches a local
   development server for the duration of a test run. It is directly relevant
   because it owns a foreground command while test work runs, then tears it
   down; it is not a persistent daemon or a container manager.
2. **`start-server-and-test`** — a CLI that starts a server, waits for a URL,
   runs a test command, and stops the server. It is relevant for the same
   bounded foreground lifetime and explicit failure/cleanup behavior.

No third approach is needed.

Downloaded-source provenance: [Playwright web-server docs](https://playwright.dev/docs/test-webserver), [Playwright plugin source](https://github.com/microsoft/playwright/blob/main/packages/playwright/src/plugins/webServerPlugin.ts), [`start-server-and-test` README](https://github.com/bahmutov/start-server-and-test/blob/master/README.md), and [`start-server-and-test` source](https://github.com/bahmutov/start-server-and-test/blob/master/src/index.js).

## Playwright Test

The [downloaded web-server documentation](sources/playwright-webserver.html)
defines `command`, optional URL or stdout/stderr readiness, startup timeout,
output routing, and teardown. Its documented default is process-group
`SIGKILL`; a configured graceful shutdown sends `SIGTERM` or `SIGINT` to the
group and escalates to `SIGKILL` after the configured timeout. The docs also
say that Windows ignores these Unix signals.

The [downloaded implementation](sources/playwright-webServerPlugin.ts#L72-L209)
shows the lifecycle precisely:

- `setup` starts a shell command, then races URL/stdout/stderr readiness against
  the child exiting or a deadline. An early exit becomes a startup error;
  timeout is also an error. With no readiness condition, setup returns after
  launch, so readiness is not intrinsically guaranteed.
- After setup, the plugin retains the launched process and keeps it alive while
  tests run. It does not restart it. stdout and stderr are forwarded to the
  test reporter when configured (and stderr by default), while regex output can
  serve as an optional startup gate.
- `teardown` invokes the returned close operation. The graceful path signals
  the *process group* with `process.kill(-pid, signal)` and waits for the child
  `close` event; the configured timeout makes failure to close observable.
  Setup failure also calls teardown before rethrowing.

This is the strongest comparison for bounded ownership, group cleanup, bounded
escalation, and observable startup failure. It also demonstrates that a tool
can report a service's early disappearance rather than silently treating the
test run as independent.

## `start-server-and-test`

The [downloaded README](sources/start-server-and-test-README.md#L40-L65)
describes the compact contract: execute the server command, wait for a URL,
run tests, and shut down the server when tests finish. It inherits server
stdout/stderr in the implementation, so command output remains visible.

The [downloaded implementation](sources/start-server-and-test-index.js#L31-L155)
uses `execa(..., {shell: true})`, then polls one or more resources with
`wait-on` (default five-minute timeout and two-second interval). Before
readiness, a `close` event rejects with `server closed unexpectedly`. Once
ready, that listener is removed. The test command runs only after readiness,
and `finally(stopServer)` runs whether the test succeeds or fails. Cleanup
calls `tree-kill` with `SIGINT`, treats an already-exited process as benign,
and rejects other kill errors.

The tool therefore supplies useful patterns—foreground lifetime, readiness
before test work, inherited output, and unconditional cleanup—but it is weaker
evidence for issue 276 after readiness: the shown code does not keep an
unexpected-exit observer attached during the test command, and it has no
explicit bounded SIGTERM-then-SIGKILL escalation in this layer. Its README also
supports multiple servers and ordered startup, which is more orchestration
than Gimble needs.

## Comparison relevant to issue 276

Transferable: establish ownership before work begins; make startup failure and
unexpected disappearance observable; retain the service through the owning
operation; run cleanup on success and failure; preserve command output; and
use process-group cleanup with bounded graceful-to-forced escalation.

Not transferable as a first Gimble contract: reusable pre-existing servers,
multiple-service sequencing, built-in URL/HTTP health-check policy, output
regexes as a readiness framework, restart behavior, or cleanup of resources
created by Docker/Overmind/etc. Those features either make ownership escape the
scope or introduce dependency/health orchestration that issue 276 explicitly
defers.
