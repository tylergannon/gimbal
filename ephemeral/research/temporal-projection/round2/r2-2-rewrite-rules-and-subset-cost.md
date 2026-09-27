# R2.2: Rewrite-rule catalogue and expressiveness cost of source projection

Date 2026-09-27. Reads, in order: BRIEF.md, round1/SYNTHESIS.md,
round1/gimbal-internals-audit.md §4-5, round1/temporal-go-sdk.md Findings
1-3 and its mapping table, round1/go-rewrite-tooling.md §5. Determinism
rules re-verified directly against `contrib/tools/workflowcheck/README.md`,
raw, 2026-09-27 (Sources). The five stock workflows and
`cmd/gimbal/run_prompt.go` were read in full for this note.

## Verdict

**Yes, with one deliberate exception.** Four of five stock workflows
(`review`, `implementation`, `researchdocument`, `pyramidsummary`) contain
zero constructs `workflowcheck` or a Temporal-shaped determinism lint would
reject outright. Every side effect in their bodies is either already one of
the seven Gimbal primitives (clean rewrite) or a small, enumerable set of
filesystem/host calls round 1 already named, all of which move cleanly into
new activities without changing what the workflow *means*. `review.go`
projects with zero exceptions; `implementation.go` needs exactly one new
activity (its outcomes-file read); `researchdocument.go`/`pyramidsummary.go`
need a handful of new "mkdir" and "check the file the turn just wrote"
activities, all mechanical.

`validateproduct.go` is the exception. It schedules real, unlogged
subprocesses (`exec.CommandContext` at lines 122 and 232) directly from the
workflow body, uses wall-clock time for elapsed-time accounting, sets a real
timeout with `context.WithTimeout`, and structures its own cleanup as three
`defer`s calling those same raw operations. None of this is exotic Go, but
every line is currently invisible to both `gimballint` and
`internal/generate`, and a naive projection would silently reproduce
non-determinism. The rewrite is still mechanical — nothing here defeats a
type-directed dst rewrite, per go-rewrite-tooling.md §5.1's closed-and-
enumerable claim — but the fraction of the file's meaning that moves into
activities is materially larger, and Gimbal today has no lint rule that
would catch a sixth workflow written the same way before it ships.

The expressiveness cost is real but bounded and enumerable: a rule
catalogue, not a wall. It is smaller than round 1 feared — writes into
shared arrays from `Group` children (round 1's most alarming finding) are
safe under Temporal's cooperative scheduler, not merely tolerable (§4a).
`cmd/gimbal/run_prompt.go`'s closure (lines 104-120) is not a recognized
workflow entry (`internal/generate/entry.go:33-52` requires a
`gimbal.Env`/params signature this lacks) and is GIMBAL108-exempt by name
(`internal/gimballint/rules.md:172-174`); it is out of scope for a source
projection by construction and is only touched once more, in §2.

---

## 1. The rule catalogue

"Mechanical" = a type-directed dst pass with one fixed replacement shape
handles every occurrence. "Manual boilerplate" = the replacement shape is
fixed but long ("real, repeated boilerplate that Temporal does not hide,"
Finding 2). "New activity" = workflow-body logic must relocate into an
activity. "Forbidden" = `workflowcheck` would reject it, or the SDK gives no
path at all.

**Entry signature/params.** `func X(ctx context.Context, env gimbal.Env,
params P) error` (`internal/generate/entry.go:33-52`) becomes
`func X(ctx workflow.Context, params P) (R, error)`. `gimbal.Env.WorkDir`
("the absolute initial working directory selected for the run," go doc) has
no workflow-level meaning under Temporal — no filesystem there — so it must
become an explicit string threaded into every activity call that already
takes a workdir argument. Mechanical, but a real narrowing: one value read
once becomes dozens of copied arguments, and `ContinueAsNew` has to
re-thread it explicitly too.

**`ctx` type through helpers/closures.** `context.Context`→`workflow.Context`
at every binding site the extractor already resolves: a
`Scope`/`Group.Go`/`Iterate`/`PromiseLoop.Tasks` callback literal, and a
same-package helper called with a context argument, inlined exactly as
`internal/generate/expr.go`'s `inline` already does (go-rewrite-tooling.md
§5.1). GIMBAL101 (`rules.md:8-31`) already forbids the two things that would
defeat this — a worker from a collection, or an opaque parameter — so the
inputs are a closed set. Mechanical. None of the three `absoluteFrom`
helpers (`implementation.go:158`, `researchdocument.go:395`,
`pyramidsummary.go:530`) even take a `ctx` argument, so this wrinkle does
not occur in the five workflows.

