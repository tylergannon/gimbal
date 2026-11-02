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
observed connection evidence. Peer addresses are not supplied to participants.
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

R4. Local and remote command/session tasks in this specimen cross the same selected
messaging contract. No task-endpoint fallback, per-peer connection construction,
shared network filesystem or SSH tunnel supplies missing participant operations.
Provider/repository connections used by ordinary tasks are separately classified
external services, not permitted alternative participant channels.

Named addressing moves into the selected transport. Remove `EnvironmentRouting`
and `environment_messaging.go` rather than extending their per-destination
connections. Manually started workers are valid: named task admission does not
require an orchestrator. If an orchestrator is selected, it retains provisioning/
use/release responsibility and receives the selected transport's narrow worker
bootstrap/readiness capability. Its ready result must correspond to registration
in that namespace. Migrate the existing local provisioner to this arrangement;
remove its route-returning methods and route configuration. It cannot construct
a second transport. Do not add a dummy orchestrator for prestarted workers.
Remove run-wide `Route.Environment`: workflow declarations are the sole task
placement authority. Update its host/CLI display/help readers and runner bootstrap;
do not retain a compatibility field or silently override workflow placement.
Migrate `internal/environments` callers of the deleted routing interface to the
selected transport's logical namespace/retained-owner operations. Its selected
provisioning setup may bootstrap capacity, but application SSH tunnels/direct
worker routes cannot carry task operations for this slice. Do not add a parallel
native protocol. Any required owning-capability decision beyond that migration
returns through spec-defect intake rather than being exempted as setup code.

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
| Transport | Resolve logical destinations and retained owners; durably store requests/facts/bytes; atomically accept registrations/ownership; provide replay/live delivery and bounded reconnect through selected River connection. Own database details, never native outcome interpretation. | `messaging_registration.go`, `backend/messaging.go`, `backend/backend.go`, `backend/messagefiles/store.go`, `environment_messaging.go` (delete), `plugins/messaging-river/main.go`, `plugins/messaging-filesystem/main.go`, `plugins/messaging-http/main.go` | 1,535 / 2,450 |
| Native dispatch | Admit commands/session operations under stable identities, reconcile original native outcomes, republish recovered completion and retain session owner; report uncertainty honestly. Native harness remains outcome authority. | `internal/effects/*.go`, `internal/taskdaemon/*.go` | 2,606 / 3,300 |
| Workflow context/results | Preserve ordinary scope semantics and explicit bytes; submit selected commands/session operations and return outcomes to local Go control flow. | `session.go`, `task_files.go`, `command.go`, `scope.go`, `artifact.go`, `remote_execution.go` | 2,729 / 3,100 |
| Provision/bootstrap | Keep provisioning distinct from named addressing; propagate selected transport/bootstrap to manually started or provisioned workers and connect normal registered invocation; migrate existing environment callers from deleted routing contracts. | `resource_registration.go`, `backend/orchestration.go`, `backend/localorchestrator/*.go`, `internal/invocation/invocation.go`, `internal/invocation/backends.go`, `internal/runner/runner.go`, `cmd/gimbal/messaging.go`, `cmd/gimbal/workflows.go`, `internal/host/routes.go`, `internal/environments/*.go` | 4,316 / 4,800 |

The registered catalog, dispatch envelopes, native admission state and retained
record/byte references are the seams. Their reason to change differs: authoring
and input validation; delivery/storage; native execution; workflow context use;
capacity/bootstrap. Provisioning changes with deployment, not task outcome semantics.
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
| D3 | Cell-local / transport | Duplicate active registration is rejected; requests/facts retain stable identities; repeated delivery does not create another accepted assignment. Rebind a logical name: new work selects the new owner and existing sessions still address the original owner. A transient connection loss resumes the same execution wait and its retained records; budget exhaustion returns uncertainty without resubmission. Exercise the actual database implementation. |
| D4 | Cell-local / native dispatch | The original native identity governs acceptance and outcome; uncertain replies reconcile that identity and never authorize replacement work. |
| D5 | Cell-local / context | Visible inherited/shadowed context and complete-value artifact bytes retain existing semantics at the task-input boundary. |
| D5a | Cell-local / provision | Prestarted registered workers can receive named work without an orchestrator; selected provisioning receives the same transport bootstrap and cannot return ready before registration. Missing or failed registration fails at use without another route. |
| D6 | Integration / all | One registered workflow on A runs a real local command, then a real session on B consumes its fresh context, reads complete transferred context and returns the derived result; a follow-up uses the same session. Show actual machine and native-session identities. |
| D7 | Integration / transport, native, context | Worker restart after native completion but before return to A recovers that result under its original identity, with one native execution; absent native evidence remains explicit unresolved. |
| D8 | Integration / transport, native, context | Observation disconnect/reconnect recovers recorded facts and new activity while work continues; final outcomes and referenced bytes are retrieved through the intermediary. |
| D9 | Integration / all | Independent source/configuration/connection review attests no circumvention in the exercised composition, or refuses acceptance and identifies the gap. No enforced network denial. |

Compilation, focused/race tests and affected application checks supplement these
claims. They do not replace D6–D9. Proof is running the real application and
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
   and the actual River plugin/database for live claims; mocks cannot satisfy D6–D9.
   Do not commit a standalone specimen/proof program or relabel it as an example.
   Run focused checks and D1–D5a.
3. Live checkpoint: reserve two real computers and one isolated Compose database
   with their owners, then demonstrate D6–D8 using that opt-in integration test.
   Start a message-worker on A for its local tasks and a message-worker on B for
   remote tasks. The test/workflow process remains on A; the observer is independently
   reconnectable and does not own task execution. Use unversioned `luna` for Codex live
   specimen sessions (or `haiku` if Claude is selected); report actual model.
4. Independent Sonnet validation examines application behavior and D1–D9; retain
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
  `OrchestratorDefinition.Construct` receives the selected transport's narrow
  registration/bootstrap capability; `backend.Dependencies` carries it to workers;
  `MessagingDefinition` exposes plugin-owned schema/namespace interpretation and
  registration/retained-owner addressing. Remove `EnvironmentRouting` and
  `Route.Environment`. Extend existing message record/selection contracts with
  stable source sequence/identity and selected-boundary byte delivery as required
  above. Exact internal layout is derivable; no untyped generic RPC or new public
  workflow function beyond `WithEnvironment` is approved.
- Configuration uses the existing Route envelope (`Name`, `Configuration`) and
  plugin-owned logical-name/worker-identity bindings. Concrete serialization and
  private credential references are derivable implementation details, except any
  materially new public contract must return for approval. Namespace addressing
  and native outcome ownership remain as specified above.

Before live work: assign A/B/database location, OS choice and owned processes;
confirm credentials/build tools without exposing secrets. Two computers are
required; Windows qualification is not silently mandatory for this first slice.

Integration constraints: #665 owns resource-lifetime repair; do not modify its
branch or shared runtime services. If shared interfaces change, identify exact
base revisions and merge dependencies with that owner before integration.
Parent chat A initially released the #664 proof hold, then reserved a new short
main-merge/version-preparation hold for integrating #664. Main merges, version
preparation, stable-tag changes and release dispatch are held until A releases
that window. No shared-runner or
native-control experiment is currently reserved by this task. No production or
qualification installation is to be changed for this milestone.
