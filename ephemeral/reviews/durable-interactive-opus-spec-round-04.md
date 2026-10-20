# Durable Interactive: Opus spec review, round 04

## Target

- nlspec `specs/durable-interactive.md` (611 lines) and delivery plan
  `ephemeral/research/durable-interactive/DELIVERY-PLAN.md` (149 lines) at
  `3270c21e` ("docs: remove version policy and settle suspension authoring
  rules"); only the worklog was modified in the working tree.
- References: `ephemeral/research/durable-interactive/SOURCE-MAP.md`,
  `/Users/tyler/src/gimbal-token-cache/index/README.md`.
- Caller narrowing: none ignored. The request sets the target, the references
  and the output path; "do not edit any other files" is an operating limit,
  and this file is the only one written.

## Evidence inspected

- Repository instructions: `AGENTS.md`, `skills/gimbal/SKILL.md`,
  `docs/definition-of-done.md`; prior reviews rounds 01-03 and the worklog's
  recorded human decisions.
- Public contract and runtime: `run.go`, `scope.go`, `group.go`,
  `session.go`, `run_recording.go`, `events.go`, `doc.go`, `service.go`,
  `service_process_unix.go`, `loop.go`, `remote_execution.go`,
  `supervise*.go`, `registry.go`.
- Native execution: `internal/binding/binding.go`,
  `internal/agentexecutor/agent.go`, `internal/executor/files.go`,
  `internal/effects/identity.go`, `{claude,codex,pi,agy}/executor.go`,
  `codex/codex.go`, `codex/recovery.go`, `internal/conversation/{manager,requests}.go`.
- Hosting: `internal/host/routes.go`, `internal/runner/{launch,runner}.go`,
  `internal/invocation/invocation.go`.
- Authoring: `internal/gimballint/{analyzer.go,rules.md}`, maintained
  workflows under `internal/workflows/` (implementation, researcheval,
  researchdocument, validateproduct, sprintplan, pyramidsummary),
  `cmd/examples/notebook/main.go`.
- Presentation: `web/src/lib/workspace/ProjectWorkspace.svelte`,
  `web/src/lib/workspace/conversations.ts`.
- Line counts measured with `wc -l` over production (non-test, non-generated)
  Go for every directory/file named below.

## Round-03 disposition

| Round-03 finding | Status at `3270c21e` |
| --- | --- |
| F1 version/code identity | Resolved by human decision: no version policy (spec 429-437, plan 75-80). Not re-raised. |
| F2 native restart continuity | Text resolved, but on an incomplete premise that round 03 supplied: "Claude rejects IDs it does not hold" described the conversation-manager path only. The workflow path continues through `internal/agentexecutor`. See F1 below. |
| F3 dormancy vs. Go execution | Addressed: unwinding, suspension-aware scopes/Group, defer/recover rejection (spec 89-92, 415-421). The public `Run` consequence remains; see F3. |
| F4 authored timing/cancellation | Addressed by rejection (spec 94-100). Opaque-call admission and godoc consequences remain; see F4. |
| F5 ownership/budget | Mostly addressed (spec 148-170). The executor stack and several root files remain unowned; folded into F1. |

## Pass summary

| Pass | Result |
| --- | --- |
| Consistency | Defects: spec 395-403 vs. the actual workflow binding path (F1); spec 586-587 / plan 99-102 vs. shared-host restart (F2); spec 415-425 vs. `Run` godoc (F3); spec 88-100 vs. public godoc (F4). |
| Claim traceability | 12 cell-local and 4 integration claims route to cells; C4/L1 depend on "actionable unresolved recovery", which has no defined actions (F5). |
| Seam audit | Defects: the native continuity seam names the conversation-only `ResumeSession` instead of the executor checkpoint/reattach that workflow sessions use (F1); no cancellation seam on `ExecutionHost` (F5). |
| Claim classification | Correct cell-local vs. integration split; L1's "Codex only" scope follows from F1's wrong premise. |
| Derivability | Not derivable: public `Run` result on dormancy and reconstruction entry (F3); opaque-call admission rule (F4); unresolved-run exit (F5). |
| Pump hygiene | Satisfied. The round-03 decisions (no version policy, defer/recover and timing rejection, no native resource policy) are recorded in the worklog and reflected in both documents; no silent amendment found. |

## Findings

### F1 — critical — Native continuity names the wrong mechanism and leaves its implementation unowned

Pass: seam audit, consistency.

Spec 395-403: "Rebinding a native session uses an internal optional capability
with the existing ResumeSession method shape ... Codex already supplies it and
is the required restart-continuity harness ... If another harness lacks that
capability, preserve its saved identity/input and show actionable unresolved
recovery after restart ... This matches the existing non-Codex recovery
limit." H ownership adds `codex/codex.go` and `codex/recovery.go` (spec 155-156);
plan 132-133 repeats the Codex `luna`/`ResumeSession` route.

Current source contradicts that premise for workflow sessions:

- Workflow sessions bind through `internal/binding/binding.go:48-56`:
  `agy`, `claude`, `codex` and `pi` go to `agentexecutor.New(harness)`; only
  `opencode` uses a direct adapter.
- `internal/agentexecutor` already provides restart continuity independent of
  `ResumeSession`. Each turn runs in a Setsid worker process
  (`internal/executor/files.go:229`). The session config persists in
  `sessions/<id>/session.json`, and `Create` is idempotent and rejects
  conflicting inputs. The native checkpoint `native.json` is restored through
  `LoadSession` before a turn and written with `SaveSession`/`DetachSession`
  after it. A durable `result.json` is written per turn ID. `Submit` reattaches
  to an accepted turn when `executor.Match` succeeds.
- `SaveSession`/`LoadSession`/`DetachSession` are implemented by
  `claude/executor.go`, `codex/executor.go`, `pi/executor.go` and
  `agy/executor.go`. Claude's `LoadSession` restores its session map from the
  checkpoint.
- Executor IDs derive from recorded node identity:
  `run_recording.go:85-93` (`ExecutorOperation: "turn_"+action`,
  `ExecutorSession: "session_"+session`) and
  `internal/agentexecutor/agent.go:520`
  (`executor.TurnID(identity.ExecutorOperation)`). Replay with the same node
  identity therefore reattaches to completed or in-flight turns.
- `codex.ResumeSession` (`codex/codex.go:377`) is called directly only by the
  conversation manager (`internal/conversation/manager.go:126-131`). The
  "existing non-Codex recovery limit" is that path's behavior
  (`internal/conversation/requests.go:178-181` marks a non-resumable request
  failed), and this delivery deletes that path.

Impact:

- The spec mandates "unresolved after restart" for Claude, Pi and Agy, which
  the workflow path already continues. An implementer following it either
  regresses working continuity or ignores the spec.
- The spec puts the conversation-only Codex files (1,199 lines) in H, while
  the code that actually carries continuity is outside every ownership set:
  `internal/agentexecutor` (820 counted lines), `internal/executor` (493),
  `internal/commandexecutor` (313) and `internal/effects` (640).
- The root files `loop.go` (317), `service.go` (327), `remote_execution.go`
  (188), `supervise.go` plus `supervise_jev.go` (366) and `doc.go` (41) —
  1,239 lines in all — are also unowned, even though suspension must pass
  through `PromiseLoop`, services and supervision. Counting them in E would
  leave about 379 lines of headroom under its 5,500 maximum.
- Spec 166-170 makes this a stop-before-implementation blocker.
- Only `opencode` truly lacks restart continuity.

Proposed amendment (HITL):

1. Rewrite spec 395-403 to name the existing executor checkpoint/reattach
   (deterministic turn/session IDs, `native.json`, per-turn `result.json`) as
   the continuity mechanism. The continuity harnesses are the ones the
   executor serves (claude, codex, pi, agy); opencode is the
   unresolved-after-restart case.
2. Assign `internal/{agentexecutor,executor,commandexecutor,effects}` and the
   unowned root files to cells, with measured baselines and revised maximums.
   Drop the Codex conversation files from H unless they still change.
3. Fix plan 132-133 to match. If the human wants it, allow L1 live continuity
   on `haiku` as well as `luna`.

### F2 — critical — The shared host changes restart, plugin and panic behavior for running workflows without saying so

Pass: consistency, seam audit.

Today hosted workflows are independent processes:

- `internal/runner/launch.go` says `Spawn` "starts an independent workflow;
  the admission caller never owns its lifetime", and launches
  `run-process` with Setsid (`internal/host/routes.go:104-136`).
- Each process loads plugins for its own request
  (`internal/runner/runner.go:30-33`) and holds a per-run flock and socket
  controller (`runner.go:103-145`).
- A host restart therefore does not interrupt running workflows.
- Services are `exec.Command("zsh", "-c", ...)` children with Setpgid
  (`service.go:99`, `service_process_unix.go`), owned in memory by the
  hosting process.

The spec and plan replace this with a shared cross-project host (plan 37-39).
That sharing decision is settled and not re-litigated here. Its consequences
are not specified:

- **Restart interrupts every project's active runs.** L1 itself requires a
  host restart, and so does any workflow edit: compiled-in workflows need a
  rebuilt binary, and Go plugins cannot be reloaded (`registry.go:130-169`,
  `plugin.Open`). Each such restart interrupts every active run of every
  project.
- **Services die or are orphaned with the host.** Restoration is explicitly
  deferred (spec 584-590). Yet the spec says "Existing service behavior and
  claims are left alone" (spec 586-587) and the plan says "Preserve
  services/claims on suspension" (plan 101-102). Neither says what a
  reconstructed run sees when the service it depends on no longer exists. A
  silent replay past a dead service is the likely default.
- **One panic takes down every project.** `Run` deliberately re-panics
  escaped panics, leaving "a complete run.jsonl and a dead process"
  (`run.go:151-158`, `group.go:70-85`). In separate processes that kills one
  run (`runner.go:147-153`). In a shared host, one project's authored nil
  dereference kills every project's runs.

Impact: the spec's restart claims are untestable without first deciding these
behaviors. Implementers will invent them inside H, with no budget or claim.

Proposed amendment (HITL), keeping the shared host:

1. Define what an active run holding a Service sees after a host restart. The
   proposed answer is unresolved recovery, never silent replay past a missing
   service.
2. State that code/plugin updates require a host restart that interrupts all
   active runs, or that it does not.
3. Decide whether the process-fatal panic policy in `Run`/`Group` holds in
   the shared host, and add the claim that tests the decision.

### F3 — critical — The public `Run` contract is not reconciled with dormancy and reconstruction

Pass: consistency, derivability.

`Run(ctx, runID, name, models, body) error` (`run.go:118-160`) is the hosted
and direct entry (`internal/runner/runner.go:153`). Its godoc says:

- it "blocks until the body returns" (`run.go:121`);
- "The run's ctx derives from the caller's, so main can put a deadline on it"
  (`run.go:126`);
- on return "each session it created is actually released through its
  adapter's Close, then its ctx is cancelled" (`run.go:127-129`);
- a direct `Run` executes without joining a running web instance or its live
  controls.

`beginRun` always creates a new record
(`run.go:194`, `runstate.Resolve(ctx, project).Create`). Plan 40 says "Run
creates or reconstructs the same recorded run". The spec (415-425) defines
dormancy as unwinding the body without completion. It never says:

- what `Run` returns when the body goes dormant (a sentinel error would be an
  unapproved exported name);
- whether `Run` with an existing run ID reconstructs, or which entry does;
- what a direct `Run` without host ingress does on `Interactive` or a waiting
  Group branch;
- what a caller deadline means across dormancy.

Concrete conflict: `agentexecutor.RunTurn` copies `ctx.Deadline()` into
`Request.Deadline` (`internal/agentexecutor/agent.go:521-523`), and
`executor.Match` byte-compares the persisted request JSON
(`internal/executor/files.go:118-131`). A reconstructed activation under a
different caller deadline gets "operation identity conflicts with accepted
inputs" when it reattaches an in-flight turn, instead of continuing it.
`cmd/examples/notebook/main.go:53` wraps `Run` in an 8-minute
`context.WithTimeout`, so this is reachable from maintained code.

Impact: there is no derivable public behavior for the entry point every
claim exercises, and a predictable reattach failure on reconstruction.

Proposed amendment (HITL):

1. Specify `Run`'s return and session-release semantics on dormancy, and the
   reconstruction entry. Any new exported name needs Tyler's approval by name.
2. Specify direct (non-hosted) `Run` behavior at a durable wait.
3. State the caller-deadline rule across dormancy. Either the deadline is not
   part of the accepted turn identity, or it is recorded once and reused.
4. Update the `Run` godoc accordingly.

### F4 — issue — The admission rule for opaque calls is undecidable, and public godoc teaches newly forbidden constructs

Pass: derivability, consistency.

Spec 88 says "Opaque helpers cannot hide prohibited effects", and spec 482-485
says a source-visible helper "is not an escape hatch", with no Go sandbox
required. Neither says whether calls into the standard library, third-party
packages or other packages are allowlisted or denylisted. The current lint
follows only package-local source-visible helpers
(`internal/gimballint/rules.md`, GIMBAL101). The maintained workflows A must
migrate call opaque, effectful code:

- `filepath.Abs`, which reads a process cwd shared across projects in a
  shared host: `internal/workflows/sprintplan/files.go:30` and
  `pyramidsummary.go:537`, `implementation.go:165`,
  `researchdocument.go:703,745`;
- direct `os.*` calls in researcheval, validateproduct and researchdocument;
- researcheval's `waitResearch` (`researcheval.go:690-722`), which reads
  in-process `observation.Registry.Snapshot` and
  `live.Runs.InProgress(id).CancelScope` with a ticker and `context.Cause`;
- `researcheval.go:337`:
  `context.WithTimeout(context.WithoutCancel(caseCtx), time.Minute)`;
- `implementation.go:152`, which calls `ctx.Err()`;
- map iteration in researcheval's cost aggregation.

The spec also leaves undecided whether `slices.Sorted(maps.Keys(m))` and
`json.Marshal(map)` are deterministic enough to admit.

Public godoc now contradicts the authoring rules:

- `Session.Steer`: "It is called from another goroutine while Generate
  blocks" (`session.go:760`), but raw goroutines are rejected.
- `Killed`: "workflow code that cares checks errors.As(context.Cause(ctx),
  &killed)" (`events.go:235`), a direct cancellation observation.
