# Sprint: Native session state and snapshot-first observation

Status: working path delivered locally, uncommitted. See execution/DELIVERY.md for demonstrated behavior and explicitly deferred original claims under the latest shipping-priority direction.

Issue: [#130](https://github.com/tylergannon/gimbal/issues/130).
Planning base: f79a864903f1387ea9e3b55f098340a828c1a483.
This is an unnumbered artifact, as requested. It does not create a sprint
ledger, require an interview, or authorize a commit.

## Pyramid Index

- **L0:** Make Gimbal's session observation foundation correct and usable:
  native events reduce identically in Go and JavaScript, a snapshot contains
  everything needed to continue, and a real consumer survives reconnect.
- **L1:**
  - Borrow one exact OpenCode contract, including producer invariants.
  - Establish differential, restoration, and independent semantic evidence.
  - Integrate continuous reduction, atomic snapshots, and incremental SSE.
  - Demonstrate the complete path with a small consumer and real adapters.
  - Keep unrelated OpenCode UI, controls, and global application caches out.
- **L2:**
  - [Promise and scope](#overview)
  - [State and ownership](#architecture)
  - [Work and ownership](#implementation-plan)
  - [Required behavioral evidence](#definition-of-done)
  - [Resource measurements](#performance-proof)
  - [Dependencies and unresolved defects](#dependencies-and-open-questions)

## Overview

**Done means a caller can feed real agent events into Gimbal, obtain a
complete current snapshot without replaying history, subscribe to subsequent
updates, and reconstruct the same state after reconnect. The implementation
must demonstrate those properties through its actual consumer.**

A building block can have observable behavior without a polished product
feature. Its public observations are state, event order, resource costs,
errors, and the consumer's rendered result. Tests that exercise the complete
reducer contract are direct proof of that component. They do not alone prove
runtime integration, SSE, hydration, or provider normalization; those claims
have separate demonstrations below.

This sprint includes the smallest vertical observation path needed for issue
130: the shared session state contract, JavaScript and Go reduction,
normalization from existing adapters, current-state ownership, snapshot-first
SSE, SSR/hydration, and a minimal read-only session view. Completing only the
reducer milestone is useful progress but does not complete this sprint or
close #130.

### Included

- Ordinary existing Generate invocations through the Codex and Claude
  adapters, with separate run, conversation, invocation, message and tool
  identity; concurrent sessions and later turns must remain separate.
- Text, reasoning, tool input/activity/result, model-step and execution
  status, retries/errors, and correctly scoped usage.
- Session metadata and pending inbox state required to continue accepted
  events after snapshot restoration. The session projection covers the
  pinned updater's session branches, including compaction/revert behavior
  where such events are accepted; accepting a schema is not sufficient
  evidence that a branch has been implemented.
- Same-revision upstream source, a reproducible executable oracle, fixture
  provenance, and a differential runner.
- In-memory live observation, consistent snapshots/checkpoints, bounded
  subscriptions, and a small real Svelte consumer in the existing web stack.

### Excluded

- A workflow editor, control dashboard, permission-reply UI, complete
  OpenCode frontend, specialized rendering for every upstream tool, or
  additional provider adapters.
- OpenCode's project/location catalogs, editor state, optimistic command
  outboxes and other write-side UI caches. These are not part of the
  read-only session observation claim.
- Cursor recovery, a reconnect replay buffer, filesystem polling as live
  transport, pagination/windowing that silently omits canonical state,
  compressed application envelopes, or a new event vocabulary.
- Durable process resumption of a running agent. Finished-run inspection
  uses its checkpoint; startup recovery of persisted observation records
  is distinct from resuming the agent.

## Use Cases

1. Open a running session late. Its existing text, reasoning, tools, pending
   input and status appear from current state before new events arrive.
2. Watch a tool stream input, start running and finish. The same item changes
   state; the final output does not create a duplicate tool.
3. Receive final text different from the streamed draft. The final value
   replaces the draft exactly once. A completed part does not prematurely
   finish the model step or invocation.
4. Disconnect during text, tool input, running tool or pending input.
   Reconnect replaces the previous view with a full snapshot and continues
   without missing or duplicated content.
5. Observe two simultaneous invocations with deliberately reused native IDs.
   Their events, statuses, tool results and usage remain isolated.
6. Keep a slow or abandoned browser connected. Agent progress and other
   browsers continue; the slow subscription is closed and can resnapshot.

## Architecture

### Upstream authority

Pin OpenCode at **c55ee2a8152603f04a409163bd3edf79c425fbd7**:

| Concern | Authority |
| --- | --- |
| Event definitions | packages/schema/src/session-event.ts, event-manifest.ts and matching protocol definitions |
| State definitions | Matching session-message, session-inbox and session schemas/client declarations |
| Executable behavior | packages/client/src/solid/data.ts, actual createData and its event listener |
| Rendering interpretation | packages/session-ui/src/timeline projection and row renderer |
| Producer ordering | packages/core/src/session/runner/publish-llm-event.ts |

Retain MIT text, including Copyright (c) 2025 opencode, complete commit/blob
pins, dependency lock and attribution in ports. The tested workspace uses
Bun 1.3.14, Solid 1.9.15 with the upstream patch, and Effect 4.0.0-rc.112.
Do not combine the older dev standalone reducer with this schema.

The delivered oracle must run without the ignored inspiration checkout.
Vendor the required source/dependency closure or provide an immutable,
hash-checked preparation step. Ordinary regression execution then runs from
that prepared dependency set; it must not follow a moving branch or fetch
source during each test. A fresh workspace must be able to reproduce it.

### State ownership and identity

The runtime owns one ordered reduction/publication path per run. The shared
internal package imports neither the root workflow package nor web.
The root package can publish to it; web can read/subscribe to it. Preserve
the repository's one NewHandler composition for tests and the binary.

Dispatch keys are (run, conversation, invocation); native message and tool
identities are local to that projection. Conversation history groups its
invocations without flattening their identities or accounting. Record
native-to-normalized identity mappings explicitly. Parent tool-call identity
is ancestry, not the result's own identity. No new workflow-facing names are
required.

The public observation snapshot contains run/session relationships and complete
accepted consumer state: session information, messages, execution status,
pending inputs, retries, usage and every field needed to continue its
reduction. SSR and the first SSE frame use this same shape. Private indexes
and connection generations are rebuilt locally on restore.

The internal checkpoint contains that observation state plus server-owned
restartable read obligations and its internal revision. Authority references
and server execution bookkeeping do not implicitly become browser DTOs.
Both representations derive atomically from the same canonical state/cut;
there is no independently maintained transcript model. Test server restore
from the checkpoint and browser restore from the public snapshot separately.
A browser snapshot that cannot continue its accepted events is incomplete.

Absence, null, arbitrary nested JSON and ordered arrays preserve upstream
meaning. Do not narrow open JSON merely to fit a generator.

### JavaScript, Go and effects

The untouched Solid createData is the behavioral oracle. It is not a
standalone pure reducer and cannot simply be pasted into a Svelte component.
Its session handler/helpers are the source for a framework-independent
TypeScript session projection and its Go port. Label that extraction as an
adaptation. Compare **both** the extracted TypeScript and Go implementation
against the original, not merely against each other.

Keep domain events unchanged. Represent required hydration/refresh work at
an explicit internal effect boundary, with read requests and their supplied
results. The runtime resolves authoritative session reads from current
runtime-owned data, not log replay. Separate unrelated global cache behavior
from session effects. A missing authoritative source for a required read is
a contract defect to resolve before integration, not permission to return an
empty response.

Compare immediate state and relevant settled state/effect order. A request
promise or Solid proxy is not snapshot data. At a snapshot cut, pending
required reads must be represented as restartable obligations with stable
ownership; restore reissues them against the authoritative source. Old
generation replies cannot mutate replacement state. The every-cut proof
must establish equivalent eventual state while accounting explicitly for
those reissued reads.

For every-cut restoration, compare logical obligation identity (invocation,
source event/cut, read kind and target) and eventual state. Permit one reissue
of an outstanding read after each restore, with at most one active request
per obligation generation. A restored process need not reproduce the old
network request ID or count. Preserve dependency order; any retry beyond the
single reissue must be explicitly required by that fixture's failure policy.
Duplicate completions cannot apply twice, and replies from older generations
cannot apply at all. Use these normalization rules for restored-versus-
uninterrupted effect transcripts; ordinary three-way prefix tests still
compare the same scheduled logical requests and responses.

The snapshot must not wait for all future activity to become idle: a
long-running stream must still permit a connection. Independent current
projection state and outstanding obligations are captured atomically.

### Producer invariant

The schema carries text/reasoning ordinals, but the selected client updater
edits the latest matching part. The native producer rejects a second open
text block or a second open reasoning block. Preserve this invariant in
normalization. Mixed text/reasoning and multiple tool IDs remain possible.

Keep a schema-valid overlapping-ordinal fixture as a documented oracle
counterexample. It must fail the producer-boundary invariant check, rather
than drive a silently corrected reducer. If a real adapter recording cannot
be represented without losing supported streaming behavior, stop that
mapping and return the concrete counterexample to the manager. Do not
buffer arbitrarily, rewrite identity, or change event semantics unnoticed.

### Observation and persistence

The SSR snapshot carries a stream identity and exact transaction position.
Each SSE delta groups every frame caused by one accepted input and advances the
position once. Reconnect resumes a retained suffix or explicitly sends a
replacement snapshot when the cursor is stale or unavailable.

Register the subscriber and capture a detached snapshot under the same
ordering boundary used for reduction. Events accepted after the cut queue
behind that snapshot. Bound the queue by count and bytes. Overflow or write
failure closes that subscription; it must not drop selected deltas and
continue. The next connection starts from another full snapshot.

SSR reads the same state shape. Its rendered snapshot and hydration value
agree; a newer first SSE snapshot can replace them. A connection generation
blocks stale callbacks and stale asynchronous read results.

Raw logs remain history. Reduce continuously in memory and checkpoint
detached state atomically with its internal revision. Checkpoint cadence
must not serialize the growing state for every token. If persisted tail
recovery is used at process startup, it runs once before observation becomes
available; page connections never perform recovery. Preserve main's
run-owned log finalization and surfaced recording failures.

## Implementation Plan

**Three implementation phases.** The earlier six-phase breakdown separated
setup, implementation and verification too finely. Source preparation belongs
with reducer work; live and scale proof belongs with the working consumer.
Each phase ends at a useful integration boundary, with its relevant proof
performed by the implementers as they work.

| Phase | Concrete result | Session-sized worker assignments | Verification included in the work |
| --- | --- | --- | --- |
| 1. Borrow and port session state | Pinned upstream oracle, adapted TypeScript and Go reducers, explicit effects, serializable/restorable state and shared fixtures | Manager resolves the remaining state/effect interfaces from existing research. One smaller worker owns TS extraction and oracle preparation; another owns the Go port. Both use the same fixtures and preserve native event shapes. | Schema validation, every-prefix three-way parity, independent semantic assertions, negative controls and every-cut restoration. |
| 2. Connect runtime observation | Existing adapters feed the run-owned state store; consistent checkpoints and snapshot-first SSE work through the actual server | One worker owns Codex/Claude identity normalization and recorded fixtures; another owns runtime wiring, observation store, checkpoint/subscription and server endpoint. Their interface is the event placement/state contract from phase 1. | Native mapping and accounting, concurrent invocation isolation, atomic handoff, queue bounds, cancellation and absence of per-connection history replay. |
| 3. Exercise the actual consumer | Minimal Svelte run view uses SSR and live snapshots, renders incremental state and recovers on reconnect | One worker owns the page, client lifecycle and production-path browser driver. A second can run live adapter and scale cases as soon as that path works. | Deterministic uninterrupted Gimbal-to-browser demonstration, hydration/reset, cheap live runs and prescribed resource measurements. One final independent evidence review covers the complete sprint. |

### Why this fits agent sessions

A worker assignment is a focused implementation session over one coherent
surface with its tests, not an entire phase assigned to one agent. The two
workers in a phase can proceed in parallel after the manager settles their
small shared interface. The phase boundary exists because the next consumer
needs that result; it is not another planning/review cycle.

The first phase is the largest uncertainty: adapting the Solid updater's
session effects may reveal more coupling than research established. Start
from the existing pinned source, probes and fixtures; do not repeat the
research. If a worker cannot finish because of a concrete dependency or
context limit, continue from its code and exact failing case. Report that
remaining work rather than silently growing scope or manufacturing extra
phases. Three phases is an execution estimate, not a guarantee that every
worker will finish in one context window.

The manager resolves API questions during implementation. Workers must not
revert one another's files. Interface changes require notifying affected
workers, not launching a new general review process. No commits are
authorized.

### Verification cadence

- Implementers run the focused checks that prove their changed behavior in
  their own work session. There is no separate validator or approval round
  at each phase or worker handoff.
- Keep successful evidence while the implementation and its assumptions
  remain unchanged. Rerun affected checks after changes or failures; do not
  rerun the whole corpus merely because another phase finished.
- Run the assembled integration/live/scale proof when the actual consumer
  works. An independent validator reviews the full evidence once at sprint
  completion. Material fixes receive focused re-verification of affected
  claims, not an automatic restart of the sprint or all reviews.
- The eleven acceptance claims below describe behaviors, not eleven phases,
  eleven commands, or eleven review rounds. One well-designed run can prove
  several claims, and the same evidence can support all of them.

## Files Summary

These are proposed ownership boundaries; generated files are changed only
through their generators.

| Area | Intended work |
| --- | --- |
| third_party/opencode/ | Narrow pinned oracle/schema source, upstream license/provenance and dependency preparation manifest |
| internal/sessionstate/ | Go types, projection, explicit effects, restoration and differential interface |
| web/src/lib/sessionstate/ | Adapted TypeScript projection, Svelte integration, snapshot/stream lifecycle |
| internal/observation/ | Run-owned store, consistent snapshot/checkpoint, subscriptions and resource bounds |
| run.go, session.go, event_persistence.go, events.go, harness.go | Runtime event wiring and necessary identity inputs; preserve workflow API and completion semantics |
| codex/ and claude/ | Raw identity preservation and native normalization |
| web/runtime.go, web/server.go, web/src/routes/ | Existing production composition, snapshot/SSE observation and minimal read-only page |
| internal/runlog/reader.go | Keep history/recovery role; remove per-available-record delay where it affects recovery |
| internal/sessionstate/testdata/ | Sanitized captured fixtures, explicit synthetic fixtures |
| Generated schemas/bindings | Regenerated only when source definitions change |

There is no proof program: the live run is watched and reported, never
committed.

## Definition of Done

**All claims below are mandatory for this sprint.** Each evidence entry
starts outstanding. The implementation may not label the sprint complete
from aggregate test counts, screenshots alone, or upstream research.

| ID | Claim | Witness and required observation | Failure that must be detected |
| --- | --- | --- | --- |
| C01 | Exact borrowing | Source/blob manifest, license, locked dependency preparation and clean-workspace actual createData execution | Moving revision, altered oracle, missing notices, dependency workaround existing only in a developer checkout |
| C02 | Semantic conformance | Schema-decoded fixtures; actual upstream JS, adapted TS and Go compared after every event, immediately and after required effects; explicit expected content/status assertions | Wrong target, missing branch, dropped optional JSON, duplicated completion, equally wrong local reducers |
| C03 | Snapshot sufficiency | For every fixture cut, restore Go from its internal checkpoint and the browser reducer from the public snapshot, then apply the suffix; compare later prefix states and logical effect results using the explicit reissue rules | Missing pending item/index/retry/input, shallow aliasing, stale read result, reset that only works after completion |
| C04 | Native identity and accounting | Sanitized recordings from both existing adapters; multiple turns, reused IDs across invocations, interleaved tools, retry/interruption and usage owners | Tool-result mismatch, ancestry used as identity, conversation/turn mixing, totals counted twice, fabricated unknown values |
| C05 | Producer discipline | Normalized recordings pass single-open-part invariants; deliberate overlapping same-type input is rejected/diagnosed at normalization | Schema-valid ordering that silently targets the wrong part |
| C06 | Atomic handoff | Barrier-driven events at registration, capture, serialization and first-write boundaries; assert actual frames and consumer state | Event absent from both snapshot and suffix, event applied twice, snapshot mutated after its declared cut |
| C07 | Reset and SSR | Real server-rendered state with JavaScript initially disabled; hydrate then connect; disconnect during partial/pending work and reconnect; inject late old-generation callback/read reply | Hydration disagreement, delta accepted before snapshot, append-to-old-state reset, stale generation mutation, terminal status inferred from part completion |
| C08 | Subscriber isolation | Stalled writer overflows configured byte/count bound while a second subscriber and producer advance; reconnect converges; cancellation releases owned resources | Unbounded queue, producer blocked on browser, silent delta loss, cancelled observation stopping the workflow |
| C09 | Real consumer | One uninterrupted deterministic producer → actual Gimbal normalization/store → production SSE → native EventSource → adapted reducer → Svelte DOM path | Fixture injection into browser/store, file replay masquerading as transport, separate backend and browser runs presented as end-to-end proof |
| C10 | Live adapters | At least one ordinary tool-using successful run per existing adapter, plus controlled interruption on one; capture raw native events, normalized events, stream and rendered result | Simulated provider passed off as external integration, no visible consumer, false completion, lost tool/turn identity |
| C11 | Cost and history independence | Prescribed scale workload, allocation/latency/wire report and instrumentation proving zero per-connection history reads | Full transcript sent per delta, whole history processed for each delta or connection, unbounded subscriber backlog |

### Fixture corpus and controls

Every fixture records origin, upstream revision/provider version, initial
state, ordered decoded events, controlled API responses and expected
observations. Label recorded, upstream-test-derived and synthetic fixtures
separately. Preserve useful payload structure while removing secrets.

Cover every accepted session branch and guard with at least one fixture;
coverage is a reviewed event matrix, not a percentage alone. Include
authoritative text/reasoning/input differing from accumulated fragments;
multiple sequential same-type blocks; mixed content and multiple tools;
tool success/failure/progress; retry, step end/failure and execution end;
inbox enqueue/change/deliver/cancel; metadata/usage; compaction/revert if
accepted; missing targets and absent/null/nested JSON.

Use actual upstream tests and backend capture as seeds, not the entire
corpus. Current research's 14 immediate prefixes do not establish full
coverage, effects or restoration.

Prove the checks can fail: deliberately drop a delta, append a full final
value instead of replacing it, corrupt a tool ID, remove a pending item
from a snapshot, and deliver duplicate/old-generation read replies in isolated test inputs. Each must trigger its intended
assertion. Do not modify production or upstream oracle code for these
negative controls. Include a deliberately wrong authoritative expected
value so a three-way no-op comparison cannot pass by itself.

### Minimal witnessed demonstration

Use a deterministic fixture adapter at Gimbal's existing harness boundary
to generate a known ordinary workflow. It must flow through production
normalization, reduction, snapshot/SSE and the actual page. It emits
reasoning/text fragments, executes a small harmless local tool, yields a
final value different from the draft, and terminates the step and invocation
separately. Open the page before generation. Disconnect during active work,
produce more events while disconnected, and reconnect. Run a second
invocation concurrently with reused native IDs.

Record server frames, stage-specific DOM assertions, initial/streaming/final
screenshots, program exit and final server/browser states. The synthetic
adapter is explicit; the real adapter runs then establish the integration
facts synthetic events cannot establish. A model need not expose reasoning
on demand: deterministic proof covers reasoning rendering, while the live
report states exactly what its provider exposed.

Use Codex gpt-5.6-luna and Claude Haiku for live attestation and record exact
resolved model/version. A credentials/provider failure leaves that live
claim unmet. Do not silently substitute simulation, close #130, or keep
making unbounded retries.

## Performance Proof

These are proposed acceptance budgets, not measured results.

- Use at least eight simultaneous invocation projections, each retaining
  128 completed 4 KiB text blocks plus tools, pending/status and metadata.
  Feed at least 100,000 mixed incremental events across them.
- For history-scaling measurements, keep the active part at the same size
  and compare 128 versus 2,048 completed blocks. Delta wire content must
  contain only placement and the changed native event, never cold history.
- Record raw-log bytes, reduced snapshot bytes, SSR bytes/time, first-SSE
  snapshot bytes/time, bytes per event, Go/TS reduction cost, browser
  event-to-DOM latency, allocation growth and subscriber queue high-water
  marks. Report hardware/runtime versions and event sizes/rates.
- At 100 events/second aggregate on the documented local machine, target
  p95 receive-to-DOM latency below 100 ms and first snapshot-to-visible state
  below 2 seconds for the eight-invocation workload after warm-up.
  Reduction must not scan or clone cold history per event; inspect the path
  and use history-scaling allocation/cost measurements to detect this.
- Saturate the configured subscriber count/byte bounds and prove they are
  respected independently of total retained history. Producer-owned
  canonical text can grow with output; subscriber backlog cannot grow
  without bound.

No compression ratio is promised. Measurements must show whether reduction
actually shrinks the chosen recordings. If budgets fail, report the failed
claim and evidence; do not silently adjust workload or thresholds after
seeing the result.

## Verification and Evidence Delivery

Define one documented proof entrypoint with separate conformance, runtime,
live and scale modes. Commands may differ by repository tooling, but every
mode must name the claims it demonstrates and return failure when they fail.
It must build required web assets before invoking NewHandler/browser tests.

Run gofmt, the affected generator checks, Go tests/vet and race checks for
the changed concurrency paths, TypeScript checks and the relevant browser
suite. Those are supporting checks; they do not replace the claim table.

At sprint completion, one independent validator reads the accumulated
fixture provenance, checks that the
actual production composition was exercised, inspects the staged browser
observations and negative controls, and writes a verdict against every
claim. For uncommitted proof, identify the base revision and a source-content
manifest; a HEAD SHA alone cannot identify dirty code.

The final evidence manifest records each claim as demonstrated, failed or
not run, with command, input, actual observation and artifact path. Keep
the reproducible test inputs/runner with the code. Store bulky run artifacts
under the project proof area and make them reviewable; future publication
requires the user's authorization. No commits are authorized by this plan.

## Risks and Mitigations

| Risk | Mitigation and stop condition |
| --- | --- |
| Upstream schema is broader than its producer contract | Borrow and test both; retain overlap counterexample; escalate a real unrepresentable recording |
| Both local reducers share a translation bug | Compare each against untouched actual createData and independent expected observations |
| Solid extraction loses effects or hidden state | Explicit effect/state inventory, settled comparisons and every-cut future equivalence |
| Snapshot introduces a second incomplete state model | Derive internal checkpoint and complete public observation snapshot from the same canonical state/cut; test both restore paths |
| Existing adapters lose identity/raw fields | Preserve them at the native parse boundary; demonstrate mappings from recordings before reducer integration |
| Root/web dependency cycle or proof-only stack | Shared internal package and the same NewHandler used by binary and tests |
| Scope expands to rebuilding OpenCode | Limit to accepted session projection, existing adapters and read-only observation; no unrelated catalogs/controls |
| Live model output is nondeterministic | Deterministic boundary fixtures prove edge behavior; cheap live runs prove actual integration; report absent reasoning honestly |
| Timing tests pass by luck | Barriers force races; timing is used only for the separately measured performance claims |

## Dependencies and Open Questions

- #117 and #122 overlap native event identity and schema work; coordinate
  ownership before editing adapters/events. This sprint includes the minimal
  changes its supported observation path needs, not unrelated adapter work.
- Existing skgo generated assets and development/production topology must
  build normally. Reuse the existing server composition.
- The source-backed boundary in [contract.md](contract.md), the exact
  [port findings](port-contract.md) and local ignored runtime research (`ui-runtime.md`)
  are prior art. None starts a Gimbal acceptance claim as demonstrated.
- Required session hydration data sources and exact extraction scope are
  decisions the manager settles at the start of phase 1. Record them in the
  event matrix before dependent implementation starts; this is not a
  separate planning or approval phase. A branch cannot be called
  supported while its required read is stubbed away.

No further user interview or numbering decision is required to use this
artifact. Implementation discoveries can produce concrete contract defects;
the manager resolves them against the agreed scope and records the decision.
They are not permission to invent event meanings or weaken proof.

## Completion Decision

The reducer milestone is done when conformance, independent semantics and
snapshot-future equivalence pass for every accepted branch. The sprint is
done only when the runtime, real consumer, live-adapter and resource claims
also have inspected evidence. Until then, report the completed milestone
and outstanding claim IDs plainly. Issue #130 remains open.

## Execution adjustment: proportionate proof

User challenged excessive proof machinery during execution. Keep every-prefix
borrowed-reducer conformance, focused atomic handoff/queue/cancellation checks,
one actual browser streaming/reconnect demonstration and cheap live adapter runs.
Replace the prescribed 100,000-event browser campaign and its fixed workload/timing
budgets with a focused bounded-queue and short-vs-long-history measurement that can
expose per-delta full-history work. C11 remains a performance/history-independence
check, but the earlier stress campaign is no longer required. No additional review
rounds. Existing successful component proof is reused unless affected code changes.

## Latest scope priority: ship the working path

User explicitly directed shipping working software and deferring edge-case bugs.
Finish current ID/frontend repairs, supported SSR, one production browser
stream/reconnect demonstration and a cheap live adapter run. Do not block this
on additional edge-case inventories, scale campaigns, sidecar cleanup, or review
machinery. Preserve existing successful proof. Document remaining limitations
plainly rather than claiming every originally planned acceptance item is complete.
No commit authorization has been added.
