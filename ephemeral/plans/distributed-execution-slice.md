# Distributed execution: first vertical slice

Status: DRAFT — task definition under review; not approved for implementation.
Owner: this chat, `01a19a10-29da-7a23-9998-634a063f0a57`.
Product worktree: `/Users/tyler/.codex/worktrees/1430/gimbal`.
Inspected production baseline: `a3ee508b8e7a1582971ce03170e5695fadedf118`.
Later task-definition commits change only notes/reviews, not that production source.

## Goal and authority

One registered ordinary-Go workflow runs on computer A, performs a task on A,
sets context values, and places a session on computer B that uses those values.
A and B use one selected River/Postgres communication boundary. A Docker Compose
deployment hosts Postgres on an owned instance on A, Bob, or another agreed host.
River clients/workers run in participant processes; a separate River broker is
not required. The logical intermediary may share a physical computer with A or B.
The workflow body does not move when it places a task.

Tyler selected this scope in this chat on November 1, 2026. He explicitly removed
enforced network denial from final acceptance, permanently. Independent review
must instead attest to absence of circumvention using source, configuration and
observed connection evidence. Peer task/runner addresses are not supplied to participants;
the configured database service address is allowed even if its host also runs A or B.
Unsuccessful direct connection attempts also count as circumvention.

Design inputs, all local files:

- `/Users/tyler/src/gimbal-view/ephemeral/distributed-execution/interface-migration-proposal.md`
- `/Users/tyler/src/gimbal-view/ephemeral/distributed-execution/current-architecture.md`
- `/Users/tyler/src/gimbal-view/ephemeral/distributed-execution/backend-condition-and-configuration.md`
- `/Users/tyler/src/gimbal-view/ephemeral/distributed-execution/transition-plan.md`
- `/Users/tyler/src/gimbal-view/ephemeral/reviews/20261031-distributed-proposal-review-synthesis.md`
- `/Users/tyler/src/gimbal-view/ephemeral/issues/architecture-seam-merge-gate.md`
- `/Users/tyler/.codex/worktrees/oct31-work-recap/gimbal-view/ephemeral/worklog/20261101-work-recap.md`
- Repository `AGENTS.md`, Godoc and `docs/definition-of-done.md`.

Current user instructions override older network-isolation requirements in those
documents. Their remaining whole-application merge gate is not silently waived.
This bounded milestone can be demonstrated on its isolated branch without claiming
the complete migration or permission to merge a partial repair.

Proposed checkpoint boundary, pending Tyler's approval: manually started workers
on A and B; no conversation-placement migration or orchestrator-provisioning
migration. This is an unmerged implementation milestone of the full proposal,
not a release candidate. Existing unrelated routing code remains only where not
replaced by this slice; it supplies no operation in this specimen. No compatibility
shim is added to preserve a replaced River schema or protocol. Legacy conversation
routes are not certified here and cannot use the new namespace-based River
composition until their later migration; reject that unsupported composition
explicitly rather than silently losing their selected placement. Approval of this
task includes this intermediate limitation, not a waiver for eventual publication.

## Requirements

R1. Workflow source selects logical environments for tasks. The root has a declared
environment; child scopes inherit it and may select another. Session creation
captures placement; follow-up turns and controls retain the admitted owner.
Physical addresses and connection details belong to plugin configuration.

R2. The registered definition/graph describes every referenced environment without
running the workflow body. Pure preparation and configuration validation reject
missing/indeterminate names before workflow effects. No implicit local fallback.

R3. Configuration selects one messaging implementation and opaque plugin payload.
The plugin owns parsing, schema, defaults, validation and namespace interpretation.
One overlay mechanism applies: object keys merge recursively, arrays/scalars/null
replace; switching plugin identity replaces its payload. Validate the final payload
and reject unknown fields. Existing sessions retain their owner after rebinding;
new independent work uses the current binding. Initially one active worker per
logical environment, with duplicate active registration rejected.

The plugin-configured worker identity is stable across process restarts and is
bound to its retained native state directory. Registration history alone does not
mean a worker is active. One serving instance holds database-owned exclusive
registration authority (a session advisory lock is sufficient), and only that
instance may admit its addressed work. A concurrent second instance, including one
using the same configured identity, cannot take over while the first still owns
that authority. After the database releases the departed instance's authority,
the same configured identity may register again and recover its retained sessions
without manually deleting a stale registration row. If authority is unresolved,
wait within the worker's startup budget or fail explicitly; do not assume death
or promise immediate restart. No application heartbeat/lease-expiry scheme is
required. A distinct worker identity cannot claim an already-bound logical name
without an explicit configuration rebind; rebinding preserves old session owners.

