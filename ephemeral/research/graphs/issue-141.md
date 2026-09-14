# Loop API review at PR 140: Claude and Codex joint findings and recommendations

URL: https://github.com/tylergannon/gimble/issues/141
State: closed
Milestone: None
Updated: 2026-09-13T13:49:20Z

Two independent reviews of the Loop API at the PR #140 merge commit (`2a52971`), one by Codex this morning and one by Claude, followed by three rounds of exchange between the two and a joint recommendations document that both signed. This issue is the readable write-up; the artifacts are linked at the bottom.

## TL;DR

**Keep the paradigm.** `Run`, `Scope`, `Group`, `Loop`, and `Session.Generate` compose like ordinary Go. A workflow reads as a page of pseudocode. PR #140's `Task` shape and its separation of dispatch, validation, and fulfillment is the best design decision in the repository. Both live runs today ended with the planner stopping, an independent gate passing, and a complete durable record.

**Three priority fixes, agreed by both reviewers:**

1. **One planner channel.** Stop asking the planner to edit a YAML file *and* return a JSON task that must be byte-equal to a backlog entry. A valid backlog written with a YAML block scalar ends the whole Loop today.
2. **`HarnessAdapter.Close`, required.** The Godoc promises agent processes end with their scope; the adapter interface gives the runtime no way to keep that promise.
3. **Observation tells the truth.** A turn is recorded as successful before schema validation; the browser turns `cancelled` into `failed`; a page reload has no scope, task, or planner state.

Nothing is left in dissent. Two proposals were made and withdrawn during the exchange (details below), which is the point of having two reviewers.

## How this was done

- **Codex** (Desktop session, gpt-5.6 family) got Tyler's brief at 06:08: full review of the system, design principles, API, and PR #140's Loop, judged on simplicity and capacity, top three positives and top three fixes, with a non-trivial live run. It worked in a fresh detached worktree, ran all gates, wrote fake-adapter probes and a fork-leak probe against a mock app-server, and ran a 15-minute live Loop (merge-queue CLI, Luna planner/workers, Haiku validators, a requirement injected mid-loop; 2 tasks, 25/25 then 6/6).
- **Claude** (Fable 5.1) ran the same brief independently in another fresh worktree, reading Codex's review only after its own code reading and probes. Gates: `just build` ok, `go vet` clean, `go test -race` ok in 11 packages, 8 web tests pass. Six fake-adapter probes, a ceremony measurement of the two real workflows, and an 8-minute live run (semver bump CLI with a non-obvious prerelease rule; Luna planner/worker, Haiku reviewer running concurrently in a `Group`; 1 task, 28/28 first try). Also a per-agent trace of exactly which scoped values each prompt contained.
- **Exchange.** Claude resumed Codex's own reviewer session through `tractor run-prompt --session` so Codex argued with its morning context intact. Round 1: Codex responded point by point to Claude's review. Round 2: Claude conceded, held, and drafted eight recommendations; Codex marked each agree or dissent. Round 3: Codex signed the final document. Each round is its own file; nobody edited the other's artifact.

## Shared positives

1. **The control flow is the program.** `for ctx, task := range loop.Tasks` with `loop.Err()` afterward is the `bufio.Scanner` idiom applied to dispatch. `break` and `return` mean what they mean; the task scope ends when the body returns (tests prove it for break and cancellation). `Group` is `errgroup` with a name. No registry, no graph object, no named tactics. The sprint workflow's logic is about 210 lines; Claude's live workflow is under 150 and was written from `go doc` alone.
2. **Scope is one concept doing three jobs.** Identity (the key on every event), data (set-once, revision by shadowing, `ScopeText` outermost-first), and lifetime (sessions die with their scope; `Wait` is the join). Because they coincide, the durable log falls out for free. The live trace shows role shadowing working exactly as documented: planner saw "You plan", worker saw "You implement", the reviewer in a Group child saw "read-only validator", each only its own. *Caveat agreed in discussion:* this holds at Gimble's event boundary and not yet at the harness boundary, which is fix 2.
3. **PR #140 draws authority lines correctly.** `Task{Name, Description, DefinitionOfDone, Validation{Command, Query}}` travels unchanged through planner answer, yielded value, scope, backlog, and events. The planner may end dispatch but may not certify the goal; a nonzero exit outranks prose; the enclosing workflow decides when to stop. The worklog shows the precedence rule was learned from a live Luna worker claiming a probe passed when the recorded exit was 7, and it is now in the primitive.

## Priority fix 1: one planner channel

**Today** (`loop.go:89-122`): each dispatch asks the planner to edit `backlog.md` (YAML frontmatter) and return a structured `plan{Next}`, then requires the returned `Task` to be `==` to an element parsed from the file.

**Evidence** (Claude probes, fake adapter):

