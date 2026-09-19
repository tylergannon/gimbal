# Issue 276: Add scope-owned services launched through zsh

## Problem

A workflow may need a development server or another supporting process while
agents work and validate. It should be possible to declare that dependency in
the scope that owns its lifetime, without making Gimble a general process
orchestrator.

## Requested behavior

Allow a scope to declare a required service using a command string executed
through `zsh`. Start the command and let workflow work continue while it runs.
The declaring scope owns the service and stops it when that scope closes,
including cancellation.

- Send `SIGTERM` on shutdown. Define bounded cleanup for the launched workload
  so stopping the shell alone does not silently leave its managed child
  processes running.
- A service declared in an enclosing scope stays alive across the loop
  iterations inside that scope. A service declared inside an iteration ends
  with that iteration.
- A service must not intentionally outlive the global workflow scope.
- Keep startup and runtime failures observable to the workflow.
- Let the supplied command run Docker, Overmind, or another tool when the
  caller needs more orchestration. Gimble does not manage those tools'
  external resources itself.

No public API name or signature has been chosen for this feature.

## Boundaries and proposed initial choices

Keep the first version to starting a command and owning its lifetime. Defer
automatic restarts. Keep readiness checks in ordinary workflow code. Do not
add dependency graphs, health-check frameworks, persistent daemons, or
orchestration-specific integrations.

Direct scope-owned processes are the proposed starting point. launchd was
considered because it provides restart and termination machinery, but a
registered job has an independent lifetime and can remain after the
registering Gimble process exits; it does not by itself solve scope ownership.
No launchd integration is requested.

## Open decision

How should an unexpected exit of a required service surface to the scope?
Failing the owning scope is a proposed default, not yet a settled API contract.
Settle this behavior before implementation; a required service should not
disappear silently.

## Proof

Run a real Gimble workflow with a foreground service and report what was
observed:

- The service is usable by work inside its owning scope.
- An enclosing scope keeps the same service alive across multiple loop
  iterations.
- Normal scope completion and cancellation both stop the launched workload,
  with `SIGTERM` observed by a cooperative service.
- Startup failure and unexpected exit are visible according to the settled
  failure contract.

Use a cheap model such as Codex `gpt-5.6-luna` for agent turns and name it in
the report. Repeatable lifecycle checks belong in package tests; do not commit
proof programs or run output.
