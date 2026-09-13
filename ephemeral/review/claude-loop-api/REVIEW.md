# Gimble Loop review, independent of the Codex review — 2026-09-12

Reviewer: Claude (Fable 5.1), in a detached worktree at the merged PR 140
commit `2a52971`. Same brief Tyler gave Codex this morning: a complete review
of the system, its design principles, its API, and PR 140's Loop, judged on
simplicity and capacity, with three positive marks and three fixes. Read-only
apart from this directory. Codex's review was read *after* the code and
probes below were done; the response to it is a separate section at the end.

Evidence beside this file: `probes/` and `probes.txt` (fake adapter),
`ceremony.txt`, `build.txt`, `go-vet.txt`, `go-test-race.txt`,
`web-tests.txt`, `live/` (a real Codex + Claude run), `prompts.txt`
(what each agent was shown), `snapshot-midrun.json`.

## Verdict

> **Amended after discussion with Codex (see `rounds/` and `DISCUSSION.md`).**
> Fix 3 (panicking `Set`) is withdrawn: `math.NaN()` is a real marshal
> failure, and the runtime has no panic boundary, so a panic would leave a
> run without its terminal record. Two sentences were overstated and are
> corrected in place below: Gimble controls *its* contribution to a prompt,
> not the harness's own context; scope identity, data, and lifetime agree
> at Gimble's event boundary and not yet at the harness boundary. The
> original text is otherwise unchanged; the joint result is
> `RECOMMENDATIONS.md`.

Keep it. The five names that matter (`Run`, `Scope`, `Group`, `Loop`,
`Session.Generate`) compose the way `errgroup`, `context`, and
range-over-func compose, and a workflow reads as a page of Go. PR 140's
`Task` is the right shape and its three-way separation (dispatch, validation,
fulfillment) is the best design decision in the repository.

The parts that do not yet feel like Go are (1) the planner protocol, which
asks one model to say the same thing twice in two formats and then ends the
whole loop when the two copies differ by a newline, (2) an ownership promise
about agent processes that the adapter interface gives the runtime no way to
keep, and (3) the amount of `if err != nil { return err }` a workflow author
types around calls that can only fail through their own programming error.

Gates at this commit, in a fresh worktree (`just build` first, issue 112):

| Check | Result |
| --- | --- |
| `just build` | exit 0 |
| `go vet ./...` | clean |
| `go test -race -count=1 ./...` | 11 packages ok |
| `web: pnpm test` | 8 pass |
| Live run (below) | see `live/` |

## Three positive marks

### 1. The workflow is the program, and the program is short

`for ctx, task := range loop.Tasks { ... }` with `loop.Err()` afterward is
the `bufio.Scanner` idiom applied to dispatch, and it is the correct idiom:
the assignment and the lifetime that owns it arrive together, `break` and
`return` mean what they mean in Go, and the task scope ends when the body
returns (proven by `TestLoopEndsTaskScopeOnBreak` and the cancellation test).
`Group` is `errgroup` with a name and a scope. `Scope` is a function call.
There is no registry, no graph object, no callback table, and no named
tactic. The sprint workflow's control flow, minus its prompt constants, is
about 210 lines including validation, commit, and a three-round validator
loop; my live workflow is under 150. Both were written without reading
anything but `go doc`.

