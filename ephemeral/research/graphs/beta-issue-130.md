# #130: Serve reduced session snapshots for SSR, then stream incremental events

Captured 2026-09-14 from https://github.com/tylergannon/gimble/issues/130. Read beta-implementation-handoff.md for final session decisions and coordination notes.

## Problem and direction

The proposed page observation path replays an entire event log and then tails the filesystem. Every connection starts over. That makes SSR, hydration, reconnects, and multiple concurrent sessions increasingly expensive as histories grow. The current internal reader also waits 10 ms after each record, including records already available; it is currently used only by tests, not a shipped web stream.

Tyler has already encountered this problem in an earlier tool with only one session loaded in the browser. Do not repeat that architecture for a multi-session application.

## Requested behavior

1. Implement matching state reducers in Go and TypeScript. On the server, continually reduce incoming agent events into the current state of each session invocation. The core reduction should be straightforward: append each delta to its identified message/content block or tool item, and apply completion/status/usage updates to that item or invocation.
2. Publish/write the latest reduced state as an internally consistent snapshot. On initial connection, including SSR, use the latest-written state rather than replaying the raw history.
3. Stream new events to the frontend.
4. Reduce those events into the frontend state with the TypeScript reducer.

Raw events may remain durable history. The filesystem must not serve as the live transport between the runtime and its page. State ownership must distinguish the session conversation from each invocation/turn, so simultaneous invocations cannot mix their message identities or updates.

## Recommendation — implementation details not fully vetted

Include the last-applied event sequence in each snapshot and establish a race-free snapshot-to-stream handoff. A snapshot through sequence N is followed by events after N. Handle reconnect overlap without appending the same delta twice; if the requested sequence is no longer retained, resnapshot explicitly. Do not leave a gap between fetching the snapshot and subscribing.

Keep the reducers small and test both against the same event fixtures and every prefix of those fixtures. Preserve stable message/content-block/tool-call identifiers in normalized events. A completed full message must finalize or replace its accumulated value rather than append the full text a second time. Coordinate the event shapes with #122 and adapter streaming/identity work with #117; current adapters do not yet expose the complete delta contract.

Tyler raised `query.live` as a possible way to avoid dual reducers. Assess that alternative, but emitting and serializing the entire growing state on every delta would reintroduce update costs proportional to session length. The intended steady-state path sends incremental events and updates the affected frontend item. SSR and hydration must use the same snapshot boundary.

Combining deltas should substantially shrink state relative to raw event history. Treat an order-of-magnitude reduction as a hypothesis to measure, not a guaranteed compression ratio: retained message text and tool output still grow. Report initial snapshot/SSR size as well as incremental-update cost; propose any needed history window separately rather than silently dropping state.

## Demonstrate

- Go and TypeScript reach identical state for recorded adapter events, including interleaved message/tool deltas, completion after partial content, invocation boundaries, and usage semantics.
- Initial SSR/hydration renders the latest snapshot without replaying the event history. Events arriving during connection appear exactly once after hydration.
- Disconnect/reconnect recovers consistently from a sequence cursor or a fresh snapshot; no missing or duplicated text.
- Multiple long concurrent sessions remain responsive. Measure raw-history bytes versus reduced-state bytes, SSR payload/time, stream bytes per delta, and frontend update time. A new delta must not retransmit the full session state.
- Show the snapshot followed by live updates in the running browser with real harness events; name the cheap models used. Resolve the current reader's role as part of this change rather than extending its replay-and-poll contract into the web application.

Related: #124 (web runtime), #129 (bounded supervisor memory), #117 (adapter events), #122 (event unions), and PR #127. Existing reader: [internal/runlog/reader.go](https://github.com/tylergannon/gimble/blob/5ccd00ec62300ae67fedeefa2f4628d66f84cd0c/internal/runlog/reader.go).
