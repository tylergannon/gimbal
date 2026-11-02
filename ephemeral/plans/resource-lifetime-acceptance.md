# Resource-lifetime repair: task definition and acceptance

Status: qualification completed within the accepted scope; see PR #665 for
the observed proof and validation report, and independent validation round 4
under ephemeral/reviews/. Tyler authorized execution on the task owner's
recommendation after independent plan review. Coordination holds were released
without this task merging main, preparing a version, dispatching a release or
changing a stable tag. Startup exclusion and resource observations remain
separate claims; historical process-growth causation is unestablished.

## Goal and established scope

Finish Gimbal PR #665 by demonstrating the existing ownership repair through
real Gimbal workflows. Start from cd4ed425c0258d3ba21870e895b1c644f6e80a7a in
/Users/tyler/.codex/worktrees/run-inventory-tests/gimbal. The task worktree is
clean and the remote PR is open and draft at that revision as inspected on
2026-11-01. Existing test-double checks and independent code review establish
local coverage, not actual native resource stabilization.

Tyler accepted the completion boundary in this chat: demonstrate that repaired
Gimbal does not accumulate resources; explaining the historical roughly
30-process buildup becomes required if growth recurs during qualification.
This is proof-first work, with implementation limited to defects preventing
these claims. Repeatable missing live cases belong in package integration tests;
test-only changes may be needed even if the existing runtime repair passes.

This is an acceptance plan for an existing repair, not a replacement architecture
or a new public API. Existing contracts in docs/definition-of-done.md and the
public API remain authoritative. The earlier bounded observation plan is
ephemeral/plans/codex-resource-lifetime-proof.md; this document resolves its
remaining task boundary without treating earlier authorization restrictions as
permission to experiment.

## Ownership boundaries and required behavior

The workflow scope owns its sessions and services. Its normal final result is
preserved; cancellation and native uncertainty remain explicit ordinary results.
Native workers release local ownership and their private subprocesses are waited
and reaped. No background Close retry or cleanup journal is introduced.

The Codex adapter owns its local session routes, socket, reader and debug file
descriptor. Its final session and in-flight operation release those local
resources, including when archive/unsubscribe fails. Other admitted sessions
and operations remain usable. No new routing admission revives an ended scope.

The selected Codex daemon is borrowed from its identified owner. Workflow-owned
threads request archive; retained human threads detach and retain their native
identity. Ending a Gimbal scope must preserve the daemon and other clients.
An archive acknowledgement or worker exit alone does not prove that indirect
tool processes, native workers or server-side loaded threads have unloaded.

## Definition of done

1. Integration, workflow/executor/adapter/native daemon: real normal, cancelled
   and failed Gimbal work runs through the candidate. At least one successful
   turn performs an authorized harmless tool action whose child process can be
   identified. Observe the corresponding final result, owner exit and release
   behavior. A completed native result may win a cancellation race; exercise
   an actually acknowledged active turn for the cancellation observation.
2. Integration, same boundaries plus OS resources: repeated completed owner
   lifetimes leave no unexplained accumulating Gimbal-attributable processes,
   descendants, sessions, sockets, workers or handles/FDs. Count the indirect
   resources as well as Gimbal's direct children. Local routes/readers/debug
   descriptors need supported observation or focused package checks in addition
   to external resource observations; do not infer invisible internals from a
   process count alone. Resource behavior that cannot be attributed or observed
   leaves the relevant claim unmet.
   Include one two-session concurrent Group lifetime using an already-running
   daemon. Sequential-only observations cannot satisfy the full stabilization
   claim. A separate interprocess startup-lock claim below does not establish
   resource stabilization.
3. Integration, executor/adapter/native daemon: a retained human session resumes
   with the same native identity and can do another real turn after detach.
   A separate disposable client sharing the endpoint remains usable across
   another scope's release, without a daemon restart or client interruption.
4. Cell-local, adapter: existing focused regressions demonstrate failed release,
   concurrent admission and terminal transport handling without route revival
   or retained socket/reader/debug descriptor. Integration, executor/adapter/
   native daemon: a bounded real failure on a disposable owned connection or
   private service yields an honest final error/uncertainty, releases local
   ownership and does not blindly replay an accepted assignment. Test doubles
   supplement this claim; they cannot substitute for its real failure case.
5. Cell-local, executor: existing focused regressions demonstrate final error
   preservation, completion/cancel races and no retained Close-retry worker.
   Integration, workflow/executor/OS: observed private workers and identifiable
   tool children exit and are reaped after their owner lifetimes; no retained
   retry worker or cleanup journal is used to manufacture convergence.
