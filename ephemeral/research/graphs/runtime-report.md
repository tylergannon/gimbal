# Gimble graph/runtime feasibility report

## Finding

A useful hierarchical graph is feasible from the current API because scopes
already provide named boundaries, ordinal runtime instance keys, and explicit
structured concurrency. The runtime can reconstruct containment and most
execution placement from `run.jsonl`; sessions, turns, loops, groups, and
supervisor attachments have enough facts to form typed nodes and relations.
The current observation reducer and page do not expose the full graph: they
retain six tables, native transcript frames, and flat scope/turn rendering.

The complete line-numbered source map is in
[runtime-source-evidence.md](runtime-source-evidence.md). The short routing
leaf is [runtime-index-leaf.md](runtime-index-leaf.md).

## Runtime model

Use a hierarchy whose primary edge is `contains`: run/root, explicit scope,
group, loop, task, session, turn, and command operation. Scope keys such as
`lap.3/bakeoff.1/attempt.2` are runtime instance paths. `scope.go:29-65` shows
how parent pointers and ordinals produce them. A path prefix identifies
containment, while `SessionRow.Scope` and `TurnRow.Scope` intentionally preserve
session creation scope separately from turn execution scope
(`internal/observation/rows.go:50-93`).

Typed non-tree edges should remain distinct:

- `owns-session`: creator scope to session, from `SessionCreated` and
  `SessionRow.Scope`; `uses-session` is reserved for a call/turn to the
  session it invokes, which is a separate relationship.
- `forked-from`: child session to `SessionCreated.Parent`.
- `supervises`: reviewer session to a worker turn, from
  `SuperviseAttached.Reviewer` and `.Worker`.
- `steers`: source to target with landed/dropped status from `Steer`.
- `killed`: actor/operation to target scope or turn with reason from `Killed`.
- `model-call`: turn to model-call facts from native step events.

`events.go:87-161` defines the relevant relation payloads. The API design
confirms that fork, supervision, and steering are the non-prefix relations
(`ephemeral/research/api/API.md:779-819`). A supervisor is a regular session
with a role and an attachment, and its periodic looks are ordinary turns;
nested supervision is therefore representable without inventing a new agent
kind (`supervise.go:75-129`). It is a control overlay, never an approval or
completion dependency.

Groups and loops carry useful runtime semantics. Group children are named child
scopes, and `Wait` joins all of them (`group.go:19-98`); overlapping began/end
intervals are evidence of concurrency. Loop planner decisions and task child
scopes are emitted with structured task data (`loop.go:82-164`). These facts
support runtime expansion of repeated instances while leaving dynamic task
counts to the observed run.

## Static templates versus runtime instances

Keep two related layers:

1. A static template records possible nodes/regions, declared names, source
   locations, branch/loop repetition, and typed possible edges. It can show a
   workflow before execution, but it cannot claim that a node ran or predict
   dynamic task count, command arguments, timing, or output.
2. A runtime graph records actual ordinal instances, statuses, intervals,
   values, turns, transcripts, command results, and control events. Its source
   link is a best-effort mapping to a template site; ambiguity must stay
   visible.

The design record describes this split and a future SSA pass
(`API.md:720-732`, `API.md:856-870`). Go's official `analysis` package and
`ssa` package documentation were downloaded as
[runtime-go-analysis.html](runtime-go-analysis.html) and
[runtime-go-ssa.html](runtime-go-ssa.html). They support typed analyzer passes,
dependency results, package syntax/types, and SSA control-flow inspection.
They do not provide runtime identity, command timing, dynamic argument values,
or a universal source-to-SSA mapping.

The intended join key is the path of names with ordinals removed
(`API.md:747-759`, `API.md:779-797`). This has two material gaps. First, two
distinct call sites under the same parent may deliberately use the same name;
the runtime then produces repeated ordinals indistinguishable from repeated
executions after ordinals are stripped. Second, `Generate` has no name argument;
multiple Generate sites on one session share the session's turn ordinal family,
and re-asks/implicit planner/supervisor turns make ordinal matching unreliable.
Do not claim exact source lighting in either case. An explicitly named Scope
around an operation is the smallest current API mechanism for giving important
operations a separate runtime region.

