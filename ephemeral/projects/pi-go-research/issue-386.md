# Implement Pi coding-agent harness with Diffusion Router support

https://github.com/tylergannon/gimbal/issues/386

## Goal

Add **Pi** (`@earendil-works/pi-coding-agent`) as a Gimble coding-agent harness and make a Diffusion Router open-weight model usable in a real Gimble turn. Start with `diffusion/deepseek-4.1-flash`, the model in the Router's Pi guide. This is a new Pi adapter; current `main` already has an OpenCode adapter. Gemini, Claude Code, and Codex changes are outside this issue unless a small shared binding change is necessary for Pi.

Tyler will provide a Diffusion API key to the implementing agent for live qualification. Do not put the key, bearer headers, or a credential-bearing config in the issue, repository, logs, or proof artifacts. Build and no-key tests can start before the key arrives.

## Current Gimble integration points

Work against the latest `origin/main` and read `AGENTS.md`, `skills/gimble/SKILL.md`, and `docs/definition-of-done.md`. As checked on `81dc3c8`, the active code is the root Go module:

- `harness.go`: `HarnessAdapter` has `CreateSession(ctx, model, effort, workdir)`, `RunTurn(ctx, sessionID, prompt, schema, onEvent)`, `Steer`, `Fork`, and `Close`. `RunTurn` cancellation is through its context. There are no adapter `Interrupt` or `Compact` methods.
- `session.go`: Gimble creates native sessions lazily, validates returned JSON against the requested schema, and re-asks on invalid output a bounded number of times. The adapter returns a JSON string for prose or raw JSON for schema turns.
- `internal/modelalias/modelalias.go` and `internal/binding/binding.go`: explicit model routing and adapter construction. Follow the existing explicit OpenCode route with an unambiguous Pi route such as `pi/diffusion/deepseek-4.1-flash`; keep provider and native model ID bound to the session. Wire it through normal role binding and `gimble run-prompt --model`, with help and tests.
- `opencode/adapter.go` and `opencode/events.go`: current examples for the adapter contract and Gimble event/usage projection. Pi has a different process model; no shared OpenCode server is needed for it.

## Pi and Router facts already checked

The current Pi package tested in a disposable local installation on 2026-09-25 was `@earendil-works/pi-coding-agent` **0.87.1**. The former `@mariozechner/pi-coding-agent` package is deprecated. Pin or verify the exact installed version before relying on a particular RPC event/command shape.

Pi's documented `--mode rpc` is a long-lived JSONL subprocess interface. A successful `prompt` response acknowledges acceptance; it does not finish the run. Consume stdout continuously and finish a turn at `agent_settled`, while treating provider errors, process exit, and cancellation as actual failures. `agent_end` alone is not the final idle boundary. Correlate command responses by request ID, and keep stderr separate from protocol stdout. [Pi RPC protocol](https://pi.dev/docs/latest/rpc), [RPC commands](https://pi.dev/docs/latest/rpc-commands), [JSON events](https://pi.dev/docs/latest/json).

The Router's Pi guide configures a compatible endpoint in Pi's `models.json`:

```json
{
  "providers": {
    "diffusion": {
      "baseUrl": "https://router.diffusion.io/v1",
      "api": "openai-completions",
      "apiKey": "$DIFFUSION_API_KEY",
      "models": [{ "id": "deepseek-4.1-flash" }]
    }
  }
}
```

Use a Gimble-owned `PI_CODING_AGENT_DIR` and `--session-dir`; put only the literal environment-variable reference in `models.json`, and pass the actual key to the child environment. Avoid modifying the user's personal Pi setup. Pi project resources and instructions may still load from the selected workdir; keep that behavior intentional. [Pi model configuration](https://pi.dev/docs/latest/models#configure-a-compatible-endpoint), [Pi configuration](https://pi.dev/docs/latest/configuration), [Router Pi guide](https://connect.diffusion.io/connect?guide=pi).

