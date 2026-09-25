# Live run data over `query.live`: findings (2026-09-21)

Question: can the run page stop using the hand-rolled SSE endpoint (`internal/observation/http.go`, `/api/runs/{id}/events`) and use skgo `query.live`, without re-sending the whole observation per change?

## Facts established from source
- A `query.live` yield is the WHOLE devalue-encoded value in one SSE frame; only byte-identical repeats are skipped (kit `server/remote-functions.js:109`, skgo `remote_live.go:113-124`).
- Client replaces its value wholesale; `for await` is latest-wins under backpressure (kit `utils/shared-iterator.js:8-14`): intermediate frames can be dropped.
- Reconnect re-runs the producer with the ORIGINAL argument; no Last-Event-ID; skgo runs no live queries during SSR.
- One HTTP connection per distinct argument; Gimbal's server is HTTP/1.1 (`web/runtime.go:196-238`), ~6 per origin. So: one live query per page.

## Measurements (recorded runs; on-disk files are byte-identical to the wire)
- Snapshot 6.8–17.2 MB; 9k–40k deltas, median ~700–900 B, max 1.2 MB; peaks 70–120 events/s.
- Today (snapshot + deltas): 26–59 MB per run. Full value per change: 61–300 GB (2,300–5,900×).
- Full value per item: message part 1.4–3.7×; message 1.7–168×; turn 200–350×; session 535–1,100×.
- Open parts at once: max 9–13, p95 4–5. Worst bytes: `tool.input.delta`, `text.delta`, `reasoning.delta` (whole part per keystroke/token).

## POC 1: delta batches over `query.live` (`claude/live-query-poc`) — failed
Yield `{from,to,deltas}` from `{run,stream,position}`, re-subscribe on gaps. On a 6 s one-turn run the page opened 6 streams, never reached the ended state, and moved ~7× the real deltas. Kit's own reconnect plus ours fight. Dead end.

## POC 2: open-items window (`claude/live-window-poc`, 5e59aa2) — works on what was tested
The owner's idea: the event stream has commit points (part end, message end, turn end); committed items never change, so only the handful of open items needs to be live. One `query.live({run})` yields, at most every 100 ms, the complete current value of every open part plus anything changed in the last 3 s (so a dropped frame is healed by the next). The page upserts idempotently; after an outage longer than 3 s it re-runs the route load.
- Correct: live page and a cold server replaying from disk matched byte-for-byte (transcripts and rendered text). Pass.
- Skew (turn ended before its last message): none. Pass.
- Page health while streaming: navigation 43–47 ms. Pass.
- Bytes: 178–628 KB on short runs, ~10 frames/s. No baseline measured on the old path yet.
- NOT verified: deliberate dropped frames; the long-outage resync (Playwright offline didn't cut a loopback SSE stream).
- Size: `window.remote.go` 351 lines (new), `observation/index.ts` +107, `+page.svelte` +88/−44.
- Trap: a live-query instance held in `$state` re-ran effects thousands of times/s when its `connected` getter was read; `$state.raw` fixed it.

## What skgo would need to make this a one-liner
- A windowed live-query helper: "yield the current values of keys changed in the last T, plus open keys, at most every Δ".
- A transport for `map[string]T` so windows need no raw-JSON escape hatch.
- Docs: when `query.live` fits (small values, or a bounded open window) and when it does not (a growing document as one value).