The two decisions that make this hold under load: `Loop` does not execute
`Task.Validation` (the workflow does, visibly, and records what happened), and
`Generate` injects nothing (the author pastes `ScopeText` where they want
it). Every contribution *Gimble* makes to a prompt is a line in the workflow (the harness's own instructions, history, memory, and tools are not; see the live exercise).

### 2. Scope is one concept doing three jobs, and the three agree at Gimble's boundary

A scope is at once the unit of identity (the key `delivery.1/task.2/worker.1`
on every event), the unit of data (`Set` once per key, revision by shadowing
in a child, `ScopeText` rendering the chain outermost-first), and the unit of
lifetime (sessions belong to the scope that made them and are closed when it
ends; `Group.Wait` is the join). Because the three coincide, the durable log
falls out with no extra design: `run.jsonl` is the scope tree, every `Set` is
a `value_set` event, and a task's `scope_began` carries its `Task`. The
set-once rule turns two goroutines racing on a key into an error naming the
scope instead of a silent overwrite. The live run's `prompts.txt` shows role
shadowing working exactly as documented: the root says "You plan", the task
scope says "You implement", the reviewer's group child says "read-only
validator", and each agent saw only its own.

### 3. PR 140 draws the authority lines in the right places

`Task{Name, Description, DefinitionOfDone, Validation{Command, Query}}`
replaces a lap counter and a string with an assignment a worker can act on
and a validator can judge. The same value travels through the planner's
structured answer, the yielded Go value, the task scope, the backlog, and two
lifecycle events, and `TestLoopCarriesStructuredTaskAndFeedback` checks that
a later backlog edit cannot rewrite what was dispatched. The planner may end
dispatch but may not certify the goal; the deterministic check outranks the
prose; the enclosing workflow decides whether to stop. The planner prompt
says all of this in plain English, including "a prose claim or agent judgment
cannot override a nonzero command exit", which the worklog shows was learned
the hard way from a live Luna worker claiming a probe passed when the
recorded exit was 7. That lesson is now in the primitive.

## Three things to fix, in priority order

### 1. The planner protocol has two channels; make it one

`loop.go:89-122`: each dispatch asks the planner to *edit a YAML file* and
*return a structured `plan`*, then requires the returned `Task` to be
`==`-equal to an element parsed from the file. Three consequences, all
demonstrated in `probes.txt`:

- **P1.** The planner writes a valid backlog whose description uses a YAML
  block scalar (`description: |`), the natural way to write a paragraph in
  YAML, and returns the same task as JSON. `|` keeps a trailing newline; JSON
  has none. Result: `yielded=0`, error `did not preserve that assignment`.
  The whole Loop ends, and with it the sprint.
- **P3.** One trailing space inside a quoted description: same fatal end.
- **P2.** A backlog that never parses is retried without bound: 7 planner
  calls against a body that would have accepted one task. The only bound is
  the ctx deadline.

So the two planner formatting failures the protocol invites are handled in
opposite ways: unparseable is retried forever, parseable-but-not-identical is
terminal. The worklog records this asymmetry as a deliberate choice after a
real Luna run wrote an unquoted colon, and the round-02 reviewer called exact
equality a nitpick. P1 says it is not a nitpick: a well-formed, idiomatic
backlog kills the loop. Codex's live run this morning hit the other branch
(a scanner error and a 25-second repair turn).

The root cause is asking for the same fact twice. Both Codex and Claude
enforce a JSON Schema natively on structured output; a YAML file has no such
guard. Proposed shape:

```go
// The planner's whole answer. Nothing is written by the planner.
type plan struct {
	Tasks []Task                 `json:"tasks"` // the revised backlog
	Next  polytype.Nullable[int] `json:"next"`  // index into Tasks, or null to end dispatch
}
```

The runtime then *writes* `backlog.md` from `plan.Tasks` for humans, the
page, and the next prompt (which already inlines it). This deletes
`readBacklog`, the YAML dependency, the repair loop, the goal-immutability
check, and the equality check (about 60 lines), removes a failure class
instead of bounding it, and makes the planner host-neutral: an adapter that
can only return JSON, or a planner sandboxed to its own workdir, can plan.
The backlog file path no longer needs to be writable from the planner's cwd
(today it is under `.gimble/runs/...`, outside my live run's workspace, and
works only because Codex runs with `danger-full-access`).

If Tyler wants the planner to keep free-form notes, that is a second string
field on `plan`, not a second channel.

### 2. The ownership promise needs a hook in the adapter

`API.md` and the Godoc promise that "every agent process has a definite end"
because sessions belong to scopes. `scope.go:86-100` (`end`) keeps that
promise by setting `session.closed = true` and emitting `SessionClosed`. It
never tells the adapter. `HarnessAdapter` (`harness.go`) has `CreateSession`,
`RunTurn`, `Steer`, `Fork`, and no `Close`; `Interrupt` exists only as an
optional type assertion in `session.go:358`.

What that costs today: `codex/codex.go:79-99` keeps the `codex app-server`
process that created or forked a thread until that thread's first turn, and
nothing can ever release it if the turn never comes (issue 108; Codex
reproduced it with a mock server this morning). Both adapters' `sessions`
maps grow for the life of the process. The Claude adapter is per-turn, so it
leaks nothing, but it also cannot be told to forget a session. A day-long
outer loop, which is the stated destination (the CEO loop over sprint
loops), multiplies each of these.

Proposed shape, one method, required:

```go
// Close releases whatever the adapter holds for the session. After Close the
// session id is invalid. Called by the runtime when the owning scope ends.
Close(ctx context.Context, sessionID string) error
```

Called from `scope.end` for each adopted session, after `closed` is set, with
a short timeout; a failure is a recording error on the run, not a panic.
Move `Interrupt` into the interface at the same time, so the harness contract
is one interface and not an interface plus a convention. This is the one
place I would add a name: the promise already exists in the docs, and the
runtime is the only party that can keep it.

### 3. `Set` returns an error it should never need to (withdrawn, see amendment)

`ceremony.txt` counts, in the two real workflows at this commit:

| Workflow | Code lines | `Set`/`SetJSON` calls | Lines of error plumbing |
| --- | --- | --- | --- |
| `internal/workflows/sprint/sprints.go` | 210 | 6 | 36 (17%) |
| Codex's live `main.go` | 193 | 16 | 58 (30%) |

Half or more of that plumbing is the three-line `if err := gimble.Set(...);
err != nil { return err }` around a value write. Read `scope.go:131-153`:
`Set` fails only for (a) no scope in ctx, (b) scope already ended, (c) key
already set in this scope, (d) `json.Marshal` of a type the constraint
already restricts to marshalable kinds. Every one of these is a programming
error, and `API.md` says they should be "loud". In Go the loud form of a
programmer error is a panic: `sync.WaitGroup` on a negative counter,
`context.WithCancel(nil)`, `regexp.MustCompile`, `template.Must`. None of
those return an error for misuse, and a workflow that misuses `Set` should
not be able to swallow it with `_ =` either (my own live workflow did that
inside a `Group` child without noticing; so did Codex's probe code).

Proposed: `Set` and `SetJSON` return nothing and panic on misuse, with the
scope key in the message. `Group.Go` recovers a panic in a child into the
group's error so a bad child does not take the process down, which is
runtime code and allowed to. The sprint workflow loses 18 lines; the
Codex-style workflow loses about 40. That is the difference between a
workflow that reads like the pseudocode in `API.md` and one where the eye
has to skip every third line. `Scope`, `Group.Wait`, `Generate`, and
`Loop.Err` keep their errors; those are real.

This is a taste call and I have put it third on purpose. Tyler's stated
criterion is that a workflow should *feel* like a DSL while being ordinary
Go, and this is the largest single source of the gap that I can measure.

## Smaller findings, agreeing with or sharpening Codex

- **Group children do not feed the planner** (`P4`: `direct=true
  nested=false`). By design (`loop.go:139` uses `localText`), and I think the
  design is right: promotion through a Go variable and a `Set` in the task
  scope is ordinary Go and keeps information flow visible. I would document
  it in `Tasks`' comment rather than change it. See the response to Codex on
  `Get`.
- **A value can forge a section** (`P5`): `render` writes a JSON string
  verbatim, so a worker result containing `## role` produces a second
  `role` heading in the next prompt. Cheap fix: indent or fence rendered
  values, or escape leading `#`. Nitpick today, larger once untrusted repos
  feed values.
- **A range-body error leaves the task scope looking clean** (`P6`:
  `task_scope_ended.error=""`, `loop_scope_ended.error=""`, run error set).
  Range-over-func gives `Loop` only a `bool` back, so this is structural;
  the page should treat an empty `scope_ended.error` as "ended", not
  "succeeded", and the task's recorded values are the verdict.
- **`TurnEnded` says success before `Generate` validates** (`session.go:193`
  writes `TurnEnded{Result: raw}` inside `turn`; `generate` at
  `session.go:74-86` validates afterward). A schema rejection is therefore a
  `scope_ended` error with a clean `turn_ended` above it. Validate inside
  `turn`, or record the validation failure on the turn.
- **The TypeScript observer forgets cancellation** (`web/src/lib/observation/
  index.ts:85`): `run_ended` unconditionally sets `failed`/`completed`,
  while the Go store (`store.go:110-116`) keeps `cancelled`. Same rule on
  both sides, ideally generated from one place.
- **`RunSnapshot` carries no workflow state** (`snapshot.go:60-63`: run,
  sessions, invocations). `snapshot-midrun.json` from my live run confirms
  it: the scope tree, values, task, and planner decisions exist only in
  `run.jsonl`. A reload of the page cannot show what the operator most wants
  to see about a Loop. Sprint 3/4 work, but the snapshot type is where it
  starts.
- **`Task` reserves the key `task`** in every task scope (`loop.go:135`); a
  workflow `Set(ctx, "task", ...)` fails with "already set". Document or
  prefix.
- **Planner prompt is fixed English inside the library** (`planPrompt`).
  Reasonable for now, and the goal string is the knob; note it as the most
  opinionated 20 lines in the package.

## Live exercise

`live/main.go`, run from this worktree; models Codex `gpt-5.6-luna`
(planner, workers) and Claude `haiku` (independent reviewer). Fixture:
`workspace/` starts as a stub that exits 1, with `SPEC.md` describing a
semver bump CLI whose prerelease rule is stated once, in prose, as a SemVer
precedence consequence (`1.2.3-rc.1 patch` prints `1.2.3`). The external
`acceptance.py` (28 cases) is authoritative and cannot be edited by agents.
Unlike Codex's run, no requirement is injected mid-run: the adaptation
pressure, if any, comes from the acceptance program disagreeing with the
worker's reading of the spec.

**Result: one assignment, first try, 28/28, eight minutes.** The planner
(1m34s) dispatched "Implement semverbump end-to-end" with a description
that restated the prerelease rule correctly and a `Validation.Command`
that was `go test ./... && python3 <acceptance.py> <workspace>`, lifted
from the initial-acceptance evidence in the root scope. The worker (4m21s)
delivered `main.go`, `main_test.go`, `README.md`; the acceptance and the
Haiku review ran concurrently in a `Group` (1m24s for Haiku, PASS with a
requirement-by-requirement account); the planner then ended dispatch
(38s); the workflow's own final gate ran again and passed; `run_ended`
error empty, `complete` with no recording error. `live/live.txt`,
`prompts.txt`, and `project/runs/20260912-112349.semverbump/` hold it.

| Turn | Duration | Prompt bytes |
| --- | --- | --- |
| planner.1/turn.1 | 94s | 6,493 |
| task.1/worker.1/turn.1 | 261s | 6,054 |
| task.1/validation.1/review.1/reviewer.1/turn.1 | 84s | 7,003 |
| planner.1/turn.2 | 38s | 11,892 |

So this run did **not** exercise adaptation: Luna read the spec's one
prerelease sentence correctly and the acceptance program agreed. I report
that as it is. The adaptive path has live evidence elsewhere (PR 140's
retained run with a real exit-7 regression and repair, and Codex's run this
morning with a mid-loop requirement); mine adds a clean end-to-end pass
with concurrent validation and a role per scope.

What the run showed about context composition (`prompts.txt`):

- **Shadowing works per agent.** Planner saw `role = You plan`; worker saw
  `role = You implement`; reviewer, in a `Group` child, saw `role = read-only
  validator`. Each saw exactly the root values plus its own chain, in
  outermost-first order.
- **The planner's second prompt carried two roles.** Its `Scoped context`
  rendered root `role = You plan`, and its `Previous task record` rendered
  the task scope's `role = You implement`, because `localText` renders
  every key the task scope set, including keys set only to shadow a parent
  for the worker's benefit. It is labelled, and Luna was not confused, but
  it is the sharp edge of "render everything visible": a key meant to
  steer one agent reaches another. Small fix: `Tasks`' feedback could skip
  keys that shadow a parent value, or the workflow should put per-agent
  steering in a child scope of its own rather than the task scope (which is
  what I did for the reviewer, and it stayed private).
- **Prompt size grows with the chain.** 6.5KB to 11.9KB between the two
  planner turns, with the task record inlined. There is no budget on
  `ScopeText` or on a value; the supervisor path has four. Fine at this
  scale, and the workflow owns the clipping, but worth a sentence in the
  Godoc.
- **The harness brings its own context.** The worker, a Codex session,
  created `workspace/ephemeral/worklog/202609121126-semverbump.md`,
  following Tyler's global Codex instructions inside the fixture. Nothing
  in the scoped prompt asked for it. Codex's review makes the same point:
  a scope shapes the prompt, not the agent's whole world.

As an author: the workflow took about twenty minutes to write from
`go doc` alone, compiled first time, and did what it says. The three
moments I stopped and thought were all about the primitives' edges rather
than the primitives: whether a `Group` child's `Set` would reach the
planner (it does not; I promoted by hand), whether `break` on the budget
would close the worker's session (it does), and the six `if err != nil`
blocks around `Set` that carry no information.

## Response to the Codex review

Read after the above was drafted. Where we agree I say so briefly; where we
differ I say why.

**Judgment.** Agree: keep the paradigm; the strongest quality is the
division of authority.

**Codex fix 1, selective scoped reads (`Get`, projection).** Partly
disagree on priority and on remedy. The observation is correct (`P4`
reproduces `direct=true nested=false`), but the remedy Codex reaches for,
typed selective reads and projections, adds names before a workflow exists
that needs them, which `AGENTS.md` forbids and which Sprint 1 already
declined once (issue 105). Passing a value from a `Group` child to the task
scope through a Go variable *is* the ordinary-Go answer, and it keeps
information flow visible. The concrete need I can see is smaller: prompt
size is unbounded (`ScopeText` has no budget while supervisors have four),
and the fix for that is in the workflow (`commandText` already clips). I
would wait for the two-role workflow Codex proposes to exist before adding
`Get`, and I would not rank this first.

**Codex fix 2, bounded hidden work and session ownership.** Agree on both
facts, and my fixes 1 and 2 are the root causes: the unbounded repair loop
exists because there is a YAML file to repair, and the fork leak exists
because the adapter has no `Close`. Codex asks for a "visible finite policy"
for malformed planner answers; I would rather remove the file so there is
nothing to repair, and let native schema enforcement do the bounding.

**Codex fix 3, operator view and outcome truth.** Agree on every fact
(cancel parity, `TurnEnded` before validation, range-body errors), and I
confirmed each from source rather than re-running. I rank it below the two
API fixes because it is Sprint 3/4 scope by the plan and because the run log
already has the truth; the snapshot and the reducer need to carry it.

**Where Codex is stronger than I was.** The Codex live run exercised a
changing requirement mid-loop and a real backlog repair, both of which I
only probed with fakes; its fork probe against a mock app-server is direct
evidence where I cite code. Its point that scopes are not a confidentiality
boundary (the harness reads skills and memory regardless) is right and worth
a sentence in the Godoc.

**Where I think Codex missed.** The dual-channel protocol itself (fix 1
above): Codex named the "two-step protocol" as a cost but did not show that
an idiomatic YAML block scalar is fatal (P1), nor propose removing the
channel. The adapter interface gap (fix 2): Codex asked for resources to be
released without naming the missing method. The ceremony cost (fix 3):
absent from the Codex review, and it is the item that speaks most directly
to Tyler's "feel like a DSL" criterion.