R4. Local and remote command/session tasks in this specimen cross the same selected
messaging contract. No task-endpoint fallback, per-peer connection construction,
shared network filesystem or SSH tunnel supplies missing participant operations.
Provider/repository connections used by ordinary tasks are separately classified
external services, not permitted alternative participant channels.

For the new River composition, named addressing belongs to the selected transport.
Manually started workers register in its namespace; no orchestrator is required
or selected. Reject this composition if it also supplies an orchestrator,
`Route.Environment`, a task endpoint or another legacy destination/routing source,
before constructing capacity or executing work. Workflow declarations are its
sole placement authority. Do not construct `environmentMessaging`, call
`EnvironmentRouting`, or launch a local worker as a fallback for it. Do not add a
dummy orchestrator, a new native protocol or a task-only replacement bus.

`EnvironmentRouting`, `environment_messaging.go`, `Route.Environment` and the local
orchestrator remain unchanged for existing out-of-slice compositions. Their global
removal is a subsequent migration milestone, including the conversation workflow's
runtime-selected placement. Nothing in this checkpoint certifies those existing
paths. The final proposal still requires their replacement; the bounded branch
checkpoint does not turn them into approved exceptions to the final architecture.

R5. A local command produces a fresh nonsecret value. The workflow stores it with
the existing ordinary context API. A real remote native session consumes it and
returns a verifiable derived result; a follow-up turn demonstrates the same
session/owner. Existing inheritance/shadowing semantics remain intact. Include a
large context value that uses the existing complete-value artifact representation:
the remote session must read its complete content without access to A's files.
Explicit input/output bytes use the selected boundary; bytes need not be embedded
in observation events. No automatic source/checkout preparation.

Working directories are explicit workflow inputs interpreted on the execution
owner, including native absolute paths when appropriate. They are file-location
inputs, not placement selectors. The specimen supplies existing owned directories
for A and B; ordinary commands prepare any required files. Preserve the requested
native path in replay intent; do not apply A's `filepath.Abs` rules to B's request.
No daemon-wide working-directory override or new directory API is required. This
follows the review synthesis's withdrawal of workspace management as a prerequisite.

R6. Retain distinct facts for request storage, native-owner acceptance and native
completion. Stable run/scope/session/request/native identities tie them together.
Each new-work request explicitly names its logical environment; requests for
admitted sessions address their retained owner. Physical destinations still belong
to plugin configuration. Amend `MessageRequest` and its destination comment to
distinguish logical placement from physical connection details.
Ownership acceptance is atomic and retained before an ambiguous reply can cause
another dispatch. A queue delivery acknowledgement alone is not native completion.

R7. Restart the task participant while preserving its configured durable state.
It recovers the same accepted task identity: if the native owner/result survives,
reconcile it; if the outcome cannot be established, report an explicit unresolved
state and do not blindly execute replacement work. This slice does not promise
resuming arbitrary killed computation, replacing a lost workflow runner, automatic
failover or a timing guarantee. At least one restart specimen must recover a real
completed native result whose return to A was interrupted, under the same identity.

Native dispatch owns recovery publication: on worker startup, reload accepted
message dispositions, reconcile their original native tasks and publish retained
completions without requiring A to resubmit or restart. Persist the original
completion event identity and republish it idempotently if delivery was interrupted.
An accepted but unsettled task uses its existing native mapping; missing evidence
produces an unresolved completion. Transport only delivers these owner-authored
facts. A accepts late accepted/completed records by identity, including a completion
whose acceptance record was missed. Existing taskdaemon persistence is reused.

R8. The workflow computer receives truthful placement, native outcomes and retained
observations through the selected boundary. Disconnect/reconnect its observation
subscription while tasks continue; replay catches up and joins live delivery
without duplicate task admission. Record identity/deduplication cannot rely only
on synchronized wall clocks. Results and referenced bytes remain retrievable.
For this slice, full matching-request/run replay plus live delivery is sufficient:
execution waits must not filter worker records using A's publication timestamp.
Any retained `since` option is a diagnostic filter, never the execution-completeness
boundary. Record identity and per-source sequence define deduplication/source order;
no total order across publishers is promised. Change and document the shared
listener contract and adapt all implementations/callers affected by that change.

The River transport owns reconnect for an execution wait as well as an observer.
A transient database/connection failure triggers bounded, observable reconnect and
full identity-based catch-up; it does not resubmit a request or settle native work.
Use the caller's cancellation/deadline and a finite plugin-owned reconnect budget
(default 30 seconds of reconnection effort after loss, configurable in the plugin
payload); honor whichever ends first. This is a resource bound, not a task-completion
timing promise. Permanent authentication/configuration/schema errors fail explicitly.
On budget exhaustion, return delivery uncertainty with the original identities;
retain the native result for later reconciliation. Successful reconnect must allow
an already-running call to receive its original authoritative completion.

