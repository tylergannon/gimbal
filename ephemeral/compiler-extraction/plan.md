# From the compiler specimen to a consumer-owned backend

September 29, 2026. Proposed graduated plan for discussion, based on Gimbal
`6aee0a14`, the shared-operation evidence and Tyler's annotations on the portability
review. This is not approval of API names, a universal backend framework, a
hosting provider, or implementation of all stages. The next checkpoint is the
component/ownership analysis, before API publication.

The [Stage 1 component analysis](compiler-component-analysis.md) reached ownership
consensus with Claude Fable 5.1 (only nitpicks remain, round 04). The separate
[API sketch](compiler-api-sketch.md) also reached consensus (only nitpicks remain,
round 03), as Tyler requested. Both remain proposals for the design check-in,
not approved public signatures or implementation.

## The outcome

An organization's coding agent can build and maintain its compiler using Gimbal's
published semantic tools, instructions and examples. Its authored workflows
retain the supported language, model/harness behavior, resource/context behavior,
and UI meaning. The organization owns the emitted orchestration, infrastructure
and maintenance of that compiler. We can be that organization, using Kubernetes
in a separate consumer module. A third-party volunteer is not required.

Gimbal supplies neither a supported catalog of infrastructure emitters nor a
promise of identical backend durability. It should make its own semantics
dependable and sufficiently accessible that consumer agents do not have to
reverse-engineer them.

## Clarified starting state

- Authored Go already feeds compilation and independent graph extraction. No
  UI disconnection was found. Current paired/source-mutation checks demonstrate
  selected source/event associations; current browser legibility remains unobserved.
  Update from Stage 1 ownership review: production dependency listing establishes
  that the specimen omits the authored Planning package and its graph registration;
  tests include it. This is a newly found registration defect, distinct from the
  still-missing browser evidence. See the component analysis for reproduction.
  The following review also traced the actual console cancellation command: it
  cancels the worker runtime but bypasses the specimen's Temporal cancellation
  endpoint. The proposed host boundary now includes consumer control routing.
  Tyler clarified that whole-run cancellation is required, while per-task
  cancellation may be removed if its value does not justify the semantic cost.
  The compiler proposal uses a run-only cancellation hook; task-kill continuation
  is not a readiness gate. Existing control removal remains a separate decision.
- The specimen already emits visible orchestration. Encourage this through
  examples, review and narrowly useful linters; do not make all consumer output
  readability a universal Gimbal guarantee. Preserve observable workflow meaning.
- Accepted-byte execution plus typed consumption is useful shared functionality,
  not a compulsory transport format. A consumer may pay for typed round trips,
  provided supported values/errors/agent behavior survive unchanged.
- The numeric issue is not a missing top-level scalar feature. The actual
  specimen is `Numeric struct { Count int8 }`: its pinned Polytype validator
  accepts `{"count":128}`, `{"count":1.0}` and `{"count":1e0}`, while the
  corresponding Go decoder rejects them. An object wrapper does not fix it.
  This is demonstrated in `TestPolytypeSchemaDecoderGap`; the compiler currently
  diagnoses numeric result fields. Do not file a scalar-support request as its
  solution. An upstream follow-up, if pursued, should describe validator/decoder
  range and notation agreement. It is not a gate on component analysis.
- A dependable result contract means executable behavior: for the admitted
  type/version, bytes accepted by the operation can be consumed as that type
  without an additional model attempt. Documentation names the rule; code must
  uphold it. Start by sharing the already-written admission checks where useful,
  not by inventing a new codec system. Broader fixes may belong in Polytype.
- The private named-constant defect remains: `constant` emits an inaccessible
  source-private named type into a different package. A source-located rejection
  of unnameable types is an acceptable bounded remedy. Erasing the named type
  to its underlying type is not automatically faithful Go semantics. This should
  be a small repair/diagnostic task, not a reason to defer the architecture work.
- Backend deployment/versioning and interchangeable infrastructure are consumer
  responsibilities. Our backend must address its own needs; Gimbal need not
  establish a universal policy before publishing shared semantic support.
- Fan-out expansion, general Go support and exhaustive type support are separate
  lanes. They enter this plan only if an agreed consumer task actually needs them.

## Stage 1: understand and assign the existing components

**Result:** an evidence-backed ownership map and a small API proposal derived
from the working specimen. Each proposed component explains what behavior it
owns, why it belongs to Gimbal or the consumer, and how a compiler invokes it.
Map actual calls and dependencies, including package-cycle constraints; do not
start by renaming/exporting every `compiledscope` hook.

Use these as investigation starting points, not approved package boundaries:

| Existing code | Responsibility to analyze | Likely ownership |
| --- | --- | --- |
| `session.go`, `supervise*.go`, `compiled_generate.go`, `internal/compiledscope/generate.go` | Execute one agent operation; acceptance/re-asks, recording, supervision and optional typed consumption | Shared Gimbal behavior |
| `compiled_loop.go`, `internal/compiledscope/bridge.go`, `planning_activities.go` | Single planner dispatch, feedback, steering, run/scope/task entry and exit | Shared mechanics; consumer-emitted orchestration |
| `internal/compiledscope/context.go` and scope binding hooks | Immutable context meaning, inheritance, capture and rendering versus physical storage | Shared semantics; organization-selected storage |
| `activities.go`, `continuity_activities.go` | Ownership and cleanup rules versus Temporal heartbeat, leases and worker-local handles | Separate shared invariants from target integration |
| `internal/temporalgen/result.go` and `valueMethods` | Supported result contract, implicit methods and replay-specific restrictions | Polytype/Gimbal checks where common; target restrictions where specific |
| `internal/temporalgen/generate.go`, graph extraction, observation integration | Typed call recognition and source associations versus target emission | Share analysis only where helpful; consumer owns emitter; Gimbal owns event/UI meaning |
| `main.go`, `payload.go`, `workflowStart` | Provisioning, queues, transport, timeout/retry and deployment policy | Consumer backend |