6. Cell-local, adapter/startup: an OS-managed interprocess file lock serializes
   Gimbal workers targeting the same daemon across the recheck/start/readiness
   critical section. A waiting worker checks status again under the lock before
   deciding whether to start. Process exit releases the OS lock; a cancelled
   waiter can leave. Demonstrate exclusion and release with real subprocess
   package tests. This closes the specific concurrent-start vulnerability; it
   neither attributes the historical incident nor proves a process leak fixed.

Done requires all integration claims on the actual candidate and independent
validation of their evidence, plus applicable checks. State claims separately;
a green workflow flag or test suite does not satisfy unobserved behavior.

## Bounded observation plan

### Actual topology and claim-to-harness mapping

The persistent owners are a disposable central instance started from the
candidate (`gimbal --instance-dir <private-state> --port <unused-port>`), its
task/workflow execution capacity, and the selected native Codex daemon. Each
native turn runs in a separate `--gimbal-agent-worker` process. That worker
constructs `codex.New()`, owns its connection and adapter internals, and exits
after release; the launching Gimbal process reaps it with `cmd.Wait`. Observe
worker exit/reap and daemon-side client/socket/FD and descendant changes for
every lifetime while the central and native daemon stay alive. `run-prompt`
alone is insufficient because its instance exits after its turn.

For claims 1, 2 and 5, submit the existing `review` workflow through
`gimbal run review --instance-dir <private-state> --project <disposable-project>
--goal <bounded-read-only-task> --code-review luna --follow`. Supply a harmless
fixture file for an observable read tool action. The goal asks for one specific
harmless shell command that reads this fixture and carries a unique per-turn
marker in its arguments. If no identifiable child is observed, that sub-claim
is not observed; make at most three attempts before reporting the gap. For the cancelled case, use
`gimbal cancel <run-id> --work-dir <disposable-project>` after the worker has
acknowledged an active native turn. Observe accepted cancellation separately
from the owner's final result. A finite read-only fixture workload supplies an
active turn; do not modify the review workflow to prolong it artificially.
A completed result winning the race counts as a normal lifetime. Re-attempt
with a larger read-only fixture, at most three attempts per required cancellation
observation, then report the cancellation observation as unobtained.

The two-session Group and terminal real-failure
cases are repeatable integration tests in their owning package, using ordinary
Gimbal Run/Group and the actual native executor/daemon, with no fake provider.
They may be added after approval. The Group case covers real concurrent worker
admission; a separate adapter-level live case exercises two sessions on one
connection and releases one while the other remains usable. Neither substitutes
for the other. No standalone proof workflow/program is added.

For claim 3, use production conversation detach/resume for end-to-end identity
preservation and the existing adapter-level
`TestDetachedConversationResumesTheSameNativeThread` as supporting coverage.
`TestCloseArchivesThreadsWithoutTouchingTheDaemon` supplies the existing loaded
thread query and unchanged-daemon observation technique. Both must select the
unversioned luna alias before running; their current hard-coded version is not
the approved experiment model. Use the selected daemon's positively identified
PID/socket, not the existing helper's machine-wide pgrep, when multiple daemons
exist. Internal routes/readers are assessed directly by live package assertions
and focused regressions; external counts alone never establish their release.
The opt-in debug descriptor is covered by the focused package test; live raw
protocol recording is unnecessary and remains disabled.

For claim 4, use the test-owned adapter connection to the existing daemon and
the live test's
`ad.current().ws.CloseNow()` technique without touching the daemon listener or
another client. The package test's adapter.dial hook rejects only this adapter's
redial after connection loss, inducing a local connection failure on the real
session without substituting a provider. Extend the owning live test to exercise terminal release failure,
not only successful reconnect. The existing
`TestActiveTurnReconnectPreservesToolEffectLive` supports the exactly-once
tool-effect/reconciliation observation but its recovered success alone does not
satisfy the terminal failure claim. Report what each test establishes: forced
redial rejection demonstrates failure/release; successful real redial with an
exactly-once tool effect demonstrates recovery without blind replay. Neither
observation alone establishes the other behavior. If a required observation
cannot safely be obtained, leave the relevant claim unmet.

Tyler explicitly replaced the authenticated private cold-start experiment with
the startup-lock implementation and subprocess proof in claim 6. No additional
login or credential copying is needed. The accepted requirement is exclusion of
Gimbal's competing starts, not proof that the historical startup race occurred.

### Observation sequence