| Probe | Planner behavior | Result |
| --- | --- | --- |
| P1 | Valid backlog using a YAML block scalar (`description: \|`), identical task returned as JSON | `yielded=0`, Loop ends: "did not preserve that assignment" (the `\|` trailing newline) |
| P3 | One trailing space in a quoted description | Same fatal end |
| P2 | Backlog that never parses | 7 planner calls against a 1-task budget; bounded only by ctx deadline |

Codex's probe showed the same repair loop (9 calls, 0 tasks), and Codex's live run hit it for real (an unquoted colon, a 25-second repair turn). The round-02 reviewer on PR #140 had called exact equality a nitpick; P1 says a well-formed, idiomatic backlog kills the loop, so it is not.

**Proposal** (both agree):

```go
// The planner's whole answer. The planner writes nothing.
type plan struct {
	Tasks []Task                 `json:"tasks"` // the revised backlog
	Next  polytype.Nullable[int] `json:"next"`  // index into Tasks, or null to end dispatch
}
```

The runtime keeps the goal (so it cannot change), validates the plan once (index in range, no duplicate names, no blank required fields), records the selected `Task` in `PlannerDecision` and the task scope's `ScopeBegan`, and writes `backlog.md` itself for humans and the page. **A structurally invalid plan ends the Loop with a descriptive error.** No repair turn (Codex's call, Claude agreed): a retry is ordinary Go by calling `Loop` again, with the planner session created in the enclosing scope so its conversation survives.

Deletes `readBacklog`, the YAML dependency, the repair loop, the goal-immutability check, and the equality check (about 60 lines). Native schema enforcement on Codex and Claude structured output does most of the bounding. The planner no longer writes outside its workdir (today the backlog lives under `.gimble/runs/...` and only works because Codex runs with `danger-full-access`), and an adapter that can only return JSON can plan.

## Priority fix 2: `HarnessAdapter.Close`

**Today:** `scope.end` (`scope.go:86-100`) sets `session.closed = true` and emits `SessionClosed`. It never tells the adapter. `HarnessAdapter` has `CreateSession`, `RunTurn`, `Steer`, `Fork`, and no release method. `Interrupt` exists only as an optional type assertion (`session.go:358`).

