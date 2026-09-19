# Planning, finite iteration, and adaptive dispatch

## Source coverage and authority

- `loop.go:L15-L33` is **current code**: the public `Task` contract has a
  name, outcome description, assignment-level definition of done, and optional
  command/query validation.
- `loop.go:L67-L88` is **current code/instruction**: `PromiseLoop` owns a
  revisable planner backlog; `Tasks` yields child scopes and ending dispatch is
  distinct from validation and goal fulfillment.
- `loop.go:L198-L203` is **current code**: `Err` reports loop/harness,
  persistence, malformed-plan, or cancellation errors; failed task validation
  is workflow feedback.
- `loop.go:L254-L279` is **current code/instruction**: planner selection,
  deterministic-result precedence, role boundary, context projection, steering,
  and `next: null` semantics are encoded in the prompt.
- `iterate.go:L8-L29` is **current code**: `Iterate` preserves input order,
  creates a child scope per item, closes it before the next item, and stops on
  cancellation or a false yield result.
- `ephemeral/research/api/LOOP.md:L11-L34` is **design**: promises define
  success, dispatch chooses work, and overall assessment decides whether to stop;
  promises are direction, not a task queue.
- `ephemeral/research/api/LOOP.md:L36-L61` is **design**: assignments state
  desired outcomes and verified non-obvious facts, leaving approach to workers.
- `ephemeral/research/api/LOOP.md:L63-L97` is **design/current contract**:
  task fields, validation as a request rather than a result, and ordinary Go
  for counts and limits.
- `ephemeral/research/api/LOOP.md:L99-L126` is **design**: scoped claims need
  evidence, the original goal remains authoritative, and child feedback must be
  explicitly projected to the next planner call.
- `ephemeral/research/api/LOOP.md:L140-L155` is **design**: deterministic
  validation is binary; agent readiness latitude is per promise; yielding work
  does not decide that the job is done.
- `ephemeral/research/api/LOOP.md:L157-L212` is **design/proposal grounded in
  current prompt**: choose greatest concrete gain, size for one coherent worker
  session, adapt plans, retain deferred defects, and do not treat phase headings
  or percentages as gates.
- `ephemeral/research/api/LOOP.md:L230-L280` is **design/current contract**:
  public shape, backlog/feedback projection, task scopes, and cleanup behavior.
- `ephemeral/research/api/LOOP-WORK.md:L9-L25` is **historical execution
  record**: issue #125 replaced lap counters with structured adaptive dispatch,
  workflow-owned validation, and explicit feedback.
- `ephemeral/research/api/LOOP-WORK.md:L27-L47` is **historical proof record**:
  mechanical evidence is insufficient; live evidence must show failed-check
  feedback, no-plan dispatch, plan adaptation, and independent assessment.
- `ephemeral/worklog/202609111449-issue125-loop-contract.md:L19-L22` is
  **historical decision**: preserve an editable backlog, snapshot task data,
  explicitly project local task feedback, and keep validation data separate from
  results and planner exhaustion.
- `ephemeral/worklog/202609111449-issue125-loop-contract.md:L24-L33` is
  **historical friction/finding**: strict nullable `next`, deterministic exit
  precedence, planner role limits, repair turns, unconditional task assessment,
  and the warning that planner prose cannot override recorded command failure.
- `ephemeral/worklog/20260916-scoped-loop-254.md:L2-L5` is **historical/current
  decision**: finite iteration and adaptive planner dispatch are separate APIs;
  lifecycle cleanup is shared but data flow remains explicit.

## Author workflows

Audience: workflow authors writing ordinary Go. Authority: current code plus
current design (`loop.go:L80-L88`, `ephemeral/research/api/LOOP.md:L230-L280`).

- Use `Iterate(ctx, scope, items)` for a known finite collection. It is ordered,
  bounded by the slice, and has no planner inbox, backlog, or adaptive feedback
  protocol (`iterate.go:L8-L29`; `ephemeral/worklog/20260916-scoped-loop-254.md:L2-L5`).
- Use `PromiseLoop(ctx, name, goal, planner)` when the next assignment depends
  on prior results, failures, priorities, or newly discovered dependencies
  (`loop.go:L67-L88`; `ephemeral/research/api/LOOP.md:L157-L164`).
- Range over `loop.Tasks`, perform the assignment in the yielded child context,
  and record worker results and validation evidence there before returning;
  those local values become the next planner's previous-task record
  (`loop.go:L163-L179`; `ephemeral/research/api/LOOP.md:L268-L274`).
- Keep shared constraints and promises in parent scoped context. Do not expect a
  child write to become parent data automatically (`ephemeral/research/api/LOOP.md:L122-L126`).
- Write tasks as desired outcomes plus useful verified facts. Leave method and
  obvious command checklists to the worker (`ephemeral/research/api/LOOP.md:L36-L61`).
