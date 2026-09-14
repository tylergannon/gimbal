# Issue 130: join the stream from durable reduced state

## Authoritative requirement

Tyler clarified and approved this direction in this task: the entire point of
the data store is to make joining and rejoining the data stream faster and
easier. Avoid reconstructing application state from the entire event history
whenever a saved reduced state can serve as the starting point. This includes
opening a run after a server restart. Each snapshot identifies exactly the
events it includes; recovery applies only newer events through the reducer.

This direction supersedes the conflicting decisions in the old issue-130
contract and issue 169: removing durable snapshots and requiring a fresh full
snapshot on every connection are not the target behavior. No further scope
confirmation is needed. Preserve complete state; do not silently window history.

## Implementation

1. Extend the existing observation store and Go/TypeScript reducers. Establish
   an ordered event position covering all changes in the public observation,
   including lifecycle and agent updates across concurrent sessions and turns.
   Keep invocation identity isolated. Define precisely when an event is durable,
   reduced, published, and covered by a snapshot; implement that ordering rather
   than merely attaching an unrelated counter to snapshots.
2. Save consistent reduced snapshots and their recovery positions atomically.
   Use a bounded, deliberate persistence cadence that avoids serializing the
   entire growing state for every delta. Save final state on clean completion.
   After restart, restore the latest saved state and reduce only its suffix.
   Locate that suffix without scanning the full historical log. Preserve raw
   history. A run with no saved snapshot may bootstrap once from its logs and
   save the result. Keep this a small extension of the current store.
3. Carry the snapshot cursor through the actual SSR load and hydration payload.
   Subscribe from that position with a race-free handoff: updates during SSR,
   serialization, hydration, and subscription appear exactly once. Stream
   incremental changes, apply them with the existing browser reducer, and track
   the last fully applied position. Define cursor granularity so related frames
   cannot be partly applied and then skipped on reconnect.
4. Resume reconnects from the browser's cursor. Retain a bounded suffix for
   recovery; if it is unavailable, explicitly replace browser state with the
   latest snapshot and continue after its cursor. Handle process restart and
   stale connections without reusing ambiguous positions. Slow subscribers must
   neither block producers nor silently lose updates. Resolve the old runlog
   reader's role without using filesystem polling as live page transport.

## Validation and completion

- Exercise snapshot restore plus suffix reduction against uninterrupted state,
  in both reducers, including interleaved turns, partial content followed by
  authoritative finals, and usage updates. Reuse existing fixture/parity tests.
- Prove hydration-time arrivals, cursor reconnect, unavailable-cursor resnapshot,
  and server restart produce no missing or duplicate text. Test interrupted
  snapshot publication and ensure the recovered cursor matches recovered state.
- Measure short versus long concurrent histories: raw-history and reduced-state
  bytes, bytes actually read during recovery, SSR payload and join time, stream
  bytes per delta, and browser update time. Establish that an available snapshot
  avoids full-history recovery and a delta does not transmit full state.
- Use the production handler and browser to show snapshot-first entry, live
  updates, reconnect, and restart. Use Codex gpt-5.6-luna and Claude Haiku for
  live attestation and name the exact models. Keep measurements in proof code.
- Run checks appropriate to the changed Go, TypeScript, and SSR paths; record
  concrete evidence and limitations. Update conflicting ephemeral contract
  material. Do not edit docs/ without express permission. Commit and push
  meaningful progress; prepare a reviewable PR with results.

## Starting points and ownership

Implementation owner: Sol. Work in this existing isolated worktree on branch
`codex/issue-130-durable-snapshots`. Relevant code: internal/observation/,
internal/sessionstate/, internal/runlog/, runtime event ingestion, the SSR load
in web/src/routes/runs/[runID]/page.server.go, and web/src/lib/observation/.
Existing repeatable browser proof is under ephemeral/research/issue-130/proof/.
Read the current repository instructions and public contract before changing
code. This plan authorizes implementation and validation, not a new application
state model or unrelated features. Escalate only a concrete unresolved product
decision; routine persistence and synchronization choices belong to the work.