- `Group`: "To stop children early, cancel their parent context before
  waiting" (`group.go:32-33`). Is `context.WithCancel` admitted?
- `doc.go:37-39`: lifetime prose built on context cancellation.

Impact: A cannot write the analyzer or migrate the built-ins without guessing
the rule, and the API would contradict its own lint.

Proposed amendment (HITL):

1. State the opaque-call rule (allowlist of admitted packages/functions, or
   an explicit denylist) and how deterministic map-key ordering is treated.
2. List the godoc contracts that change (`Steer`, `Killed`, `Group`,
   `doc.go`) and assign the edits to E.

### F5 — issue — Unresolved and dormant runs have no defined exit

Pass: claim traceability, seam audit.

- "Actionable unresolved recovery" (spec 399-400, claim C4) does not say what
  the actions are.
- Archive under uncertainty keeps the run unresolved "while reconciling"
  (spec 50-53). Reconciliation may never complete, for example opencode after
  a restart or an unconfirmable native outcome.
- `ExecutionHost` (spec 442-447) has admit, submit, schedule and observe, but
  no cancel.
- "Explicit cancellation retain[s] their normal lifetime behavior" (spec
  424-425) does not say how to cancel a dormant run. It could reactivate the
  run by replay and then cancel it, or record a terminal state directly,
  which skips cleanup for sessions and services.
