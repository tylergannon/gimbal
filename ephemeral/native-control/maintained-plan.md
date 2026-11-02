# Draft: maintained native task workflow

November 1, 2026 (America/Costa_Rica). Design note only. Tyler's entrypoint choice and the authoritative spec amendment are pending. This is not permission to launch native work, restart a service, or change the release hold. The demonstrated route is the existing Desktop worker owned by one genuine loaded Desktop task; it is not a general Gimbal harness or public Desktop API.

## Proposed product and placement

If Tyler selects the dedicated entrypoint, add ordinary Go workflow `NativeTask` in `internal/workflows/nativetask/`, register it in the built-in workflow catalog, generate its normal invocation metadata, and keep focused tests in that package. This is a requested product workflow, not a generic adapter or proof program. The new workflow name and exported entry/parameter names require the user's entrypoint decision and an explicit spec amendment: the current spec expressly calls for an existing product workflow and no new exported names. No implementation should begin on the assumption those constraints are already amended.

A workflow submits one user-specified native assignment to an already bound local worker, observes its task and reconciles it. It does not create, bind, restart, or replace the Desktop owner; decide app consent; impersonate metadata; retry an uncertain creation; or claim that a completed turn proves the user's assignment succeeded. It returns a usable task/result observation and clear cleanup state. No new exported Go transport concept, wrapper, HarnessAdapter, backend, server, or private Desktop protocol implementation.

## Input contract

The ordinary workflow has one local config-file path and reads a JSON or similarly simple document in the workflow execution environment. A separate local assignment file supplies the prompt; the caller snapshots its content for the logical attempt so accidental later changes cannot silently change an already owned request. No remote URL in a task prompt. Config explicitly specifies:

- a caller-supplied, nonempty stable logical request ID. Identical logical input and ID intentionally reconcile the existing worker ledger entry; changed prompt, target, required apps or resolved model with the same ID must surface `request_conflict`, not create a second task. New work needs a new ID. This is not an automatic retry policy;
- absolute assignment, Node executable, installed `client.mjs`, and isolated worker state-directory paths. State directory is explicit on every call, avoiding the historical default ledger. Host-local worker, Node and paths are prerequisites; no implicit cross-host file transfer;
- explicit nonempty required bundle IDs, and a valid Desktop `target` object (start with projectless only unless an actual consumer establishes a project target contract). No implied native app consent;
- a finite observation deadline and client exchange timeout, with safe lower and upper bounds chosen in the amendment. Select the unversioned `luna` through `internal/modelalias.Resolve` and pass its provider-native model explicitly; establish actual Desktop support rather than using the worker's legacy default. Effort policy and whether the user may override it need a decision.

Absolute paths, prompt/target immutability, and the exact content used to fingerprint a request must survive run re-entry. Avoid putting assignment text or raw request JSON in `RunCommand` arguments, logs or shell source. Use a private workflow-owned input file or a shell `stdin` redirect to an explicitly owned file, with quoted positional arguments; treat the request as potentially sensitive. Determine whether standard Gimbal run capture itself records command output containing assignment/results and document that visibility. Cleanup must not delete the worker ledger.

## Lifecycle and evidence

1. Validate the explicit config and matching local files. Read `status` with the declared required apps: a bound owner and saved approvals are prerequisites only, not proof of Desktop admission, TCC, native access, or task success. Report missing/error precisely; never omit `requiredApps` to pass.
2. Invoke the shipped client `start` with the explicit request identity and model. Distinguish execution error, client error envelope, malformed envelope and success. On a lost/timeout/unknown response do not infer failure or create new work. Retain request ID and config fingerprint and return `outcome_unknown`; an operator may inspect the existing worker status/ledger and Desktop. A repeated identical request is allowed only as explicitly requested reconciliation, not a hidden restart. If the ledger reports a previously cancelled entry, surface it as a settled prior attempt, not a newly successful task. Retain any known child ID before proceeding.
3. Boundedly `read` that owned child until the specific created turn has an authoritative terminal result, a failure/approval/unknown state appears, the deadline expires, or the run is cancelled. The known Desktop read shape contains `thread.status`, `turns[]` with an individual `id`, `status`, and `error`, and optional output items; active thread alone is not success. `inProgress` remains pending; `interrupted` and `failed` are failure. `completed` proves the turn ended, not that the native assignment or all tool calls succeeded. Require the turn/result fields needed by the concrete contract, fail closed on absent/ambiguous data and never infer success from run status, final prose, or an empty output. Current read requests `turnLimit: 5`; newest/oldest order, paging, start response's lack of turn ID, and complete retained error/tool output need explicit resolution before implementation. A completed turn may contain a failed tool followed by model recovery; neither silently count it as successful native behavior nor treat every incidental failed tool as conclusive without a defined success contract. Native outcome is an observed task report requiring independent proof, unless the selected task defines an objective result schema.
4. The worker's `read` has a side effect on `waitingOnApproval`: it attempts archive-and-stop and returns `approval_required` with `cancellationConfirmed`. Report this as failure and retain identity; never assume cleanup succeeded. For a known child, perform bounded cancel on completion, failure, timeout, or workflow cancellation, using a bounded detached cleanup context where the Gimbal scope supports it. `cancel` counts as confirmed only when the worker returns `cancelled` after its own notLoaded/no-inProgress verification. Preserve `cancellation_unconfirmed`, lost cancel replies, and failed cleanup as owned unresolved outcomes; do not substitute successful workflow completion. Reconcile retry only with the known ID and same owner.
5. For a start whose identity is unknown, cancellation cannot safely target a fabricated task. Preserve ledger/request identity and report the operational reconciliation step. On daemon/process crash, durable Gimbal action records plus worker ledger must not cause a second creation; characterize how in-flight `RunCommand` re-entry handles an uncertain outcome before relying on it. A new bounded operator-triggered recovery is preferable to speculative background cleanup.

