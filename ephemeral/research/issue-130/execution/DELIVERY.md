# Working observation path

Implemented and demonstrated locally. Latest user direction favors
shipping the working path and deferring edge cases rather than completing every
original stress/edge-case gate.

## Current implementation after weight reduction

- Native OpenCode event application for the events Gimble emits, with a Go port
  compared after every retained fixture prefix against recorded output from the
  untouched upstream JavaScript. Full OpenCode HTTP-cache/read orchestration is
  deliberately omitted; earlier full-conformance claims no longer apply.
- Cursor-aware SSE: SSR carries the exact reduced-state position, reconnects
  resume a retained transaction suffix, and unavailable cursors receive a
  replacement snapshot. Reduced snapshots are saved every 64 positions and on
  completion with their covered durable-journal byte offset.
- Claude and Codex translate their supported native events. Unmapped provider
  traffic is ignored; there is no separate native audit log.
- No runtime validation of our emitted events or embedded OpenCode JSON schema.
  Native provenance on emitted events and upstream license notices are retained.
- UI updates target the affected invocation/message; current proof results are
  recorded below once integration completes.

## Executed evidence

- `just build`: passed (execution/integrated-build.log). Later renderer-only builds
  passed via canonical `cd web && pnpm exec vp build`.
- `go test ./...`: passed (execution/integrated-tests.log). After enabling actual
  Codex raw-event subscription, `go test ./codex` passed again.
- `cd web && pnpm run check`: zero errors/warnings (final-web-check.log).
- Existing single Playwright scenario: passed, 2.0 seconds including runner startup
  on final execution (browser-run.log). Uses installed Chrome. Real production
  runtime, JS-disabled SSR, partial content, two invocations reusing provider IDs,
  reload/reconnect, tool progress and authoritative final replacement/completion.
- `node live.mjs codex`: passed through actual Codex gpt-5.6-luna, codex-cli0.153.4,
  actual production page and native EventSource.28 captured frames. Asserted markers
  specifically inside assistant rows, not the echoed user prompt; screenshot inspected.

Final live project (raw run/session records, browser state/frames and screenshot):
`/var/folders/lt/09rsy64x65s_0fp2b8zq3n7m0000gn/T/gimble-live-codex-uqUJeg`
Run: `20260911-200303.observation-proof`.
Screenshot: `final.png`. Browser evidence: `browser.json`.
No provider reasoning text appeared in this live response; deterministic fixture
proved reasoning streaming. Live UI correctly shows unavailable cost, observed tokens.

## Concrete fixes exposed by running it

- Open native snapshot cannot pass closed Go load type projection directly. Universal
  loads are also rejected by this skgo version. Its existing Transported hook carries
  the JSON snapshot and decodes to the exact object for SSR/browser; no dependency patch.
- Embedded SSR engine lacked structuredClone. Plain-JSON fallback supplies that platform
  operation for SSR; browser keeps native implementation.
- UI used Gimble placement ID to index native state; now uses native state keys.
- Plain reducer mutation did not invalidate Svelte content for already-existing rows.
  Renderer now creates shallow render references on revision; canonical reducer unchanged.
- Completed tool metadata duplicated final results; progress metadata renders only running.
- Codex exact response notifications require experimentalApi capability and
  thread/start experimentalRawEvents. Both enabled; no change to reducer semantics.
- Separate message/event counters misordered event-derived rows; now share ordering.

## Deferred, explicitly not proven complete

- Claude has passing adapter/SDK tests but no live browser attestation in this delivery.
- Codex resumed/forked turns need follow-up: pinned protocol exposes raw-event opt-in
  on thread/start, while the current adapter launches fresh processes for later turns.
  First-turn live success does NOT establish that path. Do not silently infer response
  boundaries from token usage to work around it.
- Exact tool-to-model-response grouping needs follow-up: native tool activity can arrive
  after rawResponse/completed; live transcript currently places such activity with the
  following displayed step. Content renders, but per-step attribution needs correction.
- Rendering now shallow-copies displayed message/part references to notify Svelte.
  Large-history rendering cost remains unmeasured; no large-scale performance claim.
- User-row status and completed-stream disconnected label are presentation quirks.
- Original interruption/scale/extended edge-case campaign and independent review round
  were not run after user's explicit shipping-priority adjustment.

Exploratory reports and execution logs are local ignored working material.
The selected contract, runnable proof, and this delivery summary are retained.

Adapter follow-up: https://github.com/tylergannon/gimble/issues/135

Runtime emitted-event schema validation and its embedded schema were removed
at the user's direction. Root, Claude, and Codex tests pass after removal.
Implementation remains under independent review and is not merge-approved.

Post-reduction integration: root/adapter/reducer/observation/web Go tests pass;
TypeScript check and production build pass; existing deterministic browser proof
passes. Raw history 32,599 B; reduced state 4,525 B; SSR 6,631 B with 8.40 ms
navigation; delta frames 694–734 B; partial delta batch visible in 20.57 ms.
These are small two-session fixture measurements, not large-history/p95 claims.
Proof-only instrumentation; no runtime telemetry was added. Artifacts:
`/var/folders/lt/09rsy64x65s_0fp2b8zq3n7m0000gn/T/gimble-observation-proof-uKJuhy/measurements.json`
and `final.png` in that directory. Final screenshot inspected by the parent agent.

Round-2 follow-through: removed unused durability metadata and reference mapping,
dead reducer APIs/counters/invariant checker, and production synchronization hooks.
`cd web && bun run test` runs both suites (8 passing tests); check/build pass.
Live Claude production page passed with `claude-haiku-4-5-20251001`, run
`20260911-210012.observation-proof`, 36 stream frames, visible tool and final
markers. Artifacts:
`/var/folders/lt/09rsy64x65s_0fp2b8zq3n7m0000gn/T/gimble-live-claude-ypqJN6`.
This closes the earlier Claude live-attestation gap; resumed/forked Codex turns
and precise tool-to-step attribution remain in #135. The browser proof now
waits for the asserted content after terminal status before capturing evidence.

Independent Claude Fable consensus completed after three rounds: round 3 outcome
`only nitpicks remain`, with no material findings. Local review artifact:
`ephemeral/reviews/issue-130-round-03.md`. No Gimble push or merge was performed.
