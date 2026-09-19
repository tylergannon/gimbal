# Local agent observation and steering

Implement the agreed discover/watch/steer CLI using the runtime state and session steering already used by the browser.

- `gimble runs` discovers running instances through the project's `.gimble` directory and reports their runs. Multiple concurrent workflow processes must be distinguishable.
- `gimble watch <run>` shows the existing snapshot and follows the existing event stream, exposing session IDs usable for steering. Agent-readable structured output is sufficient; no decorative terminal UI is required.
- `gimble steer <run> --session <id> "message"` targets that exact session through existing steering. Report landed versus dropped honestly.
- `gimble steer <run> --loop <id> "message"` uses existing queued planner steering and reports queued.
- A local control socket operates alongside the browser and with no web listener. It shares the existing runtime's live runs and observation registry. It lives as long as the runtime. Discovery must contact the instance, rather than treat a leftover file as proof of life.
- Serve the existing HTTP snapshot and SSE observation routes over the Unix socket. Add only the missing list/steer handlers; do not introduce a separate JSON-RPC or watch protocol.
- Match existing CLI work-directory conventions. Scope is local use on this machine. No daemon service, remote access, cancellation/quit feature, new state model, provider changes, generalized transport framework, or unrelated workflow changes.
- Operating without serving Svelte is required; a broad package reorganization to eliminate all frontend build dependencies is outside this increment.

Prove a separate CLI process can discover a real workflow, observe the active session, steer it, and see changed behavior. Verify idle delivery using the existing semantics. Use cheapest live models, and report observations in chat/PR. Repeatable checks belong in package tests; commit no proof programs or run artifacts.

Planner, implementer, validator, and their coaches must keep scope narrow. Do not add over-engineering, adventitious features, gold-plating, belt-and-suspenders logic, or hypothetical edge-case fixes without a reasonable actual failing unit test. Coaching is advisory; fulfillment of these requirements is the completion criterion.

Existing implementation references: web/runtime.go, internal/live/live.go, internal/observation/http.go and registry.go, web/src/routes/steer.remote.go, cmd/gimble/main.go, cmd/gimble/workflows.go. Existing observation snapshots already associate turns with sessions and carry start/end state.