**`Scope`.** Function boundary plus a `workflow.WithCancel`-derived child
context (mapping table). Mechanical, clean. Used at
`researchdocument.go:321`, `pyramidsummary.go:439`.

**`Group.Go`/`Wait`, first-error-cancels, `Killed`.** No packaged errgroup
type exists (Finding 2): each site expands to one `workflow.Go` per child,
one `workflow.WithCancel`-derived context per child, a `Selector` watching
for the first error to cancel the others, and a drain loop that still
`Get`s every child — exactly Gimbal's contract ("the caller must Wait on
every exit path," go doc). Manual boilerplate, not forbidden. Four sites:
`researchdocument.go:197`, `pyramidsummary.go:164,320`,
`validateproduct.go:170`. `Killed` on one child cancels only that child's
derived context; a whole-run kill maps to
`CancelWorkflowExecution`/`TerminateWorkflowExecution`. Reaching the
activity attempt is bounded by heartbeat cadence, not instantaneous
(temporal-go-sdk.md:276-279) — a real behavior change, not just syntax.

**`Iterate`.** Ordinary `for`/`range` over a slice — only map-range and
channel-range are non-deterministic (verified below). Mechanical, clean.
Every `range` in the five workflows' bodies (grepped exhaustively) ranges
over a slice, array, or `iter.Seq2` — never a map. `Iterate`'s per-item
child-scope-close needs one `Scope` wrapper per iteration.

**`PromiseLoop.Tasks` and its planner turn.** No packaged planner-loop
primitive (mapping table): the planner turn is one Activity per decision,
each `Tasks` iteration is a further `Scope` around further Group/Generate
calls, and "an operator message waits for the next decision" (go doc) is
the same Signal "read once per decision" shape Finding 5 derives for Steer
at the loop level. Manual boilerplate. Sites: `implementation.go:100,106`,
`pyramidsummary.go:265,268,421,424`.

**`NewSession`/`Fork`.** `NewSession`→`workflow.CreateSession` plus
`worker.SessionWorker` — "pin every later activity to the exact host that
accepted this session" (mapping table), the cleanest match in the table
(Finding 4). Mechanical, clean. `Fork` is adapter-level and process-local,
"a plain call inside an activity" (mapping table) — unchanged, never
crosses the workflow/activity boundary.

**`Generate[T]`, with/without `WithSupervisor`/`WithScopeTemplate`.**
Without a supervisor: one Activity, `StartToCloseTimeout` sized to the turn,
`HeartbeatTimeout` for liveness (Finding 3); only the validated `T` returns,
inside the 2MB limit. Mechanical, clean. With `WithSupervisor`: **no direct
construct** — "There is no functionality to send signal to an activity"
(Finding 5, verified via the Temporal community forum) — needs the
worker-local side channel Finding 5/R1.3 describe, built once, reused
everywhere. Used in most turns across `researchdocument`/`pyramidsummary`/
`implementation`; none of the five ever calls `Steer` or `WithScopeTemplate`
directly (grepped, zero hits) — this note answers the rendering question
below from the API contract, not a worked stock-workflow example.

**`Set`/`SetJSON` and where the rendered prompt is computed.** Storage:
ordinary Go values threaded as activity arguments, and across
`ContinueAsNew` as explicit new-run arguments (mapping table). Mechanical,
with a cost: "everything in scope stays implicitly visible" becomes "every
value must be re-threaded by generated code" (Finding 3, Risk 4). The harder
question: today, "Generate appends that scope's rendered context to the
prompt itself" (go doc) inside Gimbal's own runtime, not as anything a
workflow author calls — pure string computation, no I/O, no clock, nothing
non-deterministic about it, but not part of the public contract a workflow
body invokes, and Temporal activities have no implicit reach into "the
calling workflow's scope stack." Two options, and this note's unverified
judgment: (a) render in the workflow, reimplementing the render/`template`
algorithm as new, workflow-visible generated code purely so the flattened
string can cross as the Activity argument; or (b) render in the activity,
passing the constant prompt/instruction plus a serialized snapshot of
visible `ScopeValue`s (values the workflow already carries) as separate
arguments, letting the worker own today's rendering code unchanged. (b) is
the better fit: zero new workflow-visible logic needed (activities carry no
determinism constraint, Finding 1), a more structured payload against the
2MB ceiling, and no correctness reason to replay pure string-building —
Gimbal's own `TurnStarted`/`ContextEntry` records already capture "what was
sent," independent of Temporal's Event History.

