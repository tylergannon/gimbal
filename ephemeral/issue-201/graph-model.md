# Workflow graph: accepted nested model

This records Tyler's latest direction for issue 201. It supersedes the earlier
flat operation/edge model in the implementation handoff and its 17-variant probe.
These are the semantic decisions to preserve across implementation and further
design work. Production types and recursive encoding are not implemented yet.

## What the graph describes

A Graph is the root description of a workflow. Conceptually, it is a root
subgraph plus workflow identity and source/build information. Use nested ordered
bodies to describe the structure connecting agent calls, command execution, and
context writes. The useful view is scoped groups of agent calls in the right
order. Go executes the workflow; this description helps a person follow the
agent work. It is not a map of the program. Additional program detail needs a
concrete use for the viewer, not merely an argument that it could be extracted.

Do not try to reproduce the entire Go program or construct a general control-flow
graph. Ordinary calculations can be omitted or retained as expressions attached
to relevant steps. Preserve conditions, loops, scopes, groups, and early exits
that affect how the relevant steps link up. Static mapping means recovery of
this selected workflow structure, not every Go statement or every data dependency.

The explicit workflow aesthetic still applies: as a rule of thumb, lint against
constructs that hide this structure and advise the author to make it explicit.
Dynamic dispatch and dynamic Set keys are authoring violations. Runtime branch
outcomes, loop counts, task contents, prompts, and command arguments may vary.
Those values do not require deeper analysis when the structural relationships
remain visible. Unsupported relevant structure is diagnostic partial output and
a failing gate; omitted irrelevant Go statements are not incomplete coverage.

## Ordered bodies and subgraphs

Retain Operation as a sealed interface for the alternatives an ordered body can
contain. Keep these meanings rather than the previous low-level variant list:

| Element | Meaning |
| --- | --- |
| AgentCall | One call using an agent session; multiple calls may use the same conversation. |
| Command | A call to the approved Gimble command execution function. |
| Set | A context write using Gimble’s supported primitive value types, at its source position. |
| SetJSON | A structured context write at its source position. |
| Subgraph | One of the enclosing constructs below. |
| Exit | A return, break, or continue that changes which relevant work can follow. |

Subgraph has these concrete forms:

| Kind | Contents and semantics |
| --- | --- |
| Sequence | An ordered body. This is the natural root body. |
| Condition | Alternative branches, each with a condition/case and an ordered body. |
| Loop | A repeated body with its loop condition or planner information. |
| Scope | A named body establishing a Gimble context/lifetime boundary. |
| Group | Grouped concurrent agent work, with order retained within each child body. |

Each branch and each group child has an ordered body. The order of alternative
branches is not a sequence of executions. The order of group children is not a
dependency between their completions. Reserve the name Group for Gimble's actual
parallel execution primitive, not for an unordered collection of supervisors.

Sequence relationships follow from body order. Conditions describe alternatives;
loops describe repeated bodies. Runtime supplies branch choices, iteration counts,
and task contents. Group membership describes concurrent work without implying
that siblings complete sequentially. Preserve the order of relevant calls within
their bodies; do not reconstruct a launch/join timeline or unrelated caller work.

The earlier requirement to preserve every Group.Go/Group.Wait position and
os/exec Start/Wait relationship was still modeling too much of the program. It
is withdrawn. The graph does not need universal ports, synthetic entry/exit/merge
nodes, a separate look-operation vocabulary, or a general control-flow edge map.

Operation and Subgraph should use sealed interfaces with concrete struct
variants. Concrete subgraph variants can implement both markers directly, so
they can occur in ordered bodies. An interface itself is not a concrete union
member. Keep polytype's inferred union membership; do not maintain an independent
Kind plus nullable payload scheme or a second handwritten JSON model.

## Agent sessions and commands

AgentCall is a call, not a session. For example, two Generate calls using
researcher are two steps referencing one conversation. Session declarations retain
identity, name, owning scope, model/adapter/workdir, source, and fork origin.
Creating/forking a session does not require a separate visible operation node.

### Approved command execution

Provide one opinionated Gimble command execution function so workflow command
calls can always be recognized and captured at that boundary. Lint against
direct os/exec use in workflow authoring code and advise use of that function.
This authoring rule does not prohibit the command implementation or harness
internals from using os/exec.

A Command element represents a call to this function in its scope and source
order. The extractor recognizes that call; it does not analyze exec.Cmd
construction, mutations, aliases, Start/Wait lifecycles, or arbitrary subprocess
code. Command results are secondary detail, not a reason to expand the graph into
a process trace. Their exact capture and presentation are not settled here.

The function's public name, signature, and result contract remain to be designed.
This records the requested API direction and lint rule; it does not implement
that function or claim existing direct os/exec calls already comply. Migrate
workflow command sites as part of adopting the approved API. Keep this primitive
small and direct; it does not authorize high-level workflow tactic wrappers.

## Context evolution

Keep Set and SetJSON in their source positions. This permits inspection of the
writes preceding a call and the writes associated with each alternative.
Preserve the key and value expression; runtime observations supply actual values
and the branch taken. Do not attempt arbitrary Go dataflow evaluation or assume
a stored value was included in a prompt.

A Condition does not establish a new Gimble scope. Branches use the context
supplied by their enclosing scope. A Scope or a planner task scope establishes
the relevant ownership boundary. An ordinary Go loop does not create a new scope
merely because it has a body; preserve the actual Loop.Tasks scope semantics.

## Supervision is a separate hierarchy

A scope has an ordered execution body and a separate collection of supervision
attachments. Each attachment is rooted at a watched call. Its supervisors are
unordered, and each supervisor may itself have supervisors.