A **no-key localhost mock** verified that Pi 0.87.1 parsed this provider shape, sent `POST /v1/chat/completions` with a fake Bearer key and `stream: true`, emitted ordered RPC events, and resumed the same conversation in a new Pi process via `--session-id`/`--session-dir`. This did **not** call Router or prove real tool use, schema output, steering, or cost. Pi also sent `max_completion_tokens`, `store`, and `stream_options`; verify that Router accepts those fields. The minimal model entry silently defaulted to 128K context, 16,384 output tokens, `reasoning=false`, text-only, and zero costs in that local Pi build. Those defaults are **not** verified Router metadata. [Research handoff and portal archive](https://github.com/tylergannon/gimble/tree/codex/diffusion-router-research/ephemeral/projects/gimble/diffusion-router-research).

## Implementation behavior

1. Implement a Go Pi adapter with one supervised `pi --mode rpc` child per live Gimble session as the starting topology. Run it in the session workdir, scope its config and session storage to Gimble, preserve native session identity, and clean up on `Close`. Keep a process across successive turns; verify that a replacement process can resume its saved conversation. Do not add a process pool before measuring a need.
2. Map `RunTurn` to a Pi `prompt`, the complete message/tool event stream, a terminal `agent_settled`, an authoritative final assistant message, and truthful per-model usage. Project Pi tool starts/ends and assistant output into `AgentEvent` without treating a diagnostic event gap as a successful turn failure. Surface genuine provider errors, missing final output, unexpected child exit, and cancelled context. Handle an `onEvent` error according to Gimble's current adapter convention.
3. Implement `Steer` with Pi's queued `steer` command. Report `landed=false` when no Gimble turn is active or the message cannot be queued before the turn ends. Pi delivers a steer after current tool calls and before the next model call; do not promise immediate interruption. On context cancellation, `clear_queue` before `abort`, then wait for idle or terminate the child on a bounded deadline.
4. Implement Gimble `Fork` as a full current-conversation copy with independently usable parent and child. Pi RPC `clone` duplicates the current active branch into a new session; Pi RPC `fork` instead branches from a selected *earlier user message*. Investigate `clone`/`get_state` and prove two separate follow-up turns, rather than assuming the Pi command names match Gimble's semantics. `Close` must release only its own session and be idempotent.
5. For a schema turn, Pi core RPC has no documented `outputSchema` parameter. Supply the requested schema in the prompt, extract one JSON result, and let Gimble's existing validator/re-ask loop enforce exact validity. Do not claim native schema enforcement. If a Pi extension is needed, keep it narrowly scoped and prove it works through Router's tool-call path. For text turns, return the final message encoded as a JSON string.
6. Honor explicit effort only when supported by verified model metadata. Do not force a default `high` thinking level onto the minimal Router model entry. Keep provider/model, credential source, workdir, and native session storage isolated so concurrent Pi and first-party harness turns cannot cross-route or leak keys.

## Acceptance and proof

- Tests with a local fake Chat Completions server cover config loading without a real key, JSONL framing and command correlation, prompt-to-`agent_settled` completion, tool/event projection, usage, schema validation/re-ask, steering race, cancellation and queue clearing, fork independence, unexpected child exit, close, and resume in a new Pi process. Pin the Pi version used for the tests.
- With Tyler's supplied key, run **one real Router-backed Gimble workflow/turn** that reads a file, edits a file, runs a shell command, and returns a valid result. Record the actual Pi version, Router model ID, observed event sequence, usage, and sanitized request compatibility. Also demonstrate a schema turn and a second turn resuming the same session. If the selected model cannot complete this loop, report the exact failure and qualify a different Router coding model rather than marking the issue done from a smoke prompt.
- Exercise `gimble run-prompt --model pi/diffusion/deepseek-4.1-flash` and one normal role binding. Verify concurrent isolated sessions, cancellation, and fork against the built application. Follow the repository's relevant build/test gates and independent validation; describe observed proof in the PR or issue without committing logs or keys.

This issue is complete when the adapter is implemented **and the requested live behavior has been seen working**, per `docs/definition-of-done.md`. A no-key mock is useful preparation but does not establish Router compatibility.

