# Supervision, coaching, and steering

## Leaf purpose

This leaf routes authors, maintainers, and operators to the current supervisor
implementation, hard bounds, and scope requirements. Current Go code is the
behavioral authority; requirements are current scope instruction; worklogs are
historical evidence or validation guidance.

## Topic 1: Supervision is advisory review of a live turn

- Audience: Author workflows. Authority: current code. `WithSupervisor` attaches
  a separate session and instruction to a worker turn, including a PromiseLoop
  planner turn; each objection is sent into the worker turn as steering.
  Citation: supervise.go:L34-L45.
- Audience: Author workflows. Authority: current code. A supervisor observes
  activity since its previous look, asks for structured review, and forwards
  objections; it does not own the worker result.
  Citation: supervise.go:L101-L155.
- Audience: Build/release Gimbal. Authority: current code. Worker completion
  cancels and joins supervisors before `Generate` returns, so the worker
  foreground path remains the lifecycle owner.
  Citation: supervise.go:L115-L155.
- Retrieval hint: when asked whether coaching gates delivery, start at
  `supervise.go:L140-L155`, then inspect the planner worklog.

## Topic 2: Planner supervision and scope coaching

- Audience: Author workflows. Authority: historical decision. PromiseLoop
  accepts existing `AgentOption` values for each planner turn; supervision
  timing and non-gating behavior were deliberately retained.
  Citation: ephemeral/worklog/planner-supervision.md:L1-L2.
- Audience: Author workflows. Authority: current instruction. Coaches for
  planners and working agents should oppose over-engineering, unsolicited
  features, gold-plating, and hypothetical edge cases unless a reasonable
  actual failing unit test supports them.
  Citation: ephemeral/agent-steering-requirements.md:L14-L16.
- Audience: Build/release Gimbal. Authority: current instruction. Coaching is
  advisory; completion is fulfillment of stated requirements, not a coach gate.
  Citation: ephemeral/agent-steering-requirements.md:L14-L16.
- Audience: Use Gimbal. Authority: current code. The first look bounds the
  supervisor instruction to 8 KiB and worker task to 16 KiB; later looks retain
  the fixed supervision framing only.
  Citation: supervise.go:L91-L99.
- Retrieval hint: for “coach the planner to stay in scope,” combine
  `ephemeral/agent-steering-requirements.md:L14-L16` with
  `supervise.go:L158-L169`.
- Limitation: a planner can finish before the first supervisor interval, so
  merely attaching a coach does not prove an objection landed or changed an
  assignment.
  Citation: ephemeral/worklog/planner-supervision.md:L1-L2.

## Topic 3: Session steering versus loop steering

- Audience: Use Gimbal. Authority: current instruction. `gimbal steer` to a
  session targets that exact session and must report landed versus dropped;
  steering a loop uses queued planner steering and reports queued.
  Citation: ephemeral/agent-steering-requirements.md:L5-L9.
- Audience: Use Gimbal. Authority: current code/test. A steer during a turn can
  land, while one after the turn is dropped without error; an adapter may report
  a mid-turn steer dropped when the turn ended first.
  Citation: steer_test.go:L13-L19.
- Audience: Use Gimbal. Authority: current code/test. Lifecycle records carry
  the same landed outcome that `Session.Steer` returns, so dropped delivery is
  observable rather than converted into a failure.
  Citation: steer_test.go:L70-L109.
- Audience: Build/release Gimbal. Authority: current instruction. Reuse existing
  snapshot, SSE routes, and session steering; do not add JSON-RPC, a watch
  protocol, a new state model, or a daemon.
  Citation: ephemeral/agent-steering-requirements.md:L9-L12.
- Retrieval hint: for CLI control behavior, read
  `ephemeral/agent-steering-requirements.md:L5-L14`, then confirm delivery
  semantics in `steer_test.go:L20-L109`.

## Topic 4: What a supervisor can actually see

- Audience: Build/release Gimbal. Authority: current code. The private recent
  transcript retains at most 256 KiB, while one rendered look is independently
  capped at 64 KiB and each event's stored text at 2,000 bytes.
  Citation: supervise.go:L91-L99.
- Audience: Build/release Gimbal. Authority: current code/test. Message and tool
  fragments coalesce only while unread and preserve identity; after consumption,
  later fragments form a new incremental entry.
  Citation: supervise.go:L222-L271; supervise_bounds_test.go:L68-L104.
- Audience: Use Gimbal. Authority: current code. If retention or look budget
  omits activity, the look states the missing interval and points to the durable
  worker session log; it does not claim completeness.
  Citation: supervise.go:L425-L491.
- Audience: Build/release Gimbal. Authority: current test. Fast readers stay
  within bounds; slow readers receive an explicit gap plus newest activity, and
  consumed entries are released.
  Citation: supervise_bounds_test.go:L10-L66.
- Retrieval hint: for memory or prompt-budget questions, begin at
  `supervise.go:L339-L465` and use the bounds tests as executable examples.

## Topic 5: Delivery proof versus behavior proof

- Audience: Use Gimbal. Authority: current instruction. The steering increment’s acceptance test requires a
  separate CLI process to discover a real workflow, observe and steer its active
  session, and see changed behavior; idle delivery must also be verified.
  Citation: ephemeral/agent-steering-requirements.md:L14-L14.
- Audience: Author workflows. Authority: historical observation. A landed scope
  correction is evidence only when the worker incorporates it; attachment or a
  coach response alone is insufficient.
  Citation: ephemeral/worklog/202609111647-supervisor-bounds.md:L4-L4.
- Audience: Build/release Gimbal. Authority: current instruction. Repeatable
  checks belong in package tests; do not commit proof programs, run artifacts,
  or screenshots, and report live observations in chat or a PR.
  Citation: ephemeral/agent-steering-requirements.md:L14-L14.
- Retrieval hint: search “landed,” “changed behavior,” and “planner” together
  when evaluating a supervision claim; separate transport evidence from the
  worker's resulting behavior.

## Topic 6: Operational limits and non-goals

- Audience: Build/release Gimbal. Authority: historical validation. Calibration
  reached a 262,086-byte retained peak, a 62,325-byte synthetic look under the
  65,536-byte cap, and an explicitly surfaced live gap of items 1..372.
  Citation: ephemeral/worklog/202609111647-supervisor-bounds.md:L4-L4.
- Audience: Use Gimbal. Authority: current instruction. Local control is scoped
  to this machine and supports headless CLI control; this does not establish
  a frontend-free binary build. Remote access, cancellation/quit,
  provider changes, and broad frontend reorganization are outside this increment.
  Citation: ephemeral/agent-steering-requirements.md:L9-L12.
- Gap: the assigned corpus specifies loop steering and planner behavior but
  does not include the PromiseLoop implementation; retrieve the planning
  segment before making claims about loop internals.
- Gap: the worklogs report live model observations, but they are historical
  evidence; repeat live proof before asserting current runtime behavior.

## Contradictions and retrieval cautions

- The word “supervisor” can mean a review session, a planner coach, or an
  operator steering a session. Resolve the role from the requested audience and
  use the session-versus-loop topic before proposing API changes.
- A test can establish delivery semantics and bounds, while only live evidence
  establishes that an objection changed work. Do not promote either evidence
  type into the other claim.
- The requirements prohibit hypothetical edge fixes without a failing test;
  treat an uncovered concern as a gap or proposal, not as an implementation
  requirement. Citation: ephemeral/agent-steering-requirements.md:L14-L16.