The static pass should follow context parameters and lexical captures through
supported helpers, resolve Gimble calls by symbols, and represent conditionals
as possible paths. The design explicitly limits context flow to parameters or
captures, known `context.With*` and Gimble calls, and reports `Background` chain
breaks/interface alternatives (`API.md:856-870`). It should preserve ordinary
Go, use AST source regions plus SSA where needed, and never use reflection or
`runtime.Caller` (`scope.go:52-65`; `API.md:754-759`).

## Commands and observability

The built-in sprint runs validation through ordinary `exec.CommandContext`
(`internal/workflows/sprint/sprints.go:209-310`) and records a text summary in
scope values. The runtime has no command lifecycle event, process start/end
timestamps, separate stderr, or command node. Static extraction can locate
`exec.CommandContext`, but a manifest cannot make an unmodified binary emit
execution facts. The only current command outcome is workflow-owned: `command`
returns an integer exit code and combined output to its caller, which then
stores a formatted `$ command`, `exit N`, and output string in a scope value;
that is not a runtime command row or event.

The minimal no-wrapper experiment is a named Scope containing the direct
`os/exec` call and explicit `Set` of exit/output. This yields a real scope
interval and inspectable result while preserving ordinary Go. Its duration is
the surrounding scope duration, and scope error need not equal process exit
code; it must not be presented as exact process telemetry. If exact Start/Wait
spans, process timing, stderr references, or live command state become required,
that is a separate runtime observation boundary/API decision. Agent-internal
commands remain native transcript events and are a different observation layer.

## Current persistence and UI limits

The design schedule matches this gap: `ephemeral/research/api/SPRINTS.md:96-127`
settles the durable event vocabulary and prefix-tree proof, while
`SPRINTS.md:129-170` schedules the graph/timeline and explicit relation UI in
the web sprints. `docs/web-app.md:80-100` records the current product boundary
more bluntly: F3 is still a flat list, F10 edges are designed but unbuilt, and
F13 static templates are later. The first runtime graph slice should therefore
be judged against actual observed scope/turn intervals and replay, rather than
against a static template that does not exist yet.

`internal/observation/store.go:203-280` reduces core run/scope/value/planner/
session/turn lifecycle events. It has no cases for `session_closed`,
`supervise_attached`, `steer`, `killed`, or `complete`, though these remain in
the durable log. `internal/observation/http.go:17-120` streams only a complete
snapshot and ordered row/totals/event suffix. `web/src/lib/observation/index.ts:10-203`
mirrors those rows and `web/src/lib/observation/RunViewer.svelte:62-87` renders
flat scope sections followed by flat turn invocations. `SessionTimeline.svelte`
is a native transcript view (`SessionTimeline.svelte:7-44`), not workflow
topology. A graph UI therefore needs an edge model/reducer or a deliberate
log-derived relation projection before it can show supervision, fork, steer,
kill, or overlap as first-class links.

`ScopeText` is also not a recorded read fact. `scope.go:184-201` computes the
nearest visible values from context; `ValueSet` records writes only. A graph
may show values available in an enclosing scope, but must not assert that a
specific prompt consumed one unless the workflow's explicit prompt construction
or a future read event proves it.

## Verification and proof boundary

The first `go test ./...` passed for all non-web packages, while the web
package failed because `skgo.manifest.json` was absent. `just build` then
completed web dependency installation, generation, Svelte build, and
`go build -o bin/gimble ./cmd`; a second `go test ./...` passed for every
package, and `go vet ./...` passed. This report is research only; no production
implementation or graph analyzer was added.

The repository's definition of done requires behavior to be seen working and
an agent to check that tests/commands demonstrate it (`docs/definition-of-done.md:7-30`);
that is why the build/test distinction is recorded here. The same document
keeps supervisor opinion out of the gate (`docs/definition-of-done.md:41-54`).

No screenshot attachment was present in the received task. The visualization
assessment used current repository code and the design docs, especially
`web/src/lib/observation/RunViewer.svelte`, `SessionTimeline.svelte`,
`docs/web-app.md`, and `ephemeral/research/api/API.md`.

## Recommendation

Start with a renderer-independent typed graph and a supported static pass that
shares scope analysis with lints. Keep source templates and runtime instances
separate. Make ambiguous source/runtime joins explicit. Use named scopes plus
explicit result values for the first command proof. Add command lifecycle
events only after exact process-level evidence is a stated product requirement.
This is compatible with the parent recommendation; the material qualification
is that same-name call-site collisions and unnamed/re-asked `Generate` calls
prevent exact static lighting without an additional identity contract.
