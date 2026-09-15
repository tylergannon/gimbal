# Graph contract

The trim. Every field is justified below by one test: **it serves one of the
three purposes, or it lines a node up against the runtime's records.** The
three purposes are showing the shape of a workflow before it runs, showing how
the context gets built up, and showing what the prompts look like.

`graph_test.go` builds the graph of `internal/workflows/sprint/sprints.go` by
hand and proves it round-trips and still reads as the sprint.

## What was deleted

`Site`, `Expression`, `Exit`/`ExitKind`, `Sequence`, `Subgraph`, `Supervision`,
`SetJSON`, `Graph.Package`/`Entry`/`Input`/`Generator`, `Session.Role`/
`OwnerScope`, `Source.Column`. 251 lines to 165.

The largest deletion is the identity scheme. `scope.next(name)` returns
`path.Join(s.key, fmt.Sprintf("%s.%d", name, ordinal))`, and `scope.adopt`
uses the same `next` off the same ordinals map, so a session id is its
creating scope's key plus `/name.ordinal` and a turn id is `session.id +
"/turn.N"`. **The join is therefore: strip the trailing `.N` from every
slash-separated segment.** The graph's coder is at `round/sprint/task/coder`;
a run's is at `round.2/sprint.1/task.3/coder.1`. No node needs an id field.

## Why each field stays

| Field | Why |
| --- | --- |
| `Graph.Name` | The run's name. Joins the graph to its run row. |
| `Graph.Source` | A reader goes and looks at the entry function. |
| `Graph.Body` | The shape. |
| `Graph.Diagnostics` | Honest holes. Kept by instruction, not by the test. |
| `Source.File`, `.Line` | Makes every node checkable against its source. |
| `Session.Name` | The runtime names the session with this plus an ordinal. |
| `Session.From` | A fork inherits the conversation, so this is context. |
| `AgentCall.Session` | Which conversation speaks. Two calls on one name are two turns on one conversation. |
| `AgentCall.Prompt` | Purpose three, directly. |
| `AgentCall.Output` | The type the turn returns, which the run records per turn. |
| `AgentCall.Supervisors` | Supervision, kept out of the ordered body. |
| `Supervisor.Session` | Which session watches. |
| `Supervisor.Instruction` | A prompt. Purpose three. |
| `Supervisor.Supervisors` | Nested supervision watches its supervisor's look turns. |
| `Command.Text` | Part of the shape. See the open question below. |
| `Set.Key` | Purpose two, exactly. Body order is the order `ScopeText` reads back. |
| `Scope.Name`, `Loop.Name`, `Group.Name`, `GroupChild.Name` | Each is a runtime scope name: the join. |
| `Scope.Body`, `Loop.Body`, `GroupChild.Body`, `Branch.Body`, `Group.Children` | The nesting. |
| `Loop.Planner` | Which session decides the tasks. |
| `Loop.Goal` | The planner's standing instruction. A prompt. |
| `Condition.Branches`, `Branch.Case` | See the open question below. |
| `Diagnostic.Source`, `.Message` | Names what could not be read. |

## Open questions, with a recommendation

**1. Are commands nodes?** Recommend keeping `Command`. The sprint is not
recognisable without its checks and its commit. But it is the one operation
with no counterpart in a run, and today every command in the sprint is reached
through an `os/exec` helper the extractor has no honest way to recognise. The
graph is only truthful here once the approved command primitive exists and the
`os/exec` sites migrate to it. Until then `Command` is a promise, not a
capability.

**2. Do branches appear at all?** Recommend keeping `Condition`, but barely,
and with two rules. Record a branch only when its body contains an operation.
Drop a condition whose branches differ only in run parameters: `Sprint`'s
`if in.DryRun` calls the same `run` either way and only picks harnesses, and
treating it as shape doubles the entire workflow. In the sprint every
surviving condition guards a single context write, which the run will show as
present or absent anyway. **If you want one more thing cut, this is the
candidate.** The argument against cutting it is that an unconditional `Set`
node would claim a key is always written when it is not.

**3. How much of a prompt before `text/template`?** Recommend what the test
fixture does: the composed text with constants folded in, and every run-time
part left as a `{{ hole }}` naming the expression behind it. Nothing parses
it. Fold at the leaves of the `+` tree, never at the root, or
`researchPrompt + "\n\n"` collapses into one anonymous blob and the constant's
name is lost. When prompts become `text/template`, this field becomes the
template source and the holes become real actions. The type does not change.

## Two judgments baked into the fixture

Plain Go `for` loops are not nodes. `for round` and `for _, check := range
checks` both repeat operations, and the runtime already numbers repeats:
the second round is `round.2`. A node meaning "this happens more than once"
would say nothing the run does not say better.

`runTask` is reached from an `if` **init** statement at sprints.go:105, as is
the researcher's turn at :81. A walker that only looks at statement shapes
drops both silently. Follow a helper when it transitively contains a Gimble
operation, which `gimblelint` already computes.

## Serialization

No JSON Schema. A schema is what lets a type be handed to a model as
structured output, and a `Graph` is only ever serialized for the page. JSON
Schema is also the one polytype output that cannot express this model: asked
for one, generation reports a circular dependency at `Supervisor`, and again
at `Operation` when the first is removed.

`codegen.GoJSON()` selects Go codecs without a schema, and on polytype v1.0.1
it generates all five owner codecs and the union dispatch for the recursive
model. polytype v1.0.0 cannot: it fails to resolve `Supervisor` at all. Both
facts were needed. **The handwritten `codec.go` is deleted.**