R9. Independently review the final candidate and exercised composition. Trace all
participant communication for this slice, including failure/recovery paths, inspect
effective configuration and observe connections for the whole specimen. Attestation
names exact source/build/plugin identities, inspected paths, exercised operations,
allowed external services and evidence gaps. It cannot certify unexercised product
paths. No firewall/partition experiment is required now or later by this task.

R10. Recorded logical environment and admitted worker/machine identity are visible
for each command and session on the run page. Mixed placement must not be labelled
as one local/run-wide destination. Reconnect and logical-name rebinding preserve
the recorded placement of admitted work. This is a bounded addition to existing
command/session details, not a page redesign. Inspect actual rendered pixels before
claiming human legibility. Existing out-of-slice run-wide display is not removed.
For a run using this new composition, the mapped observation registry's decoration
uses retained per-task placement to replace its task summary with the actual single
environment or a mixed summary, overriding the old host-derived `local` label.
It does not infer task location from the workflow's computer. The unchanged legacy
summary remains only for out-of-slice runs. The accepted/completed message facts
carry the admitting worker's identity and logical environment, and A records them
as lifecycle facts so replay builds the same task/session placement rows.

## Cells, contracts and implementation volume tripwires

These are responsibility boundaries in existing code, not new framework packages.
File ownership below covers known required work; adding an owning production file requires
updating the map before implementation. Changes crossing it are review signals.
Tests and generated code are excluded from production-volume counts. Count all
physical lines with `wc -l` in each cell's listed production files plus any approved
new owning production files; deletion removes a file from the count. Budgets are
stop-and-report tripwires, not a request to fill them.
Directory patterns below cover direct children only; exclude `*_test.go` and
`*_gen.go`. Regenerate generated outputs through their owning generators.

| Cell | Responsibility and seam contract | Initial production files | Baseline / maximum lines |
| --- | --- | --- | --- |
| Declaration/admission | Produce a complete environment inventory and validate effective selections before effects; pass logical placement into task dispatch. | `env.go`, `registry.go`, `workflow/*.go`, `internal/generate/{declarations,expr,stmt,graph,definition,source,inspect}.go` | 3,262 / 3,900 |
| Transport | Resolve logical destinations and retained owners; durably store requests/facts/bytes; atomically accept registrations/ownership; provide replay/live delivery and bounded reconnect through selected River connection. Own database details, never native outcome interpretation. | `messaging_registration.go`, `backend/messaging.go`, `backend/backend.go`, `backend/messagefiles/store.go`, `plugins/messaging-river/main.go`, `plugins/messaging-filesystem/main.go`, `plugins/messaging-http/main.go` | 1,343 / 2,250 |
| Native dispatch | Admit commands/session operations under stable identities, reconcile original native outcomes, republish recovered completion and retain session owner; report uncertainty honestly. Native harness remains outcome authority. | `internal/effects/*.go`, `internal/taskdaemon/*.go` | 2,606 / 3,300 |
| Workflow context/results | Preserve ordinary scope semantics and explicit bytes; submit selected commands/session operations and return outcomes to local Go control flow. | `session.go`, `task_files.go`, `command.go`, `scope.go`, `artifact.go`, `remote_execution.go` | 2,729 / 3,100 |
| Participant bootstrap | Propagate selected transport/bootstrap to manually started workers and registered invocation; reject legacy routing/provisioning mixed into this composition. | `resource_registration.go`, `backend/orchestration.go`, `internal/invocation/invocation.go`, `internal/runner/runner.go`, `cmd/gimbal/messaging.go`, `cmd/gimbal/workflows.go` | 910 / 1,300 |
| Observation/display | Project retained per-task placement truthfully and show it in existing command/session details and mixed-run summary; observation remains distinct from native execution outcome. | `events.go`, `internal/observation/{rows,store,snapshot,registry}.go`, `web/src/lib/run/{RunHeader,RunTranscript,SessionTimeline}.svelte`, `web/src/lib/workspace/projectHome.ts` | 2,044 / 2,400 |

The registered catalog, dispatch envelopes, native admission state and retained
record/byte references are the seams. Their reason to change differs: authoring
and input validation; delivery/storage; native execution; workflow context use;
participant bootstrap; observation/display. Bootstrap changes with deployment,
not task outcome semantics; presentation changes do not authorize native work.
Transport cannot decide native success. Workflow context cannot open peer routes.
Any required production change outside this map is a spec-defect intake before
coding, not an exemption. Filesystem/HTTP changes here adapt shared contracts;
their adaptation does not certify their full distributed lifecycle.

