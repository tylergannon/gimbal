# Current Gimble code handoff for Diffusion Router research

Checked after rebasing this branch onto `origin/main` at `81dc3c8` (2026-09-25). The original research notes were written against the earlier `39462ce` checkout. Their provider probes and archived Portal material remain useful, but implementation must use the current root Go module.

## Current seams

- [`harness.go`](../../../../harness.go) defines the active `HarnessAdapter`: `CreateSession(ctx, model, effort, workdir)`, `RunTurn(ctx, sessionID, prompt, schema, onEvent)`, `Steer`, `Fork`, and `Close`. Cancellation is through `RunTurn`'s context. There are no adapter `Interrupt` or `Compact` methods.
- [`session.go`](../../../../session.go) creates the native session lazily on the first turn. `Generate` validates output and can re-ask a bounded number of times. For prose, `TurnResult.Output` is a JSON string; for a schema turn it is raw JSON validated by the caller. The adapter must still ask Pi for JSON because Pi's core RPC prompt has no documented schema option.
- [`internal/modelalias/modelalias.go`](../../../../internal/modelalias/modelalias.go) handles model names. It already accepts explicit `opencode/<model>` and `opencode/<provider>/<model>` routes; Pi should get an equally explicit route. [`internal/binding/binding.go`](../../../../internal/binding/binding.go) constructs adapters and shares OpenCode's adapter among roles.
- [`opencode/adapter.go`](../../../../opencode/adapter.go) implements the current contract against OpenCode's shared server, including native sessions, streaming diagnostics, cancellation, steering, fork, and structured output. OpenCode is **already implemented** in current main, so the Pi issue does not request a new OpenCode adapter.
- [`cmd/gimble/run_prompt.go`](../../../../cmd/gimble/run_prompt.go) exposes the simple one-turn CLI path and documents its model routing. A Pi route should be available there and in normal role binding, with tested help.
- [`docs/definition-of-done.md`](../../../../docs/definition-of-done.md) requires functionality to be seen working, and separates authoritative execution results from diagnostic event gaps. Live Pi/Router claims need a real tool-use turn with a supplied key; the no-key mock only proves local protocol handling.

## Pi-specific correction

Pi's RPC `clone` duplicates the current active branch into a new native session at its current position; RPC `fork` branches from a selected earlier user message. Gimble's `HarnessAdapter.Fork` calls for a copy of the conversation so far, so test `clone` first and preserve both independently usable sessions. The RPC response alone may not identify the new session; verify the child with `get_state` and an independent follow-up. See [RPC commands](https://pi.dev/docs/latest/rpc-commands#clone).

Pi's `agent_settled` is the idle boundary after a prompted run; a successful `prompt` response only acknowledges acceptance. `Steer` is queued between model calls and should report `landed=false` if the Gimble turn has ended or the command did not queue. To cancel a turn, clear pending steering/follow-up messages before `abort`, then wait for idle or terminate the child on a bounded deadline. See [RPC lifecycle](https://pi.dev/docs/latest/rpc#run-lifecycle) and [commands](https://pi.dev/docs/latest/rpc-commands#steer).

Use one supervised `pi --mode rpc` child per live session as the initial implementation choice. Keep its configuration and native sessions in Gimble-owned directories, rather than the user's personal Pi profile, and bind the selected provider/model and working directory to the session. This avoids imposing the Codex app-server process topology on Pi. Measure startup and memory before adding a pool.