- Task `DefinitionOfDone` describes the assignment, including a valid
  investigation or disclosed uncertainty. It does not certify the enclosing
  goal (`loop.go:L19-L26`; `ephemeral/research/api/LOOP.md:L79-L97`).
- A body return is not task success, and `next: null` is not goal fulfillment;
  the workflow must assess completion separately (`loop.go:L85-L87`; `loop.go:L278-L279`).

## Build and release

Audience: implementers and reviewers proving a workflow change. Authority:
historical execution/proof plus current validation prompt (`ephemeral/research/api/LOOP-WORK.md:L27-L52`,
`loop.go:L254-L279`).

- Treat a recorded deterministic command result as authoritative. Agent prose
  cannot turn a nonzero result into a pass; a failed required check remains
  unmet until a later recorded pass (`loop.go:L257-L260`; `ephemeral/research/api/LOOP.md:L148-L153`).
- Keep validation requests, observed results, and final assessment as separate
  records. Empty `Validation.Command` and `Query` still require assessment
  against `DefinitionOfDone` (`loop.go:L25-L32`; `ephemeral/research/api/LOOP-WORK.md:L69-L74`).
- A supplied sprint plan is optional input. Prove adaptive selection from an
  empty backlog and adaptation past a nonblocking defect when later work gives
  more gain; do not assert that selecting a task implemented it
  (`ephemeral/research/api/LOOP.md:L159-L185`; `ephemeral/research/api/LOOP-WORK.md:L35-L47`; `ephemeral/research/api/LOOP-WORK.md:L64-L67`).
- A live proof should use real sessions and independent assessment. The checked
  supervisor example uses `gpt-5.6-luna`, a 3-minute context, and validates
  attached/landed steering plus the resulting decision (`promise_loop_live_test.go:L16-L24`,
  `promise_loop_live_test.go:L29-L43`, `promise_loop_live_test.go:L51-L72`).
- Review cancellation, early break, malformed plans, backlog repair, historical
  snapshots, and session cleanup; unit compilation alone is not sufficient
  (`ephemeral/research/api/LOOP-WORK.md:L27-L47`; `ephemeral/research/api/LOOP-WORK.md:L69-L74`).

## Use Gimble

Audience: users selecting a Gimble primitive. Authority: current API/design
(`iterate.go:L8-L29`, `loop.go:L67-L88`).

- Finite iteration means “these items, in this order”; adaptive dispatch means
  “choose the next useful assignment from current evidence.” Do not model a
  planner as a disguised `for` loop (`iterate.go:L17-L29`; `ephemeral/research/api/LOOP.md:L23-L34`).
- `PromiseLoop.Err()` is the error boundary for the planner/runtime. A task's
  failed validation belongs in recorded feedback and can drive replanning
  (`loop.go:L198-L203`).
- `WrapUp` is an ordinary operator message. The planner still writes the stop
  decision, and in-flight work may finish or be dropped according to that
  decision (`loop.go:L248-L252`; `loop.go:L269-L278`).
- Overall promise fulfillment is a higher-level contract: the runtime loop only
  dispatches work. It does not provide a promise registry, recurring freshness,
  badges, budgets, or automatic CEO termination (`ephemeral/research/api/LOOP.md:L99-L104`,
  `ephemeral/research/api/LOOP.md:L128-L155`; `ephemeral/research/api/LOOP-WORK.md:L54-L55`).

## Retrieval hints

- Search `PromiseLoop`, `Tasks`, `DefinitionOfDone`, `next: null`, and
  `deterministic` together when reconstructing adaptive semantics.
- Search `Iterate` and `scoped-loop-254` for finite-iteration lifecycle semantics.
- Search `LOOP-WORK.md` and the issue-125 worklog for proof history and known
  failure modes before trusting a passing aggregate test.

## Gotchas, conflicts, and gaps

- The runtime PromiseLoop is not the higher-level promise contract: it knows a
  goal, backlog, task feedback, and planner stop, but not whether the goal's
  promises cover the requested outcome (`ephemeral/research/api/LOOP.md:L99-L115`; `loop.go:L278-L279`).
- “90–95% done” applies to validator judgment per promise, never as a numeric
  task score and never as permission to ignore a failing deterministic check
  (`ephemeral/research/api/LOOP.md:L140-L153`, `ephemeral/research/api/LOOP.md:L207-L212`).
- `LOOP.md` calls planner wording proposed, while `loop.go` is the shipped
  prompt. When they differ, teach the current code and flag the design as
  rationale (`ephemeral/research/api/LOOP.md:L171-L228`; `loop.go:L254-L279`).
- Evidence freshness/reassessment after later state changes remains open; do
  not invent a freshness registry from this API (`ephemeral/research/api/LOOP.md:L117-L120`).