After approval, inspect service/process ownership read-only before choosing any
endpoint or starting a candidate. Identify candidate revision, executable,
native CLI/Desktop versions, GUI-login identity if relevant, daemon owner,
effective home, project/catalog and process ancestry. Do not expose credentials
or inspect unrelated transcripts. Preserve the concurrent Locked Use work and
its services/worktrees. Coordinate a concrete shared-host conflict before an
operation that affects its owner. Prefer disposable owned state and existing
supported workflow paths; no proof-only program is committed or placed under
ephemeral/. A repeatable missing check belongs in its owning package.

Inventory process identities/ancestry, worker/session identities, loaded owned
threads where observable, open sockets, FDs/handles and identifiable tool
descendants before work and after every owner lifetime. Observe persistent
owners while they remain alive; exiting the entire test process is not evidence
that its per-scope resources were released. Explain each intentional baseline
resource by its owner, purpose and lifetime. A stable unrelated Desktop resource
is not a Gimbal leak; a vendor-created resource caused by our work is in scope.

Use up to two warmup lifetimes followed by three sequential repetitions of each
normal/cancel/error case, sampling the same persistent owners at each boundary.
Add the bounded two-session Group described above. Also exercise human-session
resumption and a second disposable shared client.
Use unversioned luna for Codex observation turns and report the resolved model.
Keep concurrency at two sessions/workers for these observations.

Record immediate and settled observations. A two-minute observation window per
completed lifetime is an experiment limit, not a product cleanup deadline.
If resources remain unexplained at that limit, stop that case and report an
unmet claim; do not call the elapsed window a timeout defect or repeat unchanged
failures. Warmup is not permission to dismiss increasing retention: the baseline
must be explained and remain stable through the measured repetitions.
Detached retained human sessions and between-turn workflow threads may remain
loaded during the native inactivity grace period; their identified native
identities are explained baseline while retained. Assert archived workflow
thread unload at scope end, not immediately after between-turn unsubscribe.
Legitimate grace retention must still be identified and bounded by its owner;
it cannot excuse unexplained increasing loaded-thread counts.

Inject disconnect/malformed/startup failures only through an identified
disposable owner or connection with no other clients. Never damage a shared
daemon to simulate failure. Choose the smallest real failure that exercises
the changed release path. If no safe real failure is possible, identify that
proof blocker rather than silently replacing it with a double.

If growth recurs, attribute the retained resources, diagnose and repair the cause
within Gimbal ownership, then repeat the affected observations on the new head.
Do not blindly add capacity limits or a new accounting system. If resolving the
defect requires a new public contract or conflicts with exclusions, return to
Tyler with the concrete decision. If the historical failure cannot be connected
to the reproduced growth, explicitly limit the causal claim.

## Execution, evidence and closeout

Independent plan review and consensus precede Tyler's written-plan approval.
After approval, Terra 6.1 implements only if needed; Sonnet independently
validates. Use supported harnesses, disclose model availability limitations,
and keep agents in this task worktree. Planning reviewers remain read-only
except their review artifact. Runtime ownership must also include any resources
created by the implementation/validation harness itself where attributable.

Report observed proof in this chat and the PR body, naming source SHA, exact
cases and repetitions, resource baselines/deltas and owner explanations,
outcomes, native identity preservation, failure/uncertainty and material gaps.
Follow repository proof rules: commit no run logs, screenshots, text dumps,
result.md, proof programs or other runtime output. Plans, reviews and actionable
worklog notes are tracked notes. Existing code checks are reported as historical
until verified applicable to the tested head; rerun affected checks for changes
or unresolved concerns, not merely because a commit occurred.

Leave the PR draft if a required claim is unmet. When claims pass, update the
PR with concrete evidence and provide Tyler the merge-ready result; merging,
release publication and native-control qualification are separate decisions.
Send an authorized concise handback to parent chat
01a1981c-d915-7300-a4eb-0da2f8955827 at approval handoff, real blocker or final
handback, including this plan path, approval state, proved head/claims and gaps.

## Exclusions and escalation

No global process cap, aggregate capacity policy, cancellation-completeness
ledger, cleanup receipts, retained retry-cleanup lifecycle, strict orchestrator
timing guarantee, unrelated Desktop cleanup, 30-replica stress test, native
control integration or release-pipeline work. No new exported names or API
concepts. Do not replace/relaunch another investigator's app or services.

Deferred decisions are concrete safety/ownership conflicts and any repair that
would change a public contract or violate these exclusions. Report such a
conflict before the dependent action. Ordinary instrumentation and test-case
mechanics are implementation choices, not additional interview gates.