## Central semantics (language-neutral)

    destination = inherit_or_select(scope.logical_environment)
    session.owner = admit_once(session.identity, destination.current_owner)
    request = (stable_identity, session.owner_or_destination, operation, input)
    append_request_once(request)
    accept_atomically(request.identity, native_owner.identity)
    reconcile_or_execute_at_that_owner(request)
    retain_native_outcome_and_output_references(request.identity)
    publish_retained_facts(request.identity)

    on_worker_restart:
        load_original_assignments_and_native_identities()
        reconcile_existing_owner_or_retained_result()
        retain_original_completion_event_identity()
        publish_owner_outcome_idempotently_even_without_redelivery()
        if_evidence_insufficient_publish_unresolved_without_replacement()

Queue retries can redeliver an envelope; native admission must reconcile the
identity. No claim of exactly-once arbitrary external side effects is made.

## Observable definition of done

Each claim is mandatory for this milestone, with its required observation.

| ID | Class / cells | Claim and demonstration |
| --- | --- | --- |
| D1 | Cell-local / declaration | Metadata includes both logical environments; invalid or absent names reject before any workflow/native effect. Exercise registered preparation/admission, not a separate invented preflight utility. |
| D2 | Cell-local / declaration | Overlay semantics, strict plugin validation and namespace interpretation agree; invalid selection produces an explicit error without fallback or capacity startup. |
| D3 | Cell-local / transport | Duplicate active registration is rejected, including a concurrent instance with the same identity. After its predecessor's database authority is released, the restarted worker re-registers with its original identity and retained sessions without stale-row cleanup; an unresolved predecessor cannot be replaced. Requests/facts retain stable identities; repeated delivery does not create another accepted assignment. Rebind a logical name: new work selects the new owner and existing sessions still address the original owner. After transient connection loss the listener delivers the original completion from retained records; budget exhaustion returns uncertainty without resubmission. Exercise the actual database implementation. |
| D4 | Cell-local / native dispatch | The original native identity governs acceptance and outcome; uncertain replies reconcile that identity and never authorize replacement work. |
| D5 | Cell-local / context | Visible inherited/shadowed context and complete-value artifact bytes retain existing semantics at the task-input boundary. |
| D5a | Cell-local / bootstrap | Prestarted registered workers can receive named work without an orchestrator. Mixed-in legacy destination/provisioning settings reject before effects; missing/failed registration fails at use without another route. Out-of-slice conversation composition using namespace-based River fails explicitly, preserving the selected environment rather than falling back. |
| D5b | Cell-local / observation | Recorded task/session placement projects into the same rows after live delivery and retained replay; rebinding cannot rewrite admitted placement. |
| D6 | Integration / all | One registered workflow on A runs a real local command, then a real session on B consumes its fresh context, reads complete transferred context and returns the derived result; a follow-up uses the same session. Show actual machine and native-session identities. |
| D7 | Integration / transport, native, context | Worker restart after native completion but before return to A recovers that result under its original identity, with one native execution; absent native evidence remains explicit unresolved. |
| D8 | Integration / transport, native, context | Observation disconnect/reconnect recovers recorded facts and new activity while work continues; final outcomes and referenced bytes are retrieved through the intermediary. |
| D9 | Integration / all | Independent source/configuration/connection review attests no circumvention in the exercised composition, or refuses acceptance and identifies the gap. No enforced network denial. |
| D10 | Integration / native, context, observation | On the running application page, a person can identify the actual environments of A's command and B's session; mixed placement is truthful and remains so after observation reconnect. Inspect actual pixels and report what was seen. |

Compilation, focused/race tests and affected application checks supplement these
claims. They do not replace D6–D10. Proof is running the real application and
reporting what was seen in this chat and the PR. Repository instructions prohibit
committing proof programs, screenshots, logs or run dumps; tests live beside the
code and review/spec notes remain under `ephemeral/`.

## Staged plan and gates

1. Task-definition stage: reconcile current source and these requirements, settle
   remaining user decisions, mechanically review this draft, obtain independent
   adversarial consensus, then request explicit Tyler approval of task and DoD.
2. Implementation checkpoint: Terra 6.1 implements declared placement, selected
   River delivery and retained ownership in this product worktree. Put repeatable
   checks beside their owning code, including an opt-in integration test in the
   package that checks registered invocation/task dispatch. That test registers an
   ordinary workflow and uses real separately started message-worker participants
   and the actual River plugin/database for live claims; mocks cannot satisfy D6–D10.
   Do not commit a standalone specimen/proof program or relabel it as an example.
   Run focused checks and D1–D5b.
