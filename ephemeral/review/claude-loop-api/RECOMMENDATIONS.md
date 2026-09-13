# Gimble Loop: joint recommendations — Claude and Codex, 2026-09-12

Target: merged PR 140 at `2a52971`. Two independent reviews (Codex:
`/Users/tyler/.codex/worktrees/loop-api-review/gimble/ephemeral/review/loop-api/REVIEW.md`;
Claude: `REVIEW.md` beside this file), then two rounds of exchange in
`rounds/`, logged in `DISCUSSION.md`. Every item below was marked *agree* by
both parties in round 2, with Codex's amendments incorporated verbatim in
substance. No dissent remains for Tyler to adjudicate; the open design
choices are stated as decisions, not options.

**Shared verdict.** Keep the paradigm. `Run`, `Scope`, `Group`, `Loop`, and
`Session.Generate` compose like ordinary Go; PR 140's `Task` and its
separation of dispatch, validation, and fulfillment are right. Both live runs
today (Codex: merge-queue CLI with a mid-loop requirement change, 2 tasks,
31/31; Claude: semver CLI, 1 task, 28/28) ended with the planner stopping,
an independent gate passing, and a complete durable record.

## Priority fixes

### 1. One planner channel

Replace the protocol in `loop.go:89-122` (planner edits a YAML backlog file
*and* returns a JSON `Task` that must be byte-equal to a backlog entry) with
one structured answer:

```go
type plan struct {
	Tasks []Task                 `json:"tasks"` // the revised backlog
	Next  polytype.Nullable[int] `json:"next"`  // index into Tasks, or null to end dispatch
}
```

The runtime keeps the goal (so it cannot be changed), validates the plan
once (index in range, no duplicate names, no blank required fields), records
the selected `Task` in `PlannerDecision` and the task scope's `ScopeBegan`,
and writes `backlog.md` itself for humans and the page. **A structurally
invalid plan ends the Loop with a descriptive error.** No repair turn: a
retry is ordinary Go (call `Loop` again; create the planner session in the
enclosing scope so its conversation survives).

Why: a valid backlog written with a YAML block scalar (`description: |`)
ends the whole Loop today because `|` keeps a trailing newline the JSON
answer lacks (Claude probe P1); one trailing space does the same (P3); an
unparseable backlog is retried without bound (P2, and Codex's probe: 9
planner calls against a 1-task budget). Asking for the same fact twice is the
root cause. Native schema enforcement on structured output does most of the
bounding; the planner no longer needs to write outside its workdir; an
adapter that can only return JSON can plan. Deletes about 60 lines.

### 2. `HarnessAdapter.Close`, required and idempotent

```go
// Close releases whatever the adapter holds for the session. Idempotent.
// Called by the runtime when the owning scope ends.
Close(ctx context.Context, sessionID string) error
```

Called from `scope.end` after the session is marked closed (so no `Generate`
can enter), only when a native id exists, with a short cleanup context that
survives cancellation of the run. Every failure is recorded as
`SessionClosed{Error}` (new field on the existing event). `Run` joins an
**aggregate typed cleanup error** into its returned error, distinct from
both the body's error and `Complete.RecordingError`, so a caller can tell
failed work from leaked processes from a bad log. **A close failure never
enters the scope's error**: scope errors drive control flow (`Group` cancels
siblings, `Loop` ends, the sprint withholds a commit), and cleanup happens
after the body has established its result. `RunEnded` must carry the
complete verdict, or the event sequence must define a later terminal record
that does.

Why: the Godoc promises every agent process a definite end, and the runtime
has no way to keep it (`scope.go:86-100` only flips a flag; `harness.go` has
no release method). Codex reproduced the leak with the real adapter against
a mock app-server: an unused fork's process outlives `Run` (issue 108).
Day-long outer loops multiply this.

### 3. Observation tells the truth

Three correctness defects in the runtime's public account, to fix
independently of later UI work:

- Typed-output validation must affect the turn's recorded outcome. Today
  `turn` writes `TurnEnded{Result}` (`session.go:193`) before `generate`
  validates (`session.go:74-86`), so a schema rejection is a clean turn under
  a failed scope.
- Cancellation must survive reduction in the browser. `index.ts:85` sets
  `failed`/`completed` on `run_ended` unconditionally; the Go store
  (`store.go:110-116`) keeps `cancelled`. One rule, both sides.
- `RunSnapshot` (`snapshot.go:60-63`) must carry scopes, values, tasks, and
  planner decisions so a page reload shows what `run.jsonl` already knows.

## Smaller fixes, agreed

4. **Rendered values cannot forge sections.** `ScopeText` and `localText`
   render a JSON string verbatim, so a worker result containing `## role`
   becomes a peer heading (probe P5). Use an escaped or length-delimited
   representation in both renderers.
5. **`Interrupt` into the interface.** `Session.Interrupt` is public while
   the adapter method is a type assertion (`session.go:358`). Make it a
   required capability, with its own contract and proof, as a separate change.
6. **Godoc on `ScopeText`.** State that it renders workflow-stored scope
   values and excludes the harness's own instructions, session history,
   memory, and tools; and that `Loop` adds its own planner instructions
   (`planPrompt`). Both live runs showed harness context acting (Claude's
   worker wrote a worklog nobody asked for).

## Deferred, on purpose

7. **`Get` and selective projection.** Not until a workflow needs two
   different projections of the same stored evidence and cannot stay simple
   with Go variables and scope topology. The concrete experiment: a
   planner/worker/reviewer workflow where task input (a worker's `role`) and
   planner feedback are separated explicitly. Claude's live run shows the
   pressure (the planner's second prompt carried the worker's `role`;
   prompts grew 6.5 to 11.9 KB); skipping shadowing keys in `Loop`'s feedback
   was proposed and **rejected**, because key collision cannot distinguish a
   worker-only instruction from an intentional revision the planner needs.
8. **`Set`'s error return.** Panicking on misuse was proposed and
   **withdrawn**: `math.NaN()` is a real marshal failure, and the runtime has
   no panic boundary (a panic would leave a run without `RunEnded` and
   `Complete`). The measured `Set`-specific ceremony is recorded as a known
   cost: 18 of 210 code lines in `sprints.go` (9%) and 42 of 193 in Codex's
   live workflow (22%).

## What each review contributed

Codex: the live changing-requirement run, the fork-leak reproduction with a
mock app-server, the cancel-parity and `TurnEnded`-before-validation
defects, the snapshot gap, and the discipline on failure domains. Claude:
the block-scalar fatality and the dual-channel diagnosis, the missing
`Close` as the mechanism behind the ownership gap, the section-forging
probe, the prompt-composition trace per role, and the ceremony measurement.
Each reviewer caught overstatements in the other; the corrections are in
`rounds/`.
