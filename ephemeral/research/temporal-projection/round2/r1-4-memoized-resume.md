# R1.4: Can Gimbal resume a crashed orchestrator by re-executing the body and short-circuiting completed nodes from run.jsonl?

Date: 2026-09-27. Scope: risk R1.4 from SYNTHESIS.md, under Finalist 1
(runtime projection) with D (Gimbal-native memoized resume) as its
durability add-on. Gimbal claims below are read directly from the checkout
this session and cited file:line; nothing is carried over unverified from
round 1 except where marked. Web claims are fetched or searched this
session (2026-09-27) and marked verified/secondary as round 1's convention
requires.

## Verdict

**Confirmed for the stateless half, not eliminated for the stateful half,
and unbuilt in either case.** Gimbal already has the two hardest
prerequisites every durable-execution engine needs — a stable, deterministic
node-identity scheme assigned at the call site (`scope.go:104-110`,
`group.go:63-68`) and a durable, append-free, single-writer, gap-free-`Seq`
log of every node's start and end (`event_persistence.go:46-53`,
`events.go:265-277`) — for free, as a byproduct of the observability design,
not because anyone built resume. Re-executing a workflow body and
substituting each node's recorded `TurnEnded`/`CommandEnded`/`PlannerDecision`
result for a live call is mechanically sound for `Generate`, `RunCommand`,
`Check`, `PromiseLoop` task selection, and `Interview` — the same shape DBOS,
Restate, and `agenticenv/durable-go` already use in production, and Gimbal's
registered `workflow.Graph` gives it a diff-and-refuse versioning check none
of them has automatically. This is real; it does not require Temporal's
workflow-side replay discipline (`workflow.Context`, banned goroutines,
`workflow.GetVersion`) because Gimbal's ordinary-Go orchestrator is not
itself the thing being replayed — only its *effects* are being
short-circuited on re-execution.