**Evidence:** `codex/codex.go:79-99` keeps the `codex app-server` process that created or forked a thread until that thread's first turn; nothing can release it if the turn never comes. Codex reproduced this with the real adapter against a mock app-server: an unused fork's process was alive after `Run` returned (#108). Both adapters' session maps grow for the life of the process. A day-long outer loop (the CEO loop over sprint loops) multiplies this.

**Proposal** (both agree):

```go
// Close releases whatever the adapter holds for the session. Idempotent.
// Called by the runtime when the owning scope ends.
Close(ctx context.Context, sessionID string) error
```

Called from `scope.end` after the session is marked closed (no `Generate` can enter), only when a native id exists, with a short cleanup context that survives run cancellation. Every failure is recorded as `SessionClosed{Error}`. `Run` joins an **aggregate typed cleanup error** into its returned error, distinct from the body's error and from `Complete.RecordingError`, so a caller can tell failed work from leaked processes from a bad log. **A close failure never enters the scope's error**: scope errors drive control flow (`Group` cancels siblings, `Loop` ends, the sprint withholds a commit), and cleanup happens after the body has established its result. `RunEnded` must carry the complete verdict, or a later terminal record must.

## Priority fix 3: observation tells the truth

Three correctness defects in the runtime's public account, independent of later UI work (Codex found all three; Claude confirmed each from source):

- **Validation after success.** `turn` writes `TurnEnded{Result}` (`session.go:193`) before `generate` validates (`session.go:74-86`), so a schema rejection is a clean turn under a failed scope.
- **Cancelled becomes failed in the browser.** `web/src/lib/observation/index.ts:85` sets `failed`/`completed` on `run_ended` unconditionally; the Go store (`store.go:110-116`) keeps `cancelled`. One rule, both sides.
- **The snapshot has no workflow state.** `RunSnapshot` (`snapshot.go:60-63`) is run, sessions, invocations. Scopes, values, tasks, and planner decisions exist only in `run.jsonl`, so a page reload cannot show what the operator most wants to see about a Loop. Confirmed mid-run against Claude's live run.

## Smaller agreed fixes

4. **Rendered values cannot forge sections.** `ScopeText` and `localText` render a JSON string verbatim, so a worker result containing `## role` becomes a second `role` heading in the next prompt (probe P5). Fence, escape, or length-delimit values in both renderers.
5. **`Interrupt` into the interface.** Public on `Session`, optional on the adapter. Make it required, as a separate change with its own proof.
6. **Godoc on `ScopeText`.** It renders workflow-stored scope values and excludes the harness's own instructions, session history, memory, and tools; `Loop` adds its own planner instructions too. Both live runs showed harness context acting on its own: Claude's worker (a Codex session) created `ephemeral/worklog/...` inside the fixture following Tyler's global Codex instructions, unprompted.

## Deferred on purpose

7. **`Get` and selective projection.** Codex's first review ranked this #1 (#105 already records it). Claude pushed back: adding a name before a workflow needs it is what AGENTS.md forbids, and promoting a `Group` child's result through a Go variable is the ordinary-Go answer. Codex withdrew it as a top-three item but noted real pressure in Claude's own run: the planner's second prompt carried both the planner's `role` and the worker's `role` (the task-scope shadow is re-rendered in the previous-task record), and prompts grew from 6.5 KB to 11.9 KB. **Next proof:** a planner/worker/reviewer workflow where task input and planner feedback are separated explicitly. If it cannot stay simple with Go variables and scope topology, a projection primitive has earned its name. Claude proposed having `Loop`'s feedback skip keys that shadow a parent; Codex rejected it because key collision cannot tell a worker-only `role` from an intentional `status` revision the planner needs. Claude agreed.
8. **`Set`'s error return.** Claude proposed that `Set`/`SetJSON` panic on misuse, since their failures looked like programmer errors and the `if err != nil` around them is a visible share of every workflow. Codex showed `math.NaN()` through the `float64` arm is a real marshal failure, and that the runtime has no panic boundary (a panic would leave a run without `RunEnded` and `Complete`). Withdrawn. The measured cost is recorded: 18 of 210 code lines in `sprints.go` (9%) and 42 of 193 in Codex's live workflow (22%) are `Set` plumbing.

## Live runs, briefly

| | Codex run | Claude run |
| --- | --- | --- |
| Fixture | Dependency-aware merge-queue CLI | Semver bump CLI with a prerelease finalization rule |
| Models | Luna planner/workers, Haiku validators | Luna planner/worker, Haiku reviewer in a `Group` |
| Shape | 2 tasks; a `--parallel` requirement injected after the first pass | 1 task; no injection, the spec's one tricky sentence was the test |
| Result | 25/25 then 6/6; one YAML repair turn; planner stop; final gates pass | 28/28 first try; planner stop; final gate pass |
| Duration | ~15 min | ~8 min |
| Notable | Real backlog repair; fork-leak repro separately | Per-agent prompt trace; double `role` in planner's 2nd prompt; worker wrote an unprompted worklog |

Neither run is a production-delivery claim. Both show adaptive dispatch with an independent gate working end to end on the cheap tier.

## Artifacts

Claude's side is committed on branch `claude/loop-review-20260912`:

- [RECOMMENDATIONS.md](https://github.com/tylergannon/gimble/blob/claude/loop-review-20260912/ephemeral/review/claude-loop-api/RECOMMENDATIONS.md), the joint document Codex signed
- [DISCUSSION.md](https://github.com/tylergannon/gimble/blob/claude/loop-review-20260912/ephemeral/review/claude-loop-api/DISCUSSION.md), who said what and what moved
- [REVIEW.md](https://github.com/tylergannon/gimble/blob/claude/loop-review-20260912/ephemeral/review/claude-loop-api/REVIEW.md), Claude's review with an amendment note
- [rounds/](https://github.com/tylergannon/gimble/blob/claude/loop-review-20260912/ephemeral/review/claude-loop-api/rounds): `01-codex.md`, `02-claude.md`, `02-codex.md`, `03-codex.md`
- [probes/main.go](https://github.com/tylergannon/gimble/blob/claude/loop-review-20260912/ephemeral/review/claude-loop-api/probes/main.go) and [probes.txt](https://github.com/tylergannon/gimble/blob/claude/loop-review-20260912/ephemeral/review/claude-loop-api/probes.txt), [ceremony.txt](https://github.com/tylergannon/gimble/blob/claude/loop-review-20260912/ephemeral/review/claude-loop-api/ceremony.txt), [prompts.txt](https://github.com/tylergannon/gimble/blob/claude/loop-review-20260912/ephemeral/review/claude-loop-api/prompts.txt)
- [live/main.go](https://github.com/tylergannon/gimble/blob/claude/loop-review-20260912/ephemeral/review/claude-loop-api/live/main.go), [live/live.txt](https://github.com/tylergannon/gimble/blob/claude/loop-review-20260912/ephemeral/review/claude-loop-api/live/live.txt), and the run directory under `project/runs/`

Codex's review and evidence are uncommitted in its worktree at `/Users/tyler/.codex/worktrees/loop-api-review/gimble/ephemeral/review/loop-api/` (`REVIEW.md`, probes, fork probe, live run).

Related: #105 (Get/GetJSON/Each), #107 (Steer landed), #108 (idle app-server for unused fork), #112 (vet fails before `just build`), #125 and #140 (this Loop contract).

