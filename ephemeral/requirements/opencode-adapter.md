# OpenCode harness: end state and definition of done

Build this feature using the installed Gimbal implementation workflow in /Users/tyler/.codex/worktrees/c671/gimbal. Keep it simple. Follow AGENTS.md and docs/definition-of-done.md. The caller handles commits and delivery; the workflow must not edit this definition of done or commit/push/merge/install globally.

## End state

Gimbal can run its existing HarnessAdapter functionality through OpenCode’s legacy API. It uses one shared OpenCode server across projects and sessions, automatically started on first use. `gimbal opencode start` is idempotent; `gimbal opencode stop` stops it and interrupts active work. Runtime state has a configurable location defaulting under `~/.gimbal/`. Session cleanup leaves the shared server running.

Model selection explicitly chooses OpenCode: `opencode/<model-id>` uses its `opencode` provider; `opencode/<provider>/<model-id>` uses the named provider. Both workflow model flags and run-prompt support this. Existing unprefixed routing stays unchanged; prefixed models are not restricted to Gimbal’s current model families.

Start with a shared SSE stream and a synchronous prompt POST per active turn. The POST supplies completion and the final result; the stream supplies live events routed to the correct session and run. Record raw events before normalization, with enough request/result timing and identity to compare the stream to completed turns. Keep logs local and discoverable. Observation gaps must not turn an authoritative successful execution into failure. Later, Luna can use the logs to infer event-only completion; that optimization is not part of this build.

Use generated Go types for only what the adapter needs. Handwrite the handful of HTTP calls and SSE handling. No full SDK, SSE endpoint codegen, general daemon framework, or new workflow abstractions. Prefer existing adapter patterns and make routine implementation choices without asking the user about internals.

## Definition of done

The built Gimbal command demonstrably runs real OpenCode work: text and tool/file actions, schema output, conversation continuation, native fork, active-turn steering, and cancellation. Concurrent sessions in different projects receive their own events and results while sharing one server. Closing a session leaves that server and other work usable. Explicit stop ends active callers without hanging. Both model-name forms route correctly and ordinary existing models still work.

The raw capture preserves what arrived, including events omitted or unrecognized by normalization, and can be correlated with POST outcomes for later analysis. The needed types were actually generated and compile. Appropriate repo checks pass, the actual binary builds, and an independent validator personally observes the required behavior. Source review or a green unit suite alone is not live proof.

Use the cheapest already configured OpenCode model for live validation and name the exact provider/model. Live model/tool turns are authorized. Isolate test state/workdirs and clean up owned test servers; do not stop unrelated processes or print credentials. Keep probes, logs and run output in `.gimbal/`, never committed. Document normal use in concise existing README/help/skill locations as appropriate. Do not redesign the UI or modify workflow machinery. Finish at the repository’s 90–95% standard; report small remaining issues without expanding scope.

## Research access

Start at /Users/tyler/.codex/worktrees/c671/gimbal/ephemeral/research/opencode/IMPLEMENTATION-READING.md. It links all reports, notes, semantic indexes, pinned source, original OpenAPI, and completed generator experiments. Every implementation and validation agent can access those files directly. Use the indexes to find original evidence; historical recommendations are context, and this brief plus the current user instructions control the outcome.