**Ready to advance when:** we can walk one Generate, one scoped operation and
one planner task through these boundaries, explain who owns each behavior, and
show a proposed consumer call path without private imports or duplicated agent
semantics. Distinguish "extract code", "document an obligation" and "leave in
the consumer" for each item. Give an example of the resulting consumer code.

**Check-in:** review this analysis and the concrete API sketch before committing
to public signatures. We have enough evidence to begin this stage now; we do not
yet have enough analysis to publish the API responsibly.

## Stage 2: extract the smallest coherent shared implementation

**Result:** the existing Temporal/Docker specimen calls the separated components
and retains its established behavior. Public-facing design remains revisable
while actual call sites test its usefulness.

Shared Gimbal support continues to own its semantics; the specimen owns its
orchestration and infrastructure. Prefer moving/refactoring existing code to
adding parallel implementations. Centralize common result admission only to the
extent established in Stage 1. Keep unsupported shapes explicit. Repair or reject
the inaccessible constant type with a focused regression.

**Ready to advance when:** existing paired/source-edit checks still demonstrate
context capture, results/errors, task feedback, ownership/cleanup and source/event
associations, and the consumer glue contains no copied Generate retry engine or
duplicated shared semantics. Validate changed boundaries with relevant runtime
checks rather than commissioning unrelated backend features.

## Stage 3: publish a usable API and the essential compiler instructions

**Result:** a clean downstream Go module can use the selected public packages
and documented versions. The guide shows a complete small workflow, emitted
orchestration and consumer-owned integration code, with ownership clearly marked.
The current backend remains the reference example; no emitter support catalog
is implied.

Teach the agent what Gimbal guarantees, how to recognize and lower supported
operations, how to preserve source/UI associations, how to report unsupported
constructs, and how to regenerate and maintain the result. Include representative
behavioral checks a consumer can run. Explain target facts that need decisions;
do not require a general infrastructure portability layer or a deployment framework.

**Ready to advance when:** a clean downstream build imports no Gimbal `internal`
packages, the documented example runs through the public surface, a small source
edit regenerates correctly, and the guide contains the information used to do it.
Then publish the selected API and instructions through the ordinary release path.
This stage establishes usability of the surface, not the full consumer promise.

## Stage 4: define our Kubernetes consumer trial

**Result:** an agreed, bounded consumer assignment, hosted budget and acceptance
promise, separate from Gimbal's product promises. We act as the consuming
organization; its compiler and deployment are maintained as consumer code.

Choose a useful workflow and an inexpensive hosting option when ready to run it.
Decide whether to retain Temporal initially or use another controller. Reusing
Temporal would limit simultaneous unknowns, but the choice is not made here.
Choose only source/type support needed by the assignment. For example, if the
assignment needs a scoped app server, Service becomes relevant; an unrelated
fan-out or scalar feature does not become a prerequisite by association.

For our backend, investigate Tyler's preferred payload policy: serialize
business arguments/results into organization-controlled content-addressed storage
and carry references through orchestration. This works whether application code
uses typed values or raw bytes. The existing codec only externalizes payloads
at least 64 KiB; smaller values remain inline, so it is insufficient for an
all-business-payloads-out-of-history goal. Identify the actual argument, result,
failure-message and metadata paths that could carry content. Content addressing
establishes identity/integrity, not confidentiality by itself. Storage access,
retention and any remaining orchestration metadata are our target policy.

**Ready to advance when:** the assignment, supported behavior, cost boundary,
payload policy and visible UI outcomes are agreed. Hosting selection, cluster
creation and recurring expense happen here, not during API analysis.

## Stage 5: run the consumer-experience promise loop

**Promise:** using the published Gimbal API, essential instructions and examples,
a coding agent can build and maintain our consumer-owned Kubernetes compiler;
the agreed authored workflow executes with supported Gimbal semantics, and the
running UI makes its activity, trouble and supported controls understandable.

Have an implementing agent work from the consumer-facing material and a separate
consumer module. Review the implementation and observe execution/UI independently.
Include a subsequent source or target-policy change so the exercise demonstrates
maintenance, not merely a one-time build.

If implementation fails, review finds semantic drift, or substantial negative
feedback exposes a missing public tool or essential instruction, the promise is
not met. Classify the cause, improve the responsible code or documentation, and
repeat. An infrastructure-specific bug belongs in the consumer; missing shared
behavior belongs in Gimbal; a codec defect may belong in Polytype. Do not fix the
trial through undocumented private hooks or weaken its promise to obtain a pass.

**Met when:** the implementation and maintenance change work through published
interfaces, review finds no material supported-semantic defect, and the running
UI demonstrates the agreed workflow behavior and controls. Record the revisions
and concrete evidence in Gimbal View. Orchestration success alone does not prove
the generated software product's quality; keep the agreed output check separate.

This is a future Gimbal promise-loop workflow, not an active run or a loop
implementation request in this planning task. Its exact workflow and bounded
operating limits are designed when Stage 4 is ready.

## Scope of the present checkpoint

The claims and ownership corrections are recorded. The proposed next work is
Stage 1's analysis, with the narrow constant diagnostic available as an adjacent
repair. Do not start Kubernetes provisioning, design a supported backend library,
or promise broad language/type coverage as part of it.