It stops being clean at the two places every surveyed engine also struggles:
**sessions** (the harness process/conversation a `Generate` call depends on
is not durable state anyone snapshots; resuming means either reconnecting to
a live native id or accepting no continuation exists) and **the in-flight
node at crash time** (Gimbal today executes `RunTurn`/`RunCommand` in the
same process as the orchestrator via direct `os/exec`/adapter calls — there
is no separate worker to query, so "the activity may still be running
remotely" is a Finalist-1 future, not a fact about today's code). And **none
of it exists**: `grep -rn "resume\|restart\|crash"` across the module finds
exactly one relevant sentence, in the existing (unrelated) replay code for
the *observation* store: "Nothing is followed and no agent process is
resumed" (`internal/observation/replay.go:23`). `run.turns` and
`run.interviews`, the only in-memory tables that track "what is currently
open," are plain maps with no persistence (`run.go:67-83,291-301,311-327`).

**Section 6's comparison favours running the orchestrator as a
restart-on-crash pod first**, exactly as SYNTHESIS.md's Direction C already
argued for a different reason. Memoized resume is real, cheap-to-reason-
about future work on top of facts Gimbal already has; it is not urgent
enough to build before Finalist 1's activity layer exists, because the
payoff (not re-running already-billed agent turns after an orchestrator
crash) is proportional to how often the orchestrator crashes independently
of a run's own workers, which nothing in the current single-process
architecture measures yet.

## 1. Which records are sufficient to reconstruct each node type's result on replay

| Node type | Recorded in | Fields present | Sufficient alone? |
|---|---|---|---|
| `Generate` (a turn) | `TurnEnded` in `run.jsonl` (`events.go:148-154`), written at `session.go:264-271` (`TurnStarted`) and `session.go:353-390`-ish (`TurnEnded`, confirmed via `session.go:374-390` terminal path) | `Result JSONText`, `Error string`, `Usage []ModelUsage`, `Duration`, `Interrupted bool` | Yes, for the typed result and cost. The per-native-event transcript (`sessions/<id>.jsonl`, `AgentRecord`, `event_persistence.go:154-171`) is extra fidelity for the page, not needed to short-circuit the call: `generate[T]` only needs the validated JSON back (`session.go:182-223`). |
| `RunCommand`/`Check` | `CommandStarted`+`CommandEnded` pair in `run.jsonl` (`events.go:180-209`), written at `command.go:211-236` | `ExitCode`, bounded `Stdout`/`Stderr` excerpts, `StdoutFile`/`StderrFile` (run-relative paths to the full streams), `Error`, `Duration`, `Interrupted` | Yes for the exit code and error the workflow branches on. The *full* stdout/stderr live only in `commands/<id>/{stdout,stderr}.log` under the run directory (`command.go:85-92`), not in `run.jsonl` itself — resume needs the run directory on the same filesystem, not just the log line. `Check` additionally records a `ValueSet` for the scope value it derives (`command.go:200-204`, `valueEvent`), which is separately replayable. |
| `Service` | Same `CommandStarted`/`CommandEnded` pair, via `zsh -c` (`service.go:50,56`) | Same fields as above | Only for a service that has already stopped by crash time. A service still running when the orchestrator dies has a `CommandStarted` with no matching `CommandEnded` — see §4; there is no separate "service still healthy" record. |
| `Session`/`Fork` (session existence, not a call) | `SessionCreated` in `run.jsonl` (`events.go:105-114`), written at `session.go:58` (`NewSession`) and `session.go:631` (`Fork`) | `Name`, `Adapter` (a `%T` type name, not an instance), `Model`, `Effort`, `Workdir`, `Parent` | Sufficient to know a session with this scope-assigned id *existed* and what it was bound to. **Not** sufficient to resume calling turns on it — see §3; the native harness id is not in this record at all. |
| `Interview` | `InterviewQuestionAsked`+`InterviewQuestionAnswered` (`events.go:160-175`) | `QuestionID`, `Question`, `Answer` | Yes — a fully durable request/response pair; resume simply substitutes the recorded answer for a blocked wait (`run.go:329-345`). |
| `PromiseLoop` planner decisions | `PlannerDecision` (`events.go:73-78`), written at `loop.go:154,161` | The full selected `Task` (`Name`, `Description`, `DefinitionOfDone`, `Validation`, per `loop.go:16-30`), or none for "planner ended dispatch" | Yes — the entire task the planner chose is recorded, not just an index or a summary. Resume can replay the exact task sequence without re-invoking the planner session at all. `backlog.md` under `scopes/<loop key>/` (`loop.go:110-114,149-151`) is a redundant, human-readable copy of the same data, not a separate source of truth. |
| `Fork` (as a node) | Folded into `SessionCreated{Parent: s.id}` (`session.go:631`) — no separate event type | Same as `SessionCreated` above | Same caveat as sessions: existence recorded, native reattachment is not. |
| `Set`/`SetJSON` | `ValueSet` (`events.go:96-100`), written via `artifact.go:249,251` | Either the value inline (`JSONText`) or a pointer to a spilled `ValueArtifact` file | Recorded for observation, but **not required** for replay in the way the others are: `Set`/`SetJSON` are not themselves activities: they are ordinary Go statements the re-executing body runs again, and they reproduce the same value as long as everything feeding them (upstream `Generate` results, params, and — see §2 — anything else the body reads) is itself already deterministic on replay. The recorded `ValueSet` is a display artifact, not a memoization key. |
| `ScopeBegan`/`ScopeEnded` | `events.go:56-69`, written at `scope.go:161,171` (via `finish`) | `Name`, `Loop bool`, optional `Task`; `ScopeEnded.Error` | This is the closest thing Gimbal has to "the graph, as it actually ran": every scope's assigned id (`ScopeBegan`'s `LifecycleRecord.Scope`, e.g. `lap.3`) is durably logged in call order, independent of the static `workflow.Graph`. A resume path could, in principle, reconstruct "which node ids exist and in what order" purely from `run.jsonl`, without consulting the registered graph at all — see §5. |

Two structural notes not visible in the table: first, `internal/observation/replay.go:20-79` (specifically `open` at line 24 and the comment at line 23) already reconstructs the *display* tables (`scopes.json`, `turns.json`, etc.) from exactly these same two JSONL logs when they are missing — proof that the log is self-sufficient for *reading back* every node's result, and simultaneously proof that nobody has extended that same reconstruction to *drive a live re-execution*: the doc comment is explicit that "no agent process is resumed." Second, every `Ended`/`Answered`/`Decision` record above is written only after its corresponding `Started`/`Asked` record and only by the same in-process call path (`command.go:212,235`; `session.go:271` before `321`; `loop.go:154/161` after the planner call returns) — so an in-flight node at crash time is visible in the log as a `Started` record with no matching `Ended` record, which is the natural signal a resume path would key on (§4).