3. Live checkpoint: reserve two real computers and one isolated Compose database
   with their owners, then demonstrate D6–D8 using that opt-in integration test
   and D10 in the running page for its actual run.
   Start a message-worker on A for its local tasks and a message-worker on B for
   remote tasks. The test/workflow process remains on A; the observer is independently
   reconnectable and does not own task execution. Use unversioned `luna` for Codex live
   specimen sessions (or `haiku` if Claude is selected); report actual model.
4. Independent Sonnet validation examines application behavior and D1–D10; retain
   fresh review artifacts and the reviewer session for consensus. Failure needing
   source repair returns to Terra, then re-review the complete candidate.
5. Report proved revision/claims and unmet full-migration requirements. Do not
   merge/publish until the remaining applicable whole-application gate is met or
   Tyler explicitly approves changing it. This milestone alone is not that gate.

The requested implementation/validation model choices are Terra 6.1 and Sonnet.
If a supported harness cannot supply either, report the limitation; do not
substitute. Keep coordination in this chat, including any Gimbal run. No extra
coordinating chat. Gimbal-owned agents operate in this product worktree.

## Exclusions

- Enforced network denial, firewall rules and network-partition proof.
- Kafka, a new queue platform, worker pools, matching or a general scheduler.
- Automatic source sync, managed repository/worktree layouts, reachability preflight.
- Arbitrary computation resumption, automatic runner/engine failover or live migration.
- New cancellation-completeness ledger, retry-cleanup subsystem or timing promises.
- Data retention/expiration policy, quota management or cleanup framework.
- Completing every transport's migration, independently deploying console/engine/
  runner, or claiming the broad architecture gate passes from this slice alone.
- Migrating runtime-selected conversation placement, deleting global legacy routing,
  or migrating the local orchestrator. Their eventual migration remains required
  before the complete architecture repair can be certified/merged.
- Confidential scoped-value transport in the specimen unless Tyler adds it;
  ordinary context is the current proposed reading of his request.

## Remaining approval decisions / deferred tripwires

Before implementation approval:

- Confirm ordinary `Set`/`SetJSON` context is sufficient for this specimen;
  confidential stores are a different capability.
- Proposed single authoring addition for Tyler's explicit approval by name:
  `gimbal.WithEnvironment(ctx, "logical-name")` returns a decorated context, using
  the semantics already present internally. The workflow decorates its root
  context with a constant default and decorates a child context for other work;
  ordinary Scope/NewSession/RunCommand remain the composition. No helper workflow
  wrapper. All environment names must be statically enumerable. The authoring
  change plus required graph/catalog/configuration fields must be approved before
  implementation; no additional exported function names are authorized.
- Explicit public contract amendments included in this approval request:
  `MessagingDefinition` exposes plugin-owned schema/namespace interpretation and
  registration/retained-owner addressing. Existing connection/dependency data
  carries the selected namespace capability to dispatch; no orchestrator signature
  migration is part of this checkpoint. Extend existing message record/selection contracts with
  logical-environment/retained-owner request addressing, stable source
  sequence/identity and selected-boundary byte delivery. Extend lifecycle records
  and observation rows with admitted per-task placement as required
  above. Exact internal layout is derivable; no untyped generic RPC or new public
  workflow function beyond `WithEnvironment` is approved.
- Configuration uses the existing Route envelope (`Name`, `Configuration`) and
  plugin-owned logical-name/worker-identity bindings. `--task-route @file` accepts
  one Route object or an ordered array of Route objects; normalize a single object
  to a one-element composition, then apply the same R3 overlay rule in order.
  This is the sole overlay mechanism, not separate full/partial modes. The final
  effective Route is passed through registered invocation, and the plugin receives
  only its validated payload. Concrete payload serialization and
  private credential references are derivable implementation details, except any
  materially new public contract must return for approval. Namespace addressing
  and native outcome ownership remain as specified above.

Before live work: assign A/B/database location, OS choice and owned processes;
confirm credentials/build tools without exposing secrets. Two computers are
required; Windows qualification is not silently mandatory for this first slice.

Integration constraints: #665 owns resource-lifetime repair; do not modify its
branch or shared runtime services. If shared interfaces change, identify exact
base revisions and merge dependencies with that owner before integration.
Parent chat A has released both #664 integration holds following candidate proof
and merge. That grants no new merge/publication permission. No shared-runner or
native-control experiment is currently reserved by this task. No production or
qualification installation is to be changed for this milestone.