**`RunCommand`/`Check`/`Service`.** `RunCommand`/`Check`: an Activity under a
plain `context.Context`, large streams already claim-checked (bounded
excerpt plus path, Finding 8). Mechanical, clean. `Service`: **awkward** — no
"keep a subprocess alive for a scope's life" primitive exists; it becomes
one long Activity whose job is heartbeat-and-react-at-cancellation, slower
than Gimbal's own SIGTERM-then-SIGKILL sequence. Used once,
`validateproduct.go:149`.

**`Interview`.** `workflow.SetUpdateHandlerWithOptions` plus a posting
activity for the question — Updates are "a trackable synchronous request...
with an optional validator" (Finding 5), a clean match. Not used by any of
the five (present in `internal/workflows/interview`/`implementinterview`,
outside this note's five).

**Error handling: `errors.Is`/`context.Canceled` vs
`temporal.CanceledError`.** Not exercised by any of the five today — grepped
zero hits for `errors.Is|context.Canceled|context.Cause|errors.As`; the only
cancellation checks are raw `ctx.Err()` (`implementation.go:152`,
`validateproduct.go:244`). Under Temporal, an activity's own context still
yields ordinary `context.Canceled` to the activity author (Finding 1), but a
workflow's view of a cancelled Activity via `Future.Get` is a
`*temporal.CanceledError`, not `context.Canceled` — a future workflow
adopting Gimbal's documented `errors.As(context.Cause(ctx), &killed)`
pattern (`Killed`, go doc) at the workflow level would need that translated.
Load-bearing for future workflows, not a defect in the five today.

**`defer`.** Only `validateproduct.go:102,108,135` (§4a). Temporal permits
`defer` itself — nothing in `workflowcheck`'s list forbids the statement —
so what needs to move is what each deferred closure *does*: mutate shared
result state (safe, §4a) while calling raw `exec`/`os` operations
(not safe as written).

**`time.Now`.** Explicitly non-deterministic per `workflowcheck`'s default
list. Three pairs, `validateproduct.go:176/178,196/198,216/218`, measuring
elapsed wall-clock seconds. Forbidden as written; `workflow.Now(ctx)` is
Temporal's replay-safe answer.

**`os.*`/`exec.*` in bodies.** Not one of `os.ReadFile`, `os.Stat`,
`os.MkdirAll`, `os.MkdirTemp`, `os.WriteFile`, `os.Rename`, `os.Executable`,
`filepath.WalkDir`, `exec.LookPath`, `exec.CommandContext` appears on
`workflowcheck`'s own default list — verified below, the list is exactly
`crypto/rand.Reader`, `math/rand.globalRand`, `os.Stderr`/`Stdin`/`Stdout`,
`time.Now`, `time.Sleep`, plus goroutine-start, channel send/receive/range,
map range, nothing else. This is the sharpest precision this round adds:
Temporal's own tool is silent on every "hard" finding in
`gimbal-internals-audit.md §4`, because none touches a declared function or
variable by name, and (with two likely exceptions) none transitively
reaches one either. They remain real bugs under a source projection —
`workflowcheck`'s own README says so ("does not catch global var mutation...
just a helper"); Gimbal's lint has to add this coverage from scratch (§3).
Two calls likely *are* caught transitively — inference from stdlib
behavior, not a verified run this session: `context.WithTimeout`/
`WithDeadline` (`validateproduct.go:101,154`) call `time.Now()` internally
for the deadline, so `workflowcheck`'s hierarchical analysis (Finding 1)
should flag them the way it flags `fmt.Printf` for touching `os.Stdout`;
and `cmd.CombinedOutput()` (`validateproduct.go:125,236`) multiplexes
`Stdout`/`Stderr` through an internal goroutine in `os/exec`'s own
implementation, so it should be flagged as "starts a goroutine" too.

**`fmt.Errorf`.** Pure formatting, no I/O — unchanged, dozens of sites
across all five, none flagged, none moves.

**`strconv`.** `strconv.Atoi` at `researchdocument.go:330`,
`pyramidsummary.go:472` — parsing a command's own stdout, already an
activity return value by then. Unchanged.

**`filepath`.** `Join`/`Abs`/`IsAbs`/`Dir`/`Base`/`Clean`/`Rel` — pure
string manipulation, unchanged, everywhere (`implementation.go:162-165`,
`researchdocument.go:141,185,193,347,354`, `pyramidsummary.go:134,185`,
etc.). `filepath.EvalSymlinks` (`config.go:93`, inside `readSuite`) does
stat the filesystem — discussed with `readSuite` in §4a.

---

## 2. Statement census

Method: every top-level statement in the entry function plus every
same-package helper a dst rewrite would inline (go-rewrite-tooling.md
§5.1) — reaches `absoluteFrom` in three workflows,
`verifyResearchFloor`/`requireNonemptyFile` in two, `readSuite`/`absolute`/
`shellQuote`/`writeJSON`/`errorText`/`closeBrowsers` in `validateproduct`,
and the small pure helpers in `pyramidsummary`. Counts are this note's own
line-by-line pass, not a machine-verified AST walk — defensible to within a
few statements per workflow; the load-bearing part is the exact forbidden
lines below.

| Workflow | Unchanged | Mech. rewritten | Needs activity | Forbidden | Total |
|---|---|---|---|---|---|
| `review` (37 lines) | 3 | 4 | 0 | 0 | 7 |
| `implementation` (174 lines) | ~26 | ~19 | 1 | 0 | ~46 |
| `researchdocument` (465 lines) | ~55 | ~48 | ~16 | 0 | ~119 |
| `pyramidsummary` (567 lines) | ~70 | ~55 | ~18 | 0 | ~143 |
| `validateproduct` (277+127 lines) | ~48 | ~22 | ~14 | ~11 | ~95 |
| **Totals** | **~202** | **~148** | **~49** | **~11** | **~410** |

Forbidden lines, exact:

- `validateproduct.go:101` — `context.WithTimeout(ctx, timeout)`, the run's
  own deadline; likely transitively flagged, and structurally wrong anyway —
  belongs on `workflow.NewTimer`/a `Selector`, not a wrapped context.
- `validateproduct.go:102` — `defer cancel()`, paired with the line above.
- `validateproduct.go:108` — deferred `writeJSON` call, which does
  `os.WriteFile`/`os.Rename` (`validateproduct.go:262-271`) from workflow
  code.
- `validateproduct.go:135` — deferred `closeBrowsers` call, wrapping the
  forbidden `exec.CommandContext` at line 122.
- `validateproduct.go:122` — `exec.CommandContext(cleanup, driver, ...)`
  inside `closeBrowsers`, a raw subprocess launch outside
  `RunCommand`/`Check`, invisible to the run log, the graph, and the page.
- `validateproduct.go:154` — `context.WithTimeout(ctx, 30*time.Second)` for
  the readiness poll, same class as line 101.
- `validateproduct.go:176,178` — `time.Now()`/`time.Since(start)`, tester1.
- `validateproduct.go:196,198` — same, tester2.
- `validateproduct.go:216,218` — same, tester3.
- `validateproduct.go:232` — `exec.CommandContext(ctx, ffmpeg, ...)`, the
  ffmpeg conversion, the second wholly unlogged subprocess launch.

Eleven distinct forbidden source lines, zero in the other four workflows —
every side effect there resolves to "needs an activity," never to a
construct `workflowcheck` would reject or the SDK cannot express.

`cmd/gimbal/run_prompt.go`'s closure is excluded from the table:
`internal/generate` does not recognize it as an entry at all, so a source
projection has nothing to rewrite there. For the record its body has one
`context.WithTimeout` (line 106) and `defer cancel()` (line 107) of the same
forbidden shape as `validateproduct.go` — worth knowing if this path is ever
folded into the generated-workflow contract, not actionable today.

---

## 3. The determinism subset Gimbal's lint would need

`workflowcheck`'s own coverage (verified directly, raw README, 2026-09-27):
non-deterministic functions/vars are exactly `crypto/rand.Reader`,
`math/rand.globalRand`, `os.Stderr`, `os.Stdin`, `os.Stdout`, `time.Now`,
`time.Sleep`; non-deterministic constructs are starting a goroutine, channel
send/receive/range, and map range. Its README states its own limit in one
line: "This will not catch all cases of non-determinism such as global var
mutation. This is just a helper." File I/O, process execution, and host
identity are not on the list at all — not an oversight, but scope:
`workflowcheck` polices generic Go inside replay-based orchestration; it was
never asked "does this function talk to the outside world," a
domain-specific judgment Gimbal is better placed to make about its own
workflow bodies.

Proposed rules, built the same way GIMBAL101-109 already are — a
`go/analysis` pass over the recognized workflow domain, reporting a call
whose callee resolves via `go/types` to a fixed named list, the same style
`internal/generate/expr.go`'s name-switch and GIMBAL101's `takesContext`
reachability already use. None needs `go/callgraph`'s heavier RTA/VTA
(go-rewrite-tooling.md §5.1); each is checkable package-locally with the
analyzer style already in `internal/gimballint` (830 lines today).

- **GIMBAL110 — no filesystem I/O in a workflow body.** Forbids
  `os.ReadFile`/`WriteFile`/`Stat`/`MkdirAll`/`MkdirTemp`/`Rename`/`ReadDir`,
  `filepath.WalkDir`, called directly, outside a
  `RunCommand`/`Check`/`Service` argument position. Package-local, same
  reachability GIMBAL101 already computes.
- **GIMBAL111 — no process execution in a workflow body.** Forbids
  `os/exec.Command`/`CommandContext`/`LookPath` called directly rather than
  through `RunCommand`/`Check`/`Service`. Closes exactly the "invisible
  subprocess" hole `validateproduct.go:122,232` exploits, which no existing
  rule or diagnostic currently sees.
- **GIMBAL112 — no host clock or host identity in a workflow body.**
  Forbids `time.Now`/`Since`/`Sleep`, `context.WithTimeout`/`WithDeadline`,
  `os.Executable`, `os.Hostname`. `time.Now`/`Sleep` duplicate
  `workflowcheck` harmlessly (catches it earlier, at `gimbal lint` time);
  the rest is exactly the class `workflowcheck`'s list misses today and
  that `researchdocument`/`pyramidsummary`/`validateproduct` actually use.
- **GIMBAL113 — no `defer` in a workflow body.** Not a Temporal requirement
  (`defer` is legal SDK-side); Gimbal choosing to be stricter, matching its
  own `Scope`-as-function-boundary idiom ("The scope ends when body
  returns," go doc) rather than a closure that outlives the return. Policy,
  Tyler's call by name per AGENTS.md — proposed, not asserted.
- No new rule needed for `math/rand`/`crypto/rand`: already on
  `workflowcheck`'s own default list, and none of the five stock workflows
  uses either in its body.

All four are dialect-independent in the sense §5 needs: "a workflow body
must not touch the live filesystem, a live process, or a live clock/identity
directly" is true of every replay-based (or relocatable) engine
`alternatives.md` considered, not a Temporal-specific convenience.

---

## 4. The two hardest cases

### 4a. `validateproduct`'s `Group` children, shared arrays, `defer`s, raw `exec`

Round 1 flagged the three `Group` children writing into shared arrays
(`reports`, `turns [3]error`, `opened`/`recording [3]bool`) as
"unsynchronized shared mutable state that a naive Temporal translation could
not leave as real goroutines + shared arrays." Having read Finding 2 in
full, this needs a correction — the most useful thing this round found:
`workflow.Go` coroutines are cooperatively scheduled inside a
single-threaded deterministic scheduler; "there is no real parallelism,
there are no data races to reason about inside workflow code"
(temporal-go-sdk.md:88-90). Each child here only writes its own index
(`reports[0]`/`turns[0]` in `tester1`, and so on — verified by reading all
three closures), the pattern round 1 itself already noted is not a race even
in raw Go. Under `workflow.Go` it is not merely "not a race" — it is fully
deterministic and replay-safe, because every coroutine's write is ordered
relative to every other's blocking points identically on every replay. **The
array-write mechanism needs no rewrite at all**, once `Group.Go` expands to
`workflow.Go` per §1. What moves is the *value* written: `time.Since(start)`
(forbidden, GIMBAL112) and the turn text written via
`os.WriteFile(reports[i].Report, ...)` (`validateproduct.go:184,204,224` —
needs an activity, since it is the turn's own output persisted to a
run-owned file, the same "must run wherever the workdir actually is"
reasoning already applied to `researchdocument`'s `WalkDir`/`Stat`).

The `defer`s are harder: Temporal has nothing like "run this after the
function returns, whether it errored or not, on the same coroutine, closing
over live shared state" — exactly what a `defer` gives for free, and what an
activity boundary does not. `closeBrowsers` (deferred at line 135) has to
become an explicit, always-run final step (a `Scope`-shaped block after
`users.Wait()`, where the teardown logically belongs anyway) that runs
regardless of which earlier `return` fired — a real control-flow
restructuring, not a syntax swap, since a projected version must reconstruct
"runs even on an early error" with explicit error-joining at every early
return instead of one `defer`. The `writeJSON` defer (line 108) is milder
(a status file, not process teardown) but has the same restructuring cost.
Concretely: of `validateproduct.go`'s 277 lines, roughly a quarter (the two
`defer`-guarded closures, the raw `exec` calls they and the main body make,
plus the restructuring their removal requires) move into new activity
bodies or new explicit workflow-level sequencing — matching §2's "needs
activity"+"forbidden" columns (~25 of ~95 counted statements). The
workflow's meaning — "run up to three testers, always clean up their
browsers, always convert their video, always write the report" — is fully
preserved; what moves is the mechanism guaranteeing "always," from a
language feature to explicit sequencing.

### 4b. `researchdocument`'s `WalkDir`/`Stat` over agent output, `os.Executable` in scope

`verifyResearchFloor` (`researchdocument.go:405-426`) is called from inside
all five `research.Go` children (lines 211,230,249,268,287) and once more
from the gap-repair path (line 369). It `filepath.WalkDir`s a topic's
`sources` directory and calls `requireNonemptyFile` on its `INDEX.md` —
inspecting filesystem state a *worker's agent turn just wrote*, moments
after that turn's `Generate` returned. Round 1's sharpest "hard" case,
re-read against Finding 4 (Sessions) is precise about why: `NewSession`
pins every later activity on that session's context to the exact worker
process that accepted it. The check has to run against *that same worker's
disk*, not the orchestrator's or an arbitrary one — so it cannot become
ordinary workflow code (no disk there) or an independently-scheduled
activity (wrong worker); it must become a further activity issued on the
same session context, immediately after the `Generate` call, inside the same
`research.Go` child. Mechanically this is the same shape as
`RunCommand`/`Check` — "needs an activity," not "forbidden" — but it is a
*new* activity Gimbal does not have today, unless `Check`'s existing "run a
command, record the result" shape is judged to already cover it (`test -s
dir/INDEX.md && find dir/sources -type f | wc -l`, run through `Check`
rather than bespoke Go), which it plausibly does, since gathering evidence
for a following turn is exactly `Check`'s stated purpose (go doc). That
choice is Tyler's to make by name; if he takes it, no new Gimbal API is
needed, only a shell one-liner through an existing primitive.

`os.Executable()` (line 143) is structurally different: it resolves *the
orchestrator's own running binary path*, stored in scope context as `"token
counter executable"` (line 159) so later `RunCommand` calls invoke `gimbal
count-tokens` as a subprocess of itself (`researchdocument.go:323`,
`pyramidsummary.go:465` reuse the identical pattern). Under Finalist 1
(runtime projection) this is nearly free, since the orchestrator and the
`RunCommand`-executing worker are the same process today. Under Finalist 2,
the workflow function runs in a Temporal worker process that may not be the
image that later runs the `RunCommand` calls, and even if it is, the
workflow's own answer is meaningless to reproduce on replay — exactly the
host-identity class GIMBAL112 proposes to forbid. Fix: resolve it in the
first activity that needs it (or a small dedicated activity run once, its
result threaded through `Set` like any other value, unchanged under
GIMBAL102/107) rather than in the orchestrating workflow code —
functionally identical to `pyramidsummary.go:124`'s same pattern.

Both cases move real behavior into activities without changing what a
reader of the workflow sees it accomplish; the tax is entirely in *where*
the computation runs, never in *what* it computes — the same conclusion
§4a reached, and the strongest evidence for "the expressiveness cost is real
but bounded."

---

## 5. Gimbal-generic vs. consumer-template estimate

`internal/generate` is 3001 lines today and already does the *identification*
half of a projector's job for the page: find the entry function
(`entry.go:33-52`), walk its body (`stmt.go:12-56`), recognize the closed
set of Gimbal-primitive call names plus same-package-helper inlining
(`expr.go:133-284`). None of that is Temporal-specific; it is the
reachability analysis any target needs first, and it already exists,
tested, in the tree. `internal/gimballint` (830 lines) is the same story for
lint, and §3's four proposed rules extend it in the same dialect-independent
style — "forbid these named stdlib calls in the recognized workflow domain"
would read identically against DBOS or Restate.

What is unavoidably Temporal-specific text, per Finding 2/Finding 4's own
observation that no packaged errgroup or planner-loop type exists: the
replacement bodies for `Group` (the `workflow.Go`+`Selector`+per-child-
`WithCancel`+drain-loop expansion), `PromiseLoop` (planner-turn Activity plus
the same Selector-shaped dispatch), `NewSession`
(`workflow.CreateSession`/`SessionWorker` wiring), `Interview`
(`SetUpdateHandlerWithOptions` plus a posting activity), and the
`ContinueAsNew` argument-re-threading Risk 4 names. None transfers to a
different engine — a Restate consumer template would print entirely
different code for the identical `Group` site, since its own primitives
(not read this round, noted as a live alternative in `SYNTHESIS.md`
direction F) are shaped differently.

Estimate, by rough proportion of what a full generator would add beyond
today's `internal/generate`+`internal/gimballint`: the identification work
(every call site, every ctx binding, every helper inline, plus GIMBAL110-113)
is Gimbal-generic and mostly already written — call it 55-65% of the total
by line count, extrapolating from the existing 3001+830-line baseline. The
emission work — Temporal-shaped replacement text per site, worker/task-queue
registration, `ContinueAsNew` threading — is 100% Temporal-specific with no
existing analog to extrapolate from directly; given how much boilerplate
Finding 2 documents per `Group`/`PromiseLoop` site (a full expansion at
*every* call site, not a one-line substitution), this is plausibly the
larger half by raw line count despite being conceptually the simpler,
template-shaped part — call it 35-45%. Judgment, not measurement: Finalist 2
does not exist yet.

Tyler's own framing — consumers supply the template that does the
transformation (BRIEF.md) — maps cleanly onto this split: the
identification/lint half stays in `gimbal` and ships once; the emission half
is what a consumer template owns, and nothing here requires the two to be
coupled — the same identified-call-site list could feed a different
template for a different consumer without re-deriving identification.

---

## Remaining unknowns

- Whether `workflowcheck` actually flags `context.WithTimeout`/`WithDeadline`
  and `cmd.CombinedOutput()` transitively is inferred from stdlib behavior,
  not confirmed by running the tool this session — cheap to check first.
- Whether `Check`'s existing shape replaces `verifyResearchFloor`/
  `requireNonemptyFile`, or a new "assert a file's state" primitive is
  wanted, is Tyler's call, not resolved here.
- Whether prompt rendering (§1) belongs in the workflow or the activity is
  judgment, not a working implementation; a spike projecting `review.go`
  end to end would settle it cheaply and give R2.1 a real `Generate[T]`
  fixture too.
- `ContinueAsNew` re-threading cost for a `PromiseLoop`-heavy workflow is
  R2.3's question, not sized here. `Service`'s activity-shaped heartbeat-loop
  replacement (used once, `validateproduct.go:149`) serves R1.3/R2.4 more
  than this note.

---

## Sources

Verified directly this session (raw fetch, not summarized):

- `https://raw.githubusercontent.com/temporalio/sdk-go/master/contrib/tools/workflowcheck/README.md`
  — read 2026-09-27; source for every determinism-rule claim in §1 and §3.

Read in full this session from the checkout at `/home/user/gimbal`:
`review/review.go` (37 lines), `implementation/implementation.go` (174),
`researchdocument/researchdocument.go` (465), `pyramidsummary/
pyramidsummary.go` (567), `validateproduct/validateproduct.go` (277) and
`validateproduct/config.go` (127), `cmd/gimbal/run_prompt.go` (277),
`internal/gimballint/rules.md` (all 9 rules), `internal/generate/
entry.go:1-60`, and `go doc -all .` (run 2026-09-27, cited throughout).

Carried forward from round 1, re-cited by section/line, not re-read in
full: `round1/gimbal-internals-audit.md` §4-5, `round1/temporal-go-sdk.md`
Findings 1-3 and the mapping table, `round1/go-rewrite-tooling.md` §5,
`round1/SYNTHESIS.md`, `BRIEF.md`.

Line counts (`wc -l`) for §5: `internal/generate/*.go` = 3001 lines;
`internal/gimballint/*.go` = 830 lines; counted 2026-09-27 against the
checkout.