## 2. Determinism preconditions: every way re-execution could diverge, checked against the actual workflows

The audit's finding (gimbal-internals-audit.md §1.2, confirmed independently
here at `scope.go:104-110` and `group.go:63-68`) is that ordinals are
assigned synchronously, in source order, at the call site — so node identity
is deterministic **given (a) the same compiled source and (b) the same
sequence of activity results feeding any branch, loop bound, or item list**.
Precondition (b) is the entire risk surface. Below is every way it can
break, checked against `internal/workflows/{implementation,pyramidsummary,
validateproduct,researchdocument,implementinterview}` (five workflows;
`review` and `interview` were also read and contain no `Group`, `Iterate`,
or `PromiseLoop` at all, so they carry no risk of this kind).

| Hazard | Mechanism | Present today in stock workflows? |
|---|---|---|
| A `Group` child named from data, not a literal | `Group.Go(name, fn)` computes `child := g.scope.child(name)` synchronously before the goroutine starts (`group.go:67-68`), so if `name` itself were computed from a loop variable, ordinal assignment would depend on how many distinct names had been seen before, and hence on upstream data | **Not present.** Every `Group.Go` call site found (`pyramidsummary.go:165,180,195,210,225,240,321,335,349,363,377,391`; `validateproduct.go:171,188,208`; `researchdocument.go:198,217,236,255,274`; `implementinterview.go:64,69`) passes a literal string constant (`"summary1"`..`"summary6"`, `"tester1"`..`"tester3"`, `"agent1"`..`"agent5"`, `"backend"`/`"frontend"`). None is built from a loop over a slice. |
| A `Group` child that races another for the same name | Two goroutines calling `g.Go` with the same literal name concurrently would race on `s.ordinals[name]++` under `s.mu` (`scope.go:104-110`, mutex-protected) — this is safe for the counter itself, but the *pairing* of "which real-world unit of work got ordinal 2" would depend on scheduling if the same name were reused in a loop across iterations that ran concurrently | **Not present** for the same reason as above — no literal name is reused inside a concurrent context; each is called exactly once in the function body. |
| `Iterate` over items derived from a nondeterministic body computation | `Iterate` (`iterate.go:12-40`) builds its child scopes sequentially over whatever slice it is given — deterministic *given* the slice, but the slice's origin is the workflow's own problem | **Present, and real.** `internal/workflows/implementation/implementation.go:71-77`: `outcomes` is decoded from `os.ReadFile(outcomesFile)` (`implementation.go:71`), a raw filesystem read at workflow-body scope, entirely outside any Gimbal primitive and **not recorded by any lifecycle event**. `gimbal.Iterate(ctx, "outcome", outcomes)` (`implementation.go:93`) then drives one child scope per outcome. If the orchestrator restarts and the outcomes file changed (edited, moved, or simply absent on a different host/pod) between crash and resume, re-execution produces a different `outcome.N` sequence than the one recorded — exactly the audit's general finding in gimbal-internals-audit.md §4/fact 11, now pinned to one concrete, present-day instance. |
| `PromiseLoop` items derived from a Generate result | `plan.Groups`/similar structured planner or researcher output feeding a downstream loop bound | **Present but safe**, because the upstream value is itself a `Generate` result: `researchdocument.go:164` (`planner.Generate[ResearchPlan](ctx, planTopicsPrompt)`) is recorded as a `TurnEnded.Result` (`events.go:148-154`); `groupDirs` and the fixed `research.Go("agent1"…"agent5")` calls (`researchdocument.go:172-193,197-286`) derive from `plan.Groups`, whose length is asserted `== 5` (`researchdocument.go:168-169`) rather than driving the number of `Group` children — the five names are literal regardless of plan content. So this hazard resolves to "safe" here specifically because Gimbal's own model (§1) already makes the upstream `Generate` replayable; the general risk (a data-driven `Iterate`/`Group` count sourced from a *replayable* node) is not itself a problem, only a data-driven count sourced from an *unrecorded* read (row above) is. |
| Time-based branches | `time.Now()` used to compute a value that later changes control flow | **Not present as a branch.** `validateproduct.go:176,178,196,198,216,218` calls `time.Now()`/`time.Since` purely to compute `reports[i].ElapsedSeconds`, an informational field written into a report file (`validateproduct.go:184,204,224`) and never compared or branched on. Recomputing it on resume would produce a cosmetically different number in a report, not a different node sequence. |
| Map iteration order in bodies | Go's randomized map iteration order used to build a name or a child list | **Not found.** No workflow body was found ranging over a `map[...]...` to name a `Group` child, an `Iterate` item, or a `PromiseLoop` task; all ranges found are over slices (`for i, x := range someSlice`) or a decoded `[]string`/`[]Group`. |
| Planner backlog files under `scopes/` | `PromiseLoop` writes `scopes/<loop key>/backlog.md` (`loop.go:110-114,149-151`) purely as a rendered copy of the same `PlannerDecision` data already in `run.jsonl` (`loop.go:145-151` writes it right after computing `tasks`, which is the same data logged at `loop.go:161`) | **Not a hazard** — it is downstream of the recorded decision, not an input to it; nothing reads `backlog.md` back into control flow. |
| Wall-clock/network-driven supervisor timing | `WithInterval` ticker (`supervise.go:138`, default 3 min) and Jev screening (`supervise_jev.go:134-156`) decide *when*, mid-turn, a `Steer` fires | **Present, structurally**, but orthogonal to node identity: a supervisor's steer changes what the worker session hears, not which node ids the orchestrator assigns. It affects the *content* of a `TurnEnded.Result`, which resume already treats as an opaque recorded value — the timing hazard matters for R1.3 (steer/supervision), not for R1.4's node-sequence question. |