- Today tab close only hides the tab
  (`web/src/lib/workspace/ProjectWorkspace.svelte:180-186`), and
  `conversations.ts` maps unresolved to "needs-you" with no way out. The
  conversation path being deleted at least ends a non-resumable request as
  failed.

This is what remains of round-03 F1 after the version policy was removed.

Impact: runs can stay unresolved or dormant forever, with no human action and
no terminal outcome. C4 and L4 ("unconfirmed cessation is shown honestly")
have no completion criterion.

Proposed amendment (HITL):

1. Define the human actions on an unresolved run (for example, retry
   reconciliation, or abandon it to a recorded terminal outcome) and what each
   records.
2. Define cancellation of a dormant run: the seam on `ExecutionHost`, whether
   it reactivates, and how it cleans up sessions. Service cleanup stays
   within the SERVICE exclusion.

## Delivery plan notes

The plan inherits F1-F5:

- plan 132-133 carries F1's wrong mechanism (`luna`/`ResumeSession`);
- plan 37-39 removes the per-invocation subprocess without F2's
  consequences;
- work item 3 (plan 98-102) promises services are preserved, which the
  shared host restart contradicts (F2);
- plan 40 asserts "Run creates or reconstructs" with no spec contract (F3);
- work item 5 cannot be built without F4's rule.

Otherwise, the two-owner split, the single final validation pass and the
blocker-reporting rule (plan 145-149) are sound. No other plan defects were
found.

Already resolved and deliberately not raised:

- complete `RunCommand` output storage (payloads of 512 KiB or more go to
  blobs, `run_recording.go`);
- the 64 KiB display limit is in `command.go:22`, which E owns.

## Outcome

material findings remain