```text
Scope: task

  Sequence:
    1. Coder call
    2. Validation command
    3. Validator call
    4. SetJSON: assessment

  Supervision:
    Coder call
      ├─ Taste reviewer
      │    └─ Lead reviewer
      └─ Scope reviewer
```

Taste reviewer and Scope reviewer both watch the coder call; neither precedes
the other. Lead reviewer watches Taste reviewer's look turns. No supervisor is
a sequential prerequisite or an approval gate for the validation command.
An attachment can exist without any look occurring; a look need not produce a
steer, and a steer may land or be dropped.

The agreed semantic shape is:

```go
type Supervision struct {
	Target      string       // Watched call's static ID.
	Supervisors []Supervisor // Unordered.
}

type Supervisor struct {
	Session     string
	Instruction Expression
	Interval    Expression
	Supervisors []Supervisor // Watch this supervisor's look turns.
}
```

Nesting supplies the target for higher supervisors. Do not manufacture separate
look-operation nodes merely to connect nested attachments. Actual look turns and
steers remain runtime observations. Preserve source/static attachment identity
for inspection without conflating it with runtime turn identity.

## Ownership, execution, and placement

**A call's supervision tree belongs beside that call. Each supervisor references
a session whose owning scope determines its lifetime.**

- Session ownership determines how long the conversation survives.
- The attachment identifies the call being watched and its execution scope.
- The visualization places that supervision tree alongside the watched call.
- A nested supervisor watches the immediate supervisor's looks.

```text
Run
  Owns: reviewer session

  Task 1
    Sequence:     coder → checks → validator
    Supervision:  reviewer watches coder

  Task 2
    Sequence:     coder → checks → validator
    Supervision:  reviewer watches coder
```

Both reviewer appearances refer to the same session. It retains conversation
history across the sequential tasks while appearing locally in each task's
supervision tree. An ancestor-owned supervisor can watch a call in a deeper
nested scope. This does not relocate, clone, or transfer ownership of its session.

The default view shows the attachment locally. The inspector can show "session
owned by Run" and link to its complete conversation. Long cross-scope ownership
connectors are unnecessary in the default view; retain the relationship in the
data so it remains inspectable. Repeated visual appearances do not duplicate
session identity, recorded turns, or usage accounting.

An author can instead create the supervisor session inside the task scope. Its
ownership and execution are then local, and each task gets a fresh conversation.
Support both through the existing session-creation rules. Authors choose the
conversation lifetime; placement consistently follows the attachment.

The inspected runtime already derives supervisor look contexts from the watched
call's context and records those turns in that execution scope. Nested supervision
uses the same mechanism. See:

- `/Users/tyler/.codex/worktrees/a256/gimble/supervise.go`, lookCtx and the supervisor Generate call.
- `/Users/tyler/.codex/worktrees/a256/gimble/session.go`, NewSession ownership and turn recording through current(ctx).

This is source evidence, not a new live attestation. A session accepts one turn
at a time: simultaneous looks in parallel tasks need separate sessions or
explicitly scheduled use. Co-locating visual appearances does not add concurrency
to a shared conversation. No automatic scheduler is part of this decision.

## Illustrative workflow shape

```text
Graph: Sprint
└─ Sequence
   ├─ SetJSON: input
   ├─ AgentCall: researcher
   ├─ Loop: rounds
   │  └─ Scope: round
   │     └─ Loop: planner-selected tasks
   │        └─ Scope: task
   │           ├─ AgentCall: coder
   │           ├─ Set: worker result
   │           ├─ Condition: task has a validation command
   │           │  └─ Command: task validation
   │           ├─ AgentCall: validator
   │           └─ SetJSON: assessment
   └─ AgentCall: final summary
```

This illustrates the model; it is not a complete extraction of the current
sprint. Supervision is attached to the calls as described above, separately
from their ordered bodies.

## Encoding decision still open

The natural model is recursive in two places:

1. Subgraph → ordered body → Subgraph.
2. Supervisor → Supervisors → Supervisor.

Pinned polytype v1.0.0 rejects recursive type definitions. Its support for sealed
unions does not itself make either recursive structure serializable. Two possible
implementation paths remain:

- Add recursive-type support to polytype.
- Represent child relationships by static references in the serialized Go model.

Keep the conceptual model nested either way. References would encode child
membership/order and supervisor parentage; they do not require restoring the old
general control-flow graph. Go remains the canonical type, with projection through
polytype/skgo rather than a separately designed JSON/TypeScript contract.

The encoding choice has not been made. This codification does not authorize a
polytype implementation change or settle the final Graph field layout. Finalize
that choice and the command API contract before handing Sol an exact
production struct to implement. Do not silently ship the superseded flat struct
as a workaround. The earlier 17-variant projection probe proves only that earlier
nonrecursive model's codecs; it is not proof for this model.

## Acceptance consequences

- Inspect scoped, ordered agent work in sequence, condition, loop, scope, and
  group examples. Do not require a full program map or launch/join timeline.
- Capture calls to the approved command function in source order and reject direct
  os/exec use in workflow code with an actionable lint diagnostic.
- Preserve Set/SetJSON positions and actual scope ownership across branches.
- Inspect an ancestor-owned supervisor beside a deeper child call, with a nested
  supervisor targeting the first supervisor's looks.
- Show one shared reviewer across sequential tasks retaining one session identity;
  a locally created reviewer gets a new session in each task instead.
- A worker finishing before the supervisor's first look remains a valid run.
- Show attachment-local placement without duplicating observed turns or usage.
- Prove the chosen recursive/reference representation through actual generated
  JSON and devalue codecs and TypeScript projection before claiming it works.