Net for §2: of the seven hazard categories examined against the stock
workflows, six are structurally absent because every `Group`/session name in
today's code is a compile-time literal and every loop bound that matters
traces back to either a fixed constant or a recorded `Generate` result. The
seventh — an unrecorded `os.ReadFile` at workflow-body scope feeding an
`Iterate` bound — is real, present in a shipped workflow today, and is
exactly the class of side effect gimbal-internals-audit.md §4 already
flagged in general (`os.ReadFile`, `os.Executable`, `filepath.WalkDir` all
appear in stock bodies outside any recorded primitive). Any memoized-resume
design needs either a `gimballint` rule forbidding raw filesystem reads that
feed loop bounds, or a new primitive (`gimbal.ReadFile`-shaped) that records
what it read, before resume can be trusted for `implementation.go` as
written.

## 3. Sessions on resume: reattaching the harness, not just the result

`SessionCreated` (`events.go:105-114`) carries `Name`, `Adapter` (a `%T`
Go type name such as `*claude.adapter`, not an instance or process handle),
`Model`, `Effort`, `Workdir`, `Parent` — and nothing else. It is written at
`session.go:58` and `session.go:631`, **before** any turn runs, which is
exactly when `s.native` is still empty: `Session.native` is populated lazily,
only inside `turn()`, on the first call to `adapter.CreateSession`
(`session.go:26` field doc "made on the first turn"; the call itself at
`session.go:248-256`). So the native harness id — the one thing an adapter
needs to resume a conversation instead of starting a new one — **is never
written into `SessionCreated` at all**, confirming gimbal-internals-audit.md
§1.2's framing of the question exactly.

The native id does end up on disk, but only accidentally and only after the
first turn actually starts: every synthetic native event Gimbal itself
emits around a turn (`session.inbox.enqueued`, `session.execution.started`,
`session.execution.succeeded`, etc., built by `nativeEvent` at
`session.go:435-441` and emitted at, e.g., `session.go:307,374`) carries
`"sessionID": native` inside its opaque `Data` JSON blob, which is what
gets appended to `sessions/<session id>.jsonl` as an `AgentRecord`
(`event_persistence.go:71-90`). So the native id **is recorded**, but only
as an unstructured field inside a JSON blob in the session transcript, not
as a first-class column anywhere a resume path could cheaply query it
without scanning that session's whole transcript for the first
`session.execution.started`-shaped record and pulling `data.sessionID`
back out. This is a real but easy gap to close (add the native id to
`SessionCreated`, written once it exists, or to `TurnStarted`) — it is not
recorded today, and no code path reads it back for reattachment.

What a resumed `Generate` on a session whose native process is gone would
need, per harness, using the round-1 audit's own §3/fact 6 classification,
carried forward and applied to resume specifically:

- **Claude, agy** (spawn a fresh process per turn, resume from the
  harness's own transcript store): resuming is plausible in principle —
  `CreateSession` was already lazy and per-turn state doesn't depend on a
  long-lived daemon — *if* the native id is recovered (per above) and the
  harness's own transcript store is reachable from wherever resume runs.
  Nothing in Gimbal today attempts this.
- **Codex, opencode, pi** (host-pinned local daemon or in-process object,
  audit fact 6, `codex/codex.go:2-9`, `opencode/server.go`,
  `pi/adapter.go:1-3`): the daemon itself dies with the orchestrator process
  (they are not separate today — see §4). Resume for these three adapters
  is not "reattach to a surviving native id," it is "there is nothing to
  reattach to; the only available action is starting a brand new native
  session and losing the harness-side conversation state entirely" — which
  is a correctness change (a `Generate` call whose prompt assumes prior
  turns exist would now run against a blank session), not merely an
  infrastructure gap.
- `agy.Fork` is unsupported outright (`agy/agy.go:168-175`, confirmed this
  session, matching audit fact 7) — a forked session recorded in
  `run.jsonl` via `SessionCreated{Parent: ...}` for an `agy`-backed role
  cannot be recreated by any resume path either, since it could never be
  created live in the first place other than by `agy`'s own limitation
  erroring.

## 4. The in-flight node at crash time

The critical fact for this section is architectural, not a missing feature:
**Gimbal today has no orchestrator/worker split at all.** `RunTurn` and
`os/exec.CommandContext` are called directly, synchronously, from inside
the same process that runs the workflow body and owns `*run`/`*scope`
(`session.go:321`, `command.go:241`, `service.go:72`; audit fact 4: `*run`/
`*scope` "cannot be serialized across a process boundary"). So "the
orchestrator restarts while a turn was running" and "the process that was
running the turn restarts" are, today, the same event. There is no
scenario in the current code where the activity survives the orchestrator's
death, because they are the same OS process.

That reframes the comparison Q4 asks for: it is a comparison between two
*future* architectures (Finalist 1's remote worker, versus a hypothetical
Gimbal-native equivalent), not between Temporal and Gimbal-as-shipped today:

| | Temporal | Gimbal-native (hypothetical, unbuilt) |
|---|---|---|
| Where the in-flight work lives | On a separate activity worker process/pod, addressed by Temporal Server via the activity's scheduled/started Event History entries, independent of which workflow-worker process is running the workflow side | Would need an equivalent: a worker process distinct from the orchestrator, with the orchestrator recording enough to find it again (a worker id, a task handle) — none of which exists; `run.turns` (`run.go:291-301`) is an in-memory `map[string]context.CancelCauseFunc`, gone on restart |
| How the orchestrator re-attaches after its own restart | Re-executes the workflow function from Event History; when replay reaches the point of the still-pending activity, it re-issues the "query the activity's status" logic implicitly, because Server, not the workflow worker, tracks the activity's lease/heartbeat/completion — the workflow simply awaits the (already in-flight or already completed) result again | Would have to explicitly record, before the call, which worker/host owns the in-flight node (a "turn handle," mirroring what `run.turns`/`Session.activeEmit` already do in-process, `session.go:244-246,294-296`, but durably) and, on restart, poll or subscribe to that worker for either "still running" or "here is the buffered result" |
| What happens if the activity/turn finished but nobody heard | Temporal Server already durably recorded `ActivityTaskCompleted` in Event History the moment the activity's worker reported it, regardless of whether the workflow-worker was up to receive it — no result is lost | Depends entirely on whether the (still hypothetical) remote worker buffers its result until acknowledged, per SYNTHESIS.md's framing ("the worker keeps the result until acknowledged"); nothing in the current architecture buffers anything — `TurnEnded`/`CommandEnded` are written by the orchestrator's own event writer (`event_persistence.go:46-53`) once the orchestrator's own call returns, so if the orchestrator is what died, no write happens and the result, if the local child process produced one, is simply never durably recorded |
| A service still running at crash | N/A (services aren't a Temporal concept) | `Service` records `CommandStarted` at start and only records `CommandEnded` at teardown (`service.go:50,56`, `ownedService.stop`) — a service alive when the orchestrator dies leaves an open `CommandStarted` with no `CommandEnded`, and, being a plain child process of the orchestrator today, its survival past the orchestrator's death depends on OS process-group/session semantics the code does not manage for this case (only for the graceful SIGTERM→SIGKILL teardown path, `service.go:202-267`) |

Bottom line: Temporal's answer to "was the in-flight activity still running
when the workflow-worker died" is "the Server always knows, because it
never depended on that specific workflow-worker to know." Gimbal's answer
today is "nobody knows, because nothing outside the orchestrator's own
process durably recorded that a turn or service was in flight, only that it
started." Building the Gimbal-native equivalent is a strict superset of
building the remote-worker split Finalist 1 already needs for R1.1/R1.2 —
it is not separable from that work, which is one more reason to sequence it
after, not before.

## 5. Versioning: is the graph recorded with the run today?

No, explicitly, in the package's own doc comment: `RegisterGraph`'s doc at
`run.go:43-51` states "It is not written into the run's record; a run read
without its binary has the logs" (quoted verbatim from `run.go:46-47`,
confirmed by direct read this session). `graphs` is a process-global
`map[string]workflow.Graph` (`run.go:41`) populated once per binary by
generated `init()` code (`internal/generate/source.go:61`, literally
`func init() { gimbal.RegisterGraph(Graph) }`), keyed by workflow name —
it has no per-run identity, no version, and no hash. `RegisteredGraph`
(`run.go:59-65`) is a pure lookup by name against whatever binary happens to
be running; the doc comment for `Graph` itself confirms this is by design —
graphs describe "a workflow before it runs," deliberately excluding runtime
data (`workflow/graph.go:6-19`).

What would minimally have to be recorded, to let a resume path refuse
rather than silently misbehave on a shape change (the alternatives note's
proposal, durable-execution-alternatives.md "Gimbal is arguably better
positioned..."):

- A content hash of the registered `workflow.Graph` for the run's workflow
  name, written once into `run.jsonl` alongside `RunStarted` (`events.go:33-38`,
  written at `run.go:184-185`) — cheap, and `RunStarted` already carries
  `Name`; it would need one more field, or a sibling event.
- On resume, look up `RegisteredGraph(name)` in the *resuming* binary,
  hash it the same way, and refuse resume outright if the hash differs —
  this is exactly DBOS's `Patch()`-guarded-branch philosophy and Temporal's
  `GetVersion`/`Patched` philosophy, except automatic rather than
  hand-written, because Gimbal already has a structured graph to hash
  instead of asking a developer to remember to bump a version string. This
  is the single concrete advantage over `agenticenv/durable-go`'s
  `WithStepVersion` (opt-in, developer-remembered) noted in round 1 and
  reconfirmed by this session's own read of that repo: its README states
  plainly, "cached results are reused even if `fn` or inputs differ" unless
  a human bumped the version — Gimbal's registered graph gives a mechanical
  alternative no surveyed engine ships as a default.
- This hash check is necessary but **not sufficient**: `workflow.Graph`
  deliberately excludes data flow and branch conditions
  (gimbal-internals-audit.md §5.2, `workflow/graph.go:10-14`), so two
  binaries could hash identically at the graph level while differing in
  exactly the kind of ordinary-Go logic §2 identifies as the real risk (an
  `os.ReadFile` call the graph reader cannot see at all, per audit fact 10
  — `internal/generate/expr.go`/`read.go` do not model raw filesystem
  reads). The graph hash catches "the workflow was edited in a way the
  static reader can see"; it does not catch "the workflow reads different
  bytes from the same file path on two different hosts."

## 6. Effort/risk comparison: Gimbal-native memoized resume vs. orchestrator-as-pod-per-run

| | Gimbal-native memoized resume | Orchestrator as a pod per run (Temporal Workflow-execution-per-run, or a plain Kubernetes Job) |
|---|---|---|
| New code required | A run-log scanner that reconstructs "which node ids are Ended vs. Started-only" on startup; a re-execution mode for `Run` that intercepts each primitive's call and substitutes the recorded result instead of calling the adapter/`os/exec`; a native-session-id field added to `SessionCreated` (or an index built from the session transcript) and a reattach path per adapter (§3); a durable "turn/service handle" replacing the in-memory `run.turns` map (§4); a graph-hash check (§5); a new `gimballint` rule or primitive closing the `os.ReadFile`-into-`Iterate` gap (§2) | Essentially none beyond what R1.1 (host-pinned activity execution) already needs: wrap `gimbal.Run` in a workflow/Job entry point that starts fresh each time; no new Gimbal primitive, no new event schema, no adapter-specific reattach logic |
| Residual risk after building it | Sessions on `codex`/`opencode`/`pi` still cannot resume mid-conversation (§3) — the daemon is gone regardless of how good the log-replay logic is, so a `Generate` mid-conversation on those adapters is not actually resumable, only *restartable with amnesia*. `agy.Fork` remains unresumable by construction. The in-flight-node problem (§4) is only solved once the remote-worker split (R1.1/R1.2) already exists — this is not an independent deliverable. | A run that was mid-turn restarts from empty and repeats every already-billed agent turn, command, and planner decision since the last run boundary; for a `PromiseLoop` hours into dispatch, this is a real cost (re-running turns that already cost real money and wall-clock time), but it is a cost the current single-process Gimbal *already has* — `gimbal.Run` today has no resume path either, so a crash today already means "the run failed, start again" (confirmed: zero resume-related code found anywhere in the module, §0 grep above). Choosing pod-per-run changes nothing about today's actual risk profile; it only moves where the process boundary sits. |
| What it buys, concretely | Avoids re-running completed `Generate`/`RunCommand`/`PlannerDecision` steps after an orchestrator crash — real savings once the remote-worker architecture exists and orchestrator crashes are decoupled from worker crashes | Nothing beyond today's status quo; it is a deployment choice, not new durability, until D is layered on top |
| Sequencing dependency | Strictly depends on R1.1 (host-pinned remote activity execution) and R1.2 (session durability across a worker pool) already existing, because §4 shows the in-flight-node problem only exists once orchestrator and worker are different processes, and §3 shows session reattachment is the same problem R1.2 already has to solve | None — buildable today, independent of every other risk in this research |

**The evidence favours orchestrator-as-pod-per-run as the first step.**
It requires zero new Gimbal code, it does not change today's actual
durability guarantee (which is already "a crash means restart the run" —
confirmed by the total absence of resume code), and every piece memoized
resume needs (a durable turn/service handle, a recovered native session id,
a graph-hash check) is either a subset of, or blocked on, work R1.1/R1.2
already have to do for Finalist 1 to function at all. Building memoized
resume *before* that work exists would mean solving §3's and §4's hardest
parts twice: once in a form specific to "resume within one process" and
again once sessions/commands move to remote workers.

**What would trigger building it anyway:** once R1.1/R1.2 land and workers
are genuinely decoupled from the orchestrator, the trigger is empirical —
measure how often the orchestrator process itself (not a worker pod)
restarts independently of a run's own workers finishing or failing
(Kubernetes node drains, orchestrator deploys, OOM kills of the
orchestrator specifically), and how expensive the average discarded
`PromiseLoop` lap is in agent-turn cost and wall-clock time at that
restart rate. If orchestrator restarts are rare relative to run length
(the likely case for a process that isn't itself doing agent work), pod-
per-run's "just restart" cost stays low and D is not urgent. If they are
frequent (e.g., because the orchestrator is co-scheduled tightly, or
platform policy churns pods aggressively), the savings from short-
circuiting already-completed `Generate`/`RunCommand`/`PlannerDecision`
nodes on restart become the deciding factor, and at that point §2's one
concrete gap (unrecorded filesystem reads feeding loop bounds) and §5's
graph-hash check are the two prerequisites to close first, since both are
independent of the worker-pool architecture and can be built and tested
today, against the existing single-process `Run`, without waiting for R1.1.

## Remaining unknowns

- Whether `Check`'s recorded `ValueSet` (`command.go:200-204`) is keyed
  identically enough to the command's own id that a resume path can
  associate "this `Check` node's cached exit code" with "this scope value"
  without re-deriving the mapping — not traced in this session past the
  call site.
- Exact behavior of `os/exec`-spawned child processes (a `RunCommand` or a
  `Service`) when the parent Gimbal orchestrator process is killed outright
  (SIGKILL, OOM, node eviction) rather than exiting cleanly: whether they
  become orphaned-but-alive (inherited by init/a container's PID 1) or die
  with the process group depends on how the binary is deployed (bare
  process vs. container `PID 1` vs. a supervisor) and was not tested this
  session — it materially changes whether a "service still running at
  crash" scenario (§4) is even physically possible for Gimbal today or is
  moot because the OS already kills every child on orchestrator death.
- Whether DBOS's or Restate's mismatch-detection error (confirmed this
  session: DBOS raises when "recovering a workflow and attempts to execute
  step Y, but finds a checkpoint in the database for step X instead") has
  a natural Gimbal analogue beyond the graph-hash check in §5 — e.g., an
  assertion inside the re-execution path that the next node id the body is
  about to assign matches the next unconsumed `Started`-shaped record in
  `run.jsonl`, refusing resume with a clear error rather than silently
  reusing the wrong cached result. This is a promising, concretely
  specifiable check that was not designed in detail here.
- The actual crash frequency of a hosted Gimbal instance's orchestrator
  process today, which §6's "what would trigger it" section depends on and
  which no data in this checkout answers.

## Sources

Gimbal source, read directly this session (all file:line citations above
are from a fresh read, not carried over from round 1 uncited): `run.go`,
`scope.go`, `group.go`, `events.go`, `event_persistence.go`, `session.go`,
`command.go`, `service.go`, `loop.go`, `harness.go`, `agy/agy.go`,
`workflow/graph.go`, `internal/generate/source.go`,
`internal/observation/replay.go`, and the workflow bodies under
`internal/workflows/{implementation,pyramidsummary,validateproduct,
researchdocument,researchdocument,implementinterview,review,interview}`.

Web, fetched or searched this session (2026-09-27):

- https://github.com/agenticenv/durable-go — verified via fetch: step
  memoization model, "no replay-determinism sandbox," step-ID-rename and
  unbounded-loop limitations, `WithStepVersion` behavior, all quoted above
  and consistent with round 1's citation of the same repo.
- https://docs.temporal.io/workflows and search of
  docs.temporal.io/workflow-execution, docs.temporal.io/workflow-definition
  — verified: replay reconstructs state from Event History; an Activity's
  result, once recorded, "is reused, not recomputed" on replay; the intent
  is to "bring the Workflow back to the exact same state it was in before
  the pause occurred."
- https://docs.restate.dev/concepts/durable_building_blocks/ — verified via
  fetch: only `run()`-wrapped operations are journaled; replay re-executes
  the handler and skips already-completed journaled operations; determinism
  is explicitly scoped to those operations, not the whole handler (matching
  round 1's citation).
- https://github.com/hatchet-dev/durable-execution-the-hard-way — verified
  via fetch (round 1 named this repo `hatchet-dev/durable-execution-the-
  hard-way`; note the canonical URL resolved this session): "Durable
  execution is a mechanism to incrementally checkpoint the state of a
  function as it makes progress... the function can recover from where it
  left off"; distinguishes retries (preserve history) from replays (reset
  history) from forking (branch at a point).
- DBOS step-ordering mismatch behavior — verified via search summarizing
  docs.dbos.dev pages (faq, architecture, workflow/step tutorials): DBOS
  raises an error "when DBOS is recovering a workflow and attempts to
  execute step Y, but finds a checkpoint in the database for step X
  instead," and defines a deterministic workflow as one that "invokes the
  same steps with the same inputs in the same order" every time; not
  independently re-fetched from docs.dbos.dev directly this session past
  the search summary, so treat as secondary/search-summarized, consistent
  with round 1's own confidence-tagging convention.
- https://jack-vanlightly.com/blog/2025/11/24/demystifying-determinism-in-durable-execution
  — verified via fetch: "Re-execution of the control flow requires
  determinism: it must execute based on the same decision state every
  single time"; distinguishes control-flow nondeterminism (must be
  memoized) from side-effect nondeterminism (only needs idempotency);
  names non-deterministic collection iteration as a concrete pitfall
  category, directly relevant to §2's map-iteration hazard.
- https://hatchet.run/blog/durable-execution — verified via fetch:
  describes the retry/idempotency-key model and states plainly that
  "workflows must be deterministic," naming non-deterministic iteration,
  external side effects inside workflow code, and step-reordering-breaks-
  in-flight-history as the three concrete pitfalls; did not compare
  memoization against full replay explicitly, so that comparison in this
  note's own framing (§ verdict, §6) is this note's synthesis, not a quote.

Not independently re-verified this session, carried forward from round 1
as background only (not load-bearing for any claim above): Temporal SDK
version numbers, DBOS/Restate/Hatchet star counts and licensing, and the
broader engine-comparison table in durable-execution-alternatives.md.
