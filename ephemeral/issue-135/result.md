# Issue 135 live result

Implementation and live proof use codex-cli 0.153.4, Codex
`gpt-5.6-luna`, and Claude `claude-haiku-4-5-20251001`.

## Demonstrated

- `codex/codex.go` consumes `rawResponseItem/completed`, and `codex/events.go`
  uses each native tool call and its matching output as the response ownership
  boundary. This keeps delayed and serial tools together until their native
  call outputs settle, then closes the step before the next response begins.
- `codex/events_test.go` covers a delayed tool, tools that settle serially, and
  a following response whose first high-level item is another tool.
- The production `web.NewRuntime` handler rendered three Codex turns, a Codex
  forked turn, and Claude Haiku. The third Codex turn issued a file change and
  command execution from one raw model call; both appeared in one assistant
  row and the following final answer appeared in a distinct response row.
- Codex first-turn `session.step.streamed` seq 7 has response
  `resp_05e477b55b7c99d0016aa8553e226087d1bb2242f9da30ede0`; its delayed tool
  success seq 11 has the same normalized assistant message and response ID.
- Codex second turn on the same shared-daemon connection
  `session.step.streamed` seq 33 has response
  `resp_05e477b55b7c99d0016aa85542e0bc87d1bfa775a7d97c96d6`; its delayed tool
  success seq 37 has the same normalized assistant message and response ID.
- The serial-tool response's file change success seq 62 and command success
  seq 66 share normalized assistant message
  `msg_Y29kZXguMQ_00000000000000000056`. Its native completion seq 67 binds
  response `resp_05e477b55b7c99d0016aa85546e1f087d1a0e0400521260add`, and the matching
  raw call output ends that step at seq 68. The final answer uses message
  `msg_Y29kZXguMQ_00000000000000000070` and response
  `resp_05e477b55b7c99d0016aa8554e347c87d19d7ad1c6adda8370`.
- The browser rendered those tool calls in their token-bearing response rows:
  first `10784 in / 93 out / 40 reasoning / 9984 cache read`, second turn
  `748 in / 93 out / 11 reasoning / 20224 cache read`, and serial tools
  `962 in / 192 out / 69 reasoning / 20224 cache read`.
- Claude Haiku's tool marker and final marker rendered through the same
  production handler; run lifecycle seq 13 records the exact model.

Browser screenshot:
https://pub-49d826f028c94744bb6d55c4a63b56ed.r2.dev/proof/2026/09/14/7c35f185-9828-4d4b-80e7-b0b8bfff182b-final.png

Browser snapshot, assistant rows, and captured SSE frames:
https://pub-49d826f028c94744bb6d55c4a63b56ed.r2.dev/proof/2026/09/14/b3dee6f8-4846-4bc7-be28-b2d76a3299ad-browser.json

Local live artifacts:

- Project: `/var/folders/lt/09rsy64x65s_0fp2b8zq3n7m0000gn/T/gimble-live-issue135-ZO0lAh`
- Run: `01M2GRSTD1JBM874VCZJJB4MVG.observation-proof`
- Codex events: `runs/01M2GRSTD1JBM874VCZJJB4MVG.observation-proof/sessions/codex.1.jsonl`
- Forked events: `runs/01M2GRSTD1JBM874VCZJJB4MVG.observation-proof/sessions/forked.1.jsonl`
- Claude events: `runs/01M2GRSTD1JBM874VCZJJB4MVG.observation-proof/sessions/claude.1.jsonl`
- Browser capture: `browser.json`
- Screenshot: `final.png`

## Unmet: exact raw events after native resume and fork

The second Codex `Generate` in this live run reused the adapter's existing
shared-daemon connection. It proves that later turns retain raw events, but it
does not exercise `thread/resume`. The adapter invokes `thread/resume` only when
its connection dies and it redials; no shared daemon or production process was
stopped to manufacture that condition. Exact raw response events after that
redial remain unproven here.

The forked Codex turn completed and rendered both markers, but emitted no
`rawResponse/completed`; consequently it has no `session.step.streamed` event
or native response ID. Its session log contains only the high-level item stream
and the adapter's turn-finalized step.

This is an app-server protocol capability gap, not something the adapter can
truthfully synthesize:

- `codex app-server generate-ts --experimental` from installed codex-cli
  0.153.4 generated `experimentalRawEvents` only in
  `/tmp/gimble-135-schema.aMjpQp/v2/ThreadStartParams.ts:76`.
- The generated `/tmp/gimble-135-schema.aMjpQp/v2/ThreadForkParams.ts` and
  `/tmp/gimble-135-schema.aMjpQp/v2/ThreadResumeParams.ts` have no such field.
- The current upstream protocol likewise declares the field only on
  `ThreadStartParams`:
  https://github.com/openai/codex/blob/main/codex-rs/app-server-protocol/src/protocol/v2/thread.rs#L2481-L2488
- The upstream thread state enables raw events only when a caller reaches
  `try_ensure_connection_subscribed` with `experimental_raw_events=true`:
  https://github.com/openai/codex/blob/main/codex-rs/app-server/src/thread_state.rs

`codex/codex.go:211-217` already sends `experimentalRawEvents: true` on
`thread/fork`, but the installed protocol silently ignores that unknown field.
Rebuilding the fork as a new raw-enabled thread would require reconstructing
model-visible history; this was deliberately not done because it would be a
substantial semantic workaround outside issue scope. No response ID was
fabricated and token-usage notifications were not used as completion.
