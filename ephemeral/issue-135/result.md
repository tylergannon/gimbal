# Issue 135 live result

## Remaining coverage: socket-only reconnect

The 2026-09-14 follow-up used installed codex-cli 0.153.4 and generated its
experimental app-server protocol again. `experimentalRawEvents` remains present
only in `/tmp/gimble-135-schema.sADuHV/v2/ThreadStartParams.ts:76`; generated
`ThreadResumeParams.ts` and `ThreadForkParams.ts` contain no such field. There is
therefore no supported adapter request that can enable exact raw events for a
fork or for resume after daemon restart/thread unload. No history reconstruction,
synthetic response ID, or token-notification completion inference was added.

The supported loaded-thread path is now covered by
`TestRunTurnAfterRedialResumesThread`. It closes only the adapter's active
WebSocket, waits for that reader to fail, then runs another turn. The adapter
redials the still-running shared daemon and calls `thread/resume` for the loaded
thread. The retained raw-event setting stays active on that thread.

The live Codex `gpt-5.6-luna` run
`01M2GWN0CEMBENT427Q6XVQZDJ.daemon-live` demonstrated:

- reconnect tool response `resp_03555303ced615b6016aa86505390887d1914354fae79a1aed`
  in assistant row `msg_bGl2ZS4x_00000000000000000026`, with 541 input,
  68 output, 43 reasoning, and 20,224 cache-read tokens;
- reconnect final response `resp_03555303ced615b6016aa86508ad0487d181675785ebf494e3`
  in distinct assistant row `msg_bGl2ZS4x_00000000000000000036`, with 682
  input, 11 output, and 20,224 cache-read tokens;
- the production handler rendered `CODEX_RECONNECT_TOOL_MARKER` and
  `CODEX_RECONNECT_FINAL_MARKER` in those distinct token-bearing rows.

Browser screenshot:
https://pub-49d826f028c94744bb6d55c4a63b56ed.r2.dev/proof/2026/09/14/aee82b17-41a5-4c02-af78-07972f3b38c7-reconnect.png

Browser snapshot and rendered assistant rows:
https://pub-49d826f028c94744bb6d55c4a63b56ed.r2.dev/proof/2026/09/14/76d1e379-3d32-417a-9dc7-8d49522ca6d6-browser.json

Local retained evidence:

- Project: `/tmp/gimble-live-issue135-reconnect-final.qssQZy`
- Session events:
  `runs/01M2GWN0CEMBENT427Q6XVQZDJ.daemon-live/sessions/live.1.jsonl`
- Production browser:
  `http://127.0.0.1:64094/runs/01M2GWN0CEMBENT427Q6XVQZDJ.daemon-live`
- Server PID: `85575`; stop with `kill 85575` after review.

Exact native fork events and resume events after daemon restart or thread unload
remain unmet on codex-cli 0.153.4. Issue #135 must remain open for that protocol
capability.

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
  `resp_0cbb5186d6d48d53016aa856ec680887d19e7a3e842f4672d7`; its delayed tool
  success seq 11 has the same normalized assistant message and response ID.
- Codex second turn on the same shared-daemon connection
  has tool success seq 36 and `session.step.streamed` seq 37 under the same
  normalized assistant message; seq 37 binds response
  `resp_0cbb5186d6d48d53016aa856f0cb9087d187d37e1bea53f16a`.
- The serial response completes natively at seq 59 before its first tool starts
  at seq 60 and finishes at seq 64; the second tool then starts at seq 65 and
  finishes at seq 68. Both tools share normalized assistant message
  `msg_Y29kZXguMQ_00000000000000000056`. The completion at seq 59 binds
  response `resp_0cbb5186d6d48d53016aa856f525cc87d1bd10890370847ea0`, and the matching
  raw call output ends that step at seq 69. The final answer uses message
  `msg_Y29kZXguMQ_00000000000000000071` and response
  `resp_0cbb5186d6d48d53016aa856fb4a2487d19d60bb3c45536f9e`.
- The browser rendered those tool calls in their token-bearing response rows:
  first `10784 in / 93 out / 29 reasoning / 9984 cache read`, second turn
  `737 in / 93 out / 11 reasoning / 20224 cache read`, and serial tools
  `957 in / 184 out / 78 reasoning / 20224 cache read`.
- Claude Haiku's tool marker and final marker rendered through the same
  production handler; run lifecycle seq 13 records the exact model.

Browser screenshot:
https://pub-49d826f028c94744bb6d55c4a63b56ed.r2.dev/proof/2026/09/14/f61a994f-a62c-4947-a419-a9fe1939181f-final.png

Browser snapshot, assistant rows, and captured SSE frames:
https://pub-49d826f028c94744bb6d55c4a63b56ed.r2.dev/proof/2026/09/14/72b88a90-fd59-44e4-9fcc-689fa1ded6d9-browser.json

Local live artifacts:

- Project: `/var/folders/lt/09rsy64x65s_0fp2b8zq3n7m0000gn/T/gimble-live-issue135-eBe8f6`
- Run: `01M2GS70JDCA56S184QY4KW9R2.observation-proof`
- Codex events: `runs/01M2GS70JDCA56S184QY4KW9R2.observation-proof/sessions/codex.1.jsonl`
- Forked events: `runs/01M2GS70JDCA56S184QY4KW9R2.observation-proof/sessions/forked.1.jsonl`
- Claude events: `runs/01M2GS70JDCA56S184QY4KW9R2.observation-proof/sessions/claude.1.jsonl`
- Browser capture: `browser.json`
- Screenshot: `final.png`

## Unmet: exact raw events after native resume and fork

The second Codex `Generate` in this live run reused the adapter's existing
shared-daemon connection. It proves that later turns retain raw events, but it
does not exercise `thread/resume`. Native socket-redial behavior remains
unproven by this browser run. Upstream source shows the raw-event flag remains
sticky while the thread stays loaded, so a socket-only redial should retain it.
After a daemon restart or thread unload, `thread/resume` rebuilds the thread
without a supported raw-event opt-in; exact raw response events are unavailable
in that case.

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