The observation interval and deadline must remain truly bounded across controller restarts/re-entry. `RunCommand` records and replays completed node results, so a poll loop cannot treat cached reads or a newly calculated deadline as new evidence/time. Persist one absolute deadline as durable input/result and give each sequential read a distinct recorded operation identity; verify Gimbal's interrupted-action replay contract in tests. If this cannot be achieved within the 150-line budget and ordinary workflow primitives, report a spec defect instead of making a hidden framework or transport layer.

## Verification

Focused package tests use an actual local fake worker client or socket with representative worker envelopes and recorded Desktop shapes, not fake native proof. Exercise: explicit scoped paths/required-app preflight; resolved alias; same ID same request and conflicting input; start lost response/unknown without second create; returned already-cancelled identity; active to completed and terminal failed/interrupted; missing/ambiguous/multiple turns; waiting approval and confirmed/unconfirmed automatic cancel; deadline and caller cancellation; cancel failure retaining ID; durable replay/re-entry without stale polls or duplicate creation. Assertions distinguish task-turn completion from task objective success. Add tests beside worker only if actual worker behavior changes. Run affected Go/Node tests, generation and catalog checks.

After source review, A must reserve an Amrita window before live validation. An independent validator must inspect the maintained caller and actual source revision plus real task/worker observations, including post-turn owner binding, task identity, model, reads, cancellation, lock and native evidence; prior feasibility is evidence for the old conversational route, not proof of new maintained code.

## Host/catalog and delivery

Current qualification host is a running Gimbal instance whose loaded catalog and Go plugin ABI are pinned to its existing engine. It cannot discover a newly built workflow or plugin without a host restart. Restarting that existing service is outside the released window and release hold. An isolated second Gimbal host with a matching candidate binary/plugin and separate instance/project state is a possible deployment, but is a new service/resource and is **not** automatically authorized by this note; coordinate with A and explicitly scope its lifecycle before use. It must connect to the already owned worker and not launch or replace an app-server. A controlled one-shot program using the maintained workflow and `gimbal.Run` is another possible invocation, but it is a standalone runtime, not the existing hosted controller, and requires a legitimate maintained entrypoint plus review of proof criteria; do not commit a proof-only launcher or assert equivalence to the hosted instance. Another route is an explicitly authorized maintenance window to replace/restart the existing host; preserve release hold and unrelated services. No route is selected merely to pass tests.

## Budget and unresolved decisions

The caller budget is 150 nonblank physical runtime lines, including all files in its cell; generated code accounting and supported exception, if any, must be clarified in the amendment. Worker budget is 500 nonblank runtime lines. This is an unmeasured feasibility estimate, not a demonstrated fit: config parsing, replay-safe polling, terminal-state parsing, bounded cleanup and unknown ownership together may exceed 150 lines. Do not split the caller into disguised helper layers or raise limits without an approved spec decision. Ordinary test and generated files are outside runtime accounting only if the amendment explicitly says so.

Decisions needed before coding: approve dedicated workflow/new names and ownership; choose input shape, timeout limits, target scope and effort; define task result semantics and exact read-turn identification/status/pagination rules from authoritative interface evidence; choose request snapshot/recovery and durable deadline behavior; settle generated-code accounting and deploy/validation host. These are material contracts, not reviewer-added features. If one is underivable, stop and surface it rather than inventing behavior.

## Sol adjudication of implementation choices

The pending human choice is the maintained entrypoint/new exported workflow name.
Once chosen, ordinary mechanics (input field spelling, validated finite observation
budget, explicit projectless target, resolved Luna effort, strict envelope parsing,
and generated metadata) can be settled by the implementation and independent
review within the two-cell contract. They do not each require a separate approval.
Task completion is execution completion; it is not objective acceptance of an
arbitrary assignment. Native behavioral acceptance remains independently observed.

An isolated candidate Gimbal host with private state has been proposed to A for
serialized validation. It is distinct from creating a Codex app-server or replacing
an existing Gimbal service. Source work can proceed after entrypoint choice; actual
native execution waits for A's runtime window. Generated catalog/graph/input
metadata is mechanical output, not hand-written caller logic; report its size
separately. The 150-line tripwire applies to handwritten runtime code and is not
permission to compress statements to disguise complexity. A real breach is surfaced.
