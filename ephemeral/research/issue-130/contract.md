# Issue 130: current implementation contract

Use OpenCode's native `session.*` event payload vocabulary for the supported
Gimbal observations. Do not add a parallel custom vocabulary or runtime schema
validation of Gimbal's own emitted events.

## Borrowed implementation

The authority is OpenCode revision
`c55ee2a8152603f04a409163bd3edf79c425fbd7`, particularly
`packages/client/src/solid/data.ts` and its matching event/message/inbox types
under `packages/schema/src/`. This newer revision has a native V2 UI; the older
standalone V2 reducer and legacy projection are not interchangeable with it.
The source is MIT, copyright 2025 opencode; retain its notices and source pin.

Port the event-application behavior used by Gimbal, including authoritative
text/reasoning/tool finals. Preserve upstream handling of optional fields,
message identity, and ordering. OpenCode's separate HTTP cache-refill/read
orchestrator is not part of Gimbal's observation reducer. The adapters serialize
same-type content blocks; overlapping same-type starts retain upstream's known
limitation, recorded by the oracle fixture.

The Go and browser reducers hold native message state. Go snapshots restore
that state in the browser. There are no read/completion obligations, generation
counters, reducer checkpoint APIs, or read/settle transport frames.

## Producer and storage boundary

Claude and Codex translate supported provider activity into native payloads.
Gimbal assigns event IDs, timestamps, and stable ascending message IDs while
preserving provider references on emitted records. Unmapped provider traffic is
ignored. There is no separate native-audit log or unused durable aggregate
sequence/version field.

A live run owns its reduced observation. Each accepted input becomes one
durable delta before publication. The store saves an atomic reduced snapshot
every 64 positions and on clean completion, together with the exact covered
delta-journal byte offset.

## Browser delivery

SSR and the JSON route read current reduced state and its cursor. An SSE
connection resumes after that cursor while its bounded suffix is retained;
otherwise it starts with a complete replacement snapshot. Capture the suffix
and register the subscriber under the same lock. Overflow closes only that
stream; reconnect explicitly resumes or replaces state.

Use skgo's existing transport hook for SSR. Streamed content updates the affected
message instead of copying all transcript content. Runtime profiling machinery
is not part of the viewer; measurements belong in the proof harness.

## Evidence

The retained event fixtures are compared after every prefix with recorded output
from the untouched pinned JavaScript reducer. This proves those event paths,
not full OpenCode cache orchestration. `third_party/opencode/prepare.sh` and its
oracle runner reproduce upstream output without tracking downloaded sources.

Use the existing production-handler browser scenario for SSR, streaming text,
reasoning, tools, authoritative finals, and reconnect replacement. Delivery
results, measurements, and admitted adapter limitations are in
[DELIVERY.md](execution/DELIVERY.md). The original [SPRINT.md](SPRINT.md) is a
historical plan; this contract and subsequent explicit user instructions
supersede its extra infrastructure and verification requirements.
