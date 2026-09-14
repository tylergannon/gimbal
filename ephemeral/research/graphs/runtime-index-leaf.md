# Runtime graph index leaf

Use this leaf to route future graph/runtime work. It intentionally points to
the source evidence file rather than duplicating the corpus.

## Claims and routes

| Route/query | Evidence | What it supports | Boundary |
| --- | --- | --- | --- |
| `runtime graph scope key ordinal prefix` | [scope.go:29-65](../../../scope.go#L29) | Runtime scope instances are parent-linked ordinal paths; prefix gives containment. | Same-name sites under one parent share a node key after ordinals are stripped; runtime ordinals do not identify source sites. |
| `runtime session owner turn execution scope` | [rows.go:50-93](../../../internal/observation/rows.go#L50) | Owner scope and execution scope are separate facts; fork parent is persisted. | A session can be borrowed in a child; static ownership/lifetime is not a borrow checker. |
| `runtime Group Go Wait concurrency` | [group.go:19-98](../../../group.go#L19) | Group child scopes and Wait establish observable intervals and joins. | Raw goroutines are outside workflow contract; a missing Wait is a source lint, not inferred from rows. |
| `runtime Loop task planner scope` | [loop.go:82-164](../../../loop.go#L82) | Planner decisions and repeated task scopes are typed runtime regions. | Dynamic task count/selection is runtime data, not a static prediction. |
| `runtime supervision nested reviewer look` | [supervise.go:75-129](../../../supervise.go#L75) | Reviewer session, worker turn, nested looks, and steers are feasible typed relations. | Current reducer drops `supervise_attached`; recover from run log or add edge frames later. |
| `runtime fork steer killed edge` | [events.go:87-161](../../../events.go#L87) | Fork parent, steer source/target, and kill target/by/reason are explicit lifecycle facts. | Current observation snapshot has no edge collection; `Killed.By` is a string actor, not a typed source ID. |
| `runtime observation reducer edge loss` | [store.go:203-280](../../../internal/observation/store.go#L203) | Current table reducer handles core rows but ignores session close, supervise, steer, killed, Complete. | Raw `run.jsonl` preserves the events; UI receives only six-table rows plus native event frames. |
| `runtime SSE snapshot row totals event` | [http.go:17-120](../../../internal/observation/http.go#L17) | Graph facts would need to fit snapshot/row/event streaming or add a graph frame. | No cursor/Last-Event-ID and no graph-specific route/frame today. |
| `runtime current UI flat list` | [RunViewer.svelte:62-87](../../../web/src/lib/observation/RunViewer.svelte#L62) | Current page renders flat scope sections and turn invocations. | It has no topology layout or overlap timeline; `SessionTimeline` is transcript-only. |
| `runtime command exec observability` | [sprints.go:284-310](../../../internal/workflows/sprint/sprints.go#L284) | Static extractor can locate ordinary `os/exec`; workflow returns an exit code and combined output, then explicitly records a `$ command`/`exit N` summary value. | Runtime sees no process start/duration/stderr lifecycle or command row. A named Scope around a command plus explicit result recording is the smallest no-wrapper probe, but scope duration is only an enclosing interval. |
| `static graph runtime instance join key` | [API.md:734-819](../../../ephemeral/research/api/API.md#L734) | Static template names and runtime ordinal instances can be conceptually separated; typed non-prefix relations are defined. | Name paths are not guaranteed unique across distinct same-name call sites; unnamed Generate has no call-site node name. |
| `static SSA analyzer feasibility` | [API.md:856-870](../../../ephemeral/research/api/API.md#L856), [runtime-go-analysis.html](runtime-go-analysis.html), [runtime-go-ssa.html](runtime-go-ssa.html) | Go analyzer/SSA can inspect typed calls, CFG/branching, and context propagation in supported source shapes. | It cannot infer dynamic timing, process outcomes, or arbitrary interface/runtime context flow. |

## Recommended typed data shape

Keep one renderer-independent hierarchy with node kinds `run`, `scope`,
`group`, `loop`, `task`, `session`, `turn`, and `command`. Derive containment
from scope-key prefixes. Add typed relations for `owns-session` (creator scope
to session), `uses-session` (a call/turn to the session it invokes),
`forked-from`, `supervises` (reviewer to worker turn), `steers`, and `killed`.
Treat static source sites as templates with source/name/conditions/repetition;
runtime rows are ordinal instances with status/timing/results. Keep unresolved
mapping explicit when two static sites could map to one runtime name path.

Preserve ordinary Go control flow. The first command experiment should be an
explicitly named Scope containing direct `exec.CommandContext` and `Set` of
exit/output. Do not describe that interval as process-exact. A separate
command lifecycle event is only justified if exact process start/end, output
references, or Start/Wait spans become a product requirement.

## Evidence file

See [runtime-source-evidence.md](runtime-source-evidence.md) for the complete
line-numbered source map and verification notes.
