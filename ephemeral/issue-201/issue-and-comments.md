# Issue 201: Extract workflow graphs from Go source for program and run visualization

Source: https://github.com/tylergannon/gimble/issues/201

Fetched 2026-09-14. The later owner comment supersedes conflicting issue-body and research text.

Workflow authors and the run viewer need a machine-readable description of a workflow's shape: scopes, operations, possible control flow, and supervision. The runtime tree alone describes what was observed; it cannot show unexecuted alternatives or the complete program structure.

Generate an inspectable JSON graph from ordinary Go workflow source through `go generate`. Start with the builtin sprint and one small workflow in a caller module. For Beta, a lint-clean workflow must have completely recoverable possible workflow structure. Diagnostic partial output is useful, but unresolved workflow operations or structural relationships do not count as successful coverage. Runtime data expressions may remain dynamic without obscuring structure.

## First delivery

- Preserve scope regions and their kinds, including groups and loops. A repeated source template appears once; actual iterations belong to a run.
- Identify agent-call and command-execution sites, plus supervisor attachments where statically recognizable. Distinguish constructing an `exec.Cmd` from executing it.
- Retain supported sequence, branch, parallel launch/join, and repeat/exit relations. Keep display/source order separate from execution dependencies.
- Keep session ownership separate from a turn's execution scope: an ancestor-owned session may be used in a child scope.
- Represent supervision as its own relation to the watched turn, including a supervisor whose look turn is itself supervised. Attachment does not imply a look actually occurred.
- Include source anchors, static site identities, and source/build version information. Keep these separate from runtime scope/session/turn instance IDs.
- Follow the statically resolved local helpers needed by the builtin example. Preserve unresolved targets as diagnostic evidence that prevents a clean structural-analysis result. Dynamic command arguments remain expression labels; nonconstant Set/SetJSON keys are hard authoring errors under #162.

The graph describes semantics, not coordinates, colors, or viewport state. It must retain enough containment and cross-boundary endpoint information for the viewer to collapse a scope and restore it without losing its external connections. JSON is enough for the first delivery; a second Go-literal encoder is not required.

## Why the UI needs these distinctions

The intended viewer has a scope-aware map, a linked run timeline, and an inspector that preserves selection. It must support commands, parallel work, repeated tasks, and nested supervision alongside agent calls.

For example, a user can inspect task 3's failed command while task 4 is running, without the whole run appearing terminally failed. A source template and a runtime instance therefore cannot be the same identity. A static site without an observed event is not automatically skipped: it could be future work, an untaken branch, or unrecorded work.

Runtime events supply actual intervals, outcomes, loop counts, supervisor looks, and steering delivery. Static command discovery does not supply process telemetry. Existing named scopes with recorded command results can support a first useful observation, but scope duration is not automatically process duration. Exact source-to-run correlation remains a separate integration step; ambiguous matches must stay ambiguous.

## Acceptance

- `go generate` produces inspectable output for the builtin sprint and a small caller-module workflow. Document how callers select a workflow for extraction and invoke generation.
- Source fixtures demonstrate sequential phases, parallel children with a join, and repeat/exit structure without falsely serializing siblings.
- A command followed by an agent validator produces distinct operation kinds. A command only constructed in source is not claimed as an observed process.
- Same-named sites in different scopes remain distinguishable, and repeated execution does not duplicate the source template.
- Fixtures preserve ancestor session ownership versus child execution scope, and both targets in nested supervision.
- Nonconstant Set/SetJSON keys and unsupported workflow dispatch produce diagnostics and fail the lint gate. Partial output remains inspectable but is not accepted as a complete workflow shape. Dynamic command arguments and runtime branch choices remain valid.
- Inspect the builtin manifest against its source. Tests assert semantic relationships rather than incidental layout or node coordinates.

## Boundaries and related work

This is a **Beta deliverable**: useful source-derived program shape is part of the intended Beta UI. Coordinate the generated manifest with **#173**, the observed scope/time/token/cost view, so the viewer can show program structure alongside run observations. An observed-only view is a useful intermediate delivery, not a replacement for the Beta program-shape goal. **#162**, the bounded Set/SetJSON linter, remains a separate Beta task.

The Beta bar is the bounded first delivery above: useful graph output for the builtin sprint and a caller-module example, with explicit partial coverage. Exhaustive Go analysis, general helper expansion, and exact correlation of every runtime event are deferred; they must not hold up this useful graph.

Do not add high-level workflow wrappers, use reflection/runtime.Caller, or attempt exhaustive ownership proofs, arbitrary Go evaluation, whole-program pointer analysis, general recursive helper expansion, or critical-path analysis. Caller/callee links are optional inspection aids, not required default edges. This issue does not include building the UI or exact runtime instrumentation.

## Research and design input

The research is cached as flat files in `ephemeral/research/graphs`, including a Go feasibility probe, independent source-rule reviews, downloaded UI references, and a pinned Tenacious archive. The following links are pinned to the reviewed research commit:

- [Extraction feasibility and recommended graph model](https://github.com/tylergannon/gimble/blob/ecfa2d8/ephemeral/research/graphs/recommendation.md)
- [UI-informed semantic requirements](https://github.com/tylergannon/gimble/blob/ecfa2d8/ephemeral/research/graphs/ui-graph-extraction-notes.md)
- [Design brief and drill-down states](https://github.com/tylergannon/gimble/blob/ecfa2d8/ephemeral/research/graphs/ui-design-brief.md)
- [Research index](https://github.com/tylergannon/gimble/blob/ecfa2d8/ephemeral/research/graphs/INDEX.md)


## Workflow authoring stance

Beta linter #162 hard-fails on detected dynamic workflow-function dispatch, such as `workers[task.Kind](ctx)` or a simple alias of that lookup. Authors should express alternatives as direct worker calls in ordinary `if`/`switch` branches. This makes possible control flow visible without predicting runtime branch choices. Direct helpers and recognized Gimble callbacks remain valid. Set/SetJSON keys must also be compile-time constants. Dynamic command arguments, context values, and model/provider configuration remain permitted. The workflow still compiles and runs without the linter; this is an opinionated authoring rule, not a new runtime restriction. Incomplete lint detection is acceptable; do not add general pointer analysis to enforce it exhaustively.


## Constant-key acceptance alignment

The existing builtin sprint is deliberately a negative linter case: `gimble.Set(ctx, fmt.Sprintf("repository check %d", i+1), ...)` must fail #162's constant-key rule. Its two fixed repository checks can later be written explicitly with constant result keys. Do not weaken the rule or exempt the builtin to make this case pass. Runtime behavior remains unchanged; this is the opinionated authoring gate.


---

## Comment by tylergannon, 2026-09-14T23:10:57Z

Source: https://github.com/tylergannon/gimble/issues/201#issuecomment-5672054553

## Plan: typed workflow graphs

### Outcome

`go generate` analyzes an ordinary Go workflow and generates a typed Go object describing every recoverable possible workflow shape. The application compiles that object, registers it as an available workflow, and associates each run with the exact graph used to start it.

There is no standalone JSON manifest or separately designed JSON contract. If the graph crosses the existing skgo/polytype boundary, normal application serialization projects the Go type to TypeScript.

### Workflow contract

Gimble defines one generic contract:

```go
type Workflow[T any] interface {
    Name() string
    Graph() workflow.Graph
    Run(context.Context, T) error
}
```

The generator receives a constant workflow name and an entry function such as `func Sprint(context.Context, Input) error`. It emits a typed `workflow.Graph` literal, a small generated implementation of `Workflow[Input]`, and an exported value such as `sprint.Workflow`. Its `Run` method delegates directly to `Sprint`.

Different workflows retain unrelated input types. Nothing converts them to a common request structure.

Application assembly is explicit and type-safe:

```go
runtime, err := web.NewRuntime(
    ctx,
    project,
    web.WithWorkflow(sprint.Workflow),
    web.WithWorkflow(release.Workflow),
    web.WithPort(8080),
)

err = runtime.Run(ctx, sprint.Workflow, sprint.Input{
    Issue: "201",
})
```

`WithWorkflow[T]` extracts only the typed graph into application configuration before the server starts. It does not erase or store the runnable function. `Runtime.Run[T]` accepts the same workflow and exactly its input type, verifies that its graph is registered, creates the run, and invokes it.

### Context ownership

`Runtime` keeps one lifetime context for the application, server, project registry, and shutdown. Every `Run` derives a fresh child context from it.

This remains necessary even for sequential execution: cancelled contexts cannot be reset, and every run owns independent scopes, sessions, deadlines, cancellation causes, and observation state. Concurrent runs must be cancellable independently. Cancelling the runtime cancels every run; cancelling one run never cancels the runtime or its siblings.

A synchronous caller's cancellation is bridged into that run. A run started by an HTTP request derives from the runtime lifetime instead, so closing the browser does not cancel the work. Workflow objects are immutable descriptions and never store contexts.

### Generated graph

The Go graph contains:

- Generator/schema version, entrypoint, module, source digest, and build information.
- Root, explicit scopes, groups and their children, loops, and repeated task templates.
- Session creation, ownership, forks, and agent-call sites.
- Command construction and actual execution as distinct operations.
- Sequence, alternatives, parallel launch/join, repeat/exit, session use, and supervision relations.
- Supervisor look operations, including nested supervision.
- Stable static identities distinct from names, source anchors, and runtime instance IDs.
- Source/display order independently from execution dependencies.
- Diagnostics and whether structural recovery is complete.

The extractor uses resolved Go symbols and follows the directly reachable local helpers needed by the selected workflow. Dynamic values, command arguments, prompts, and branch choices remain expression labels. Unknown workflow targets or structural relationships produce partial generated Go for inspection but make generation and lint fail. There is no pointer analysis, arbitrary helper expansion, or prediction of runtime behavior.

The builtin sprint's two fixed repository checks become explicit calls with constant keys so it is a legitimate lint-clean positive example rather than #162's preserved negative case.

### Application and run association

The application stores registered `workflow.Graph` values directly. When a run starts, its record carries the graph's stable identity and source digest. The observation layer associates that run with the registered Go object.

A finished run is matched only when identity and digest agree. A newer application must not reinterpret an older run using changed source. Historical graph persistence, if required later, would serialize the same Go value into the run store; it would not introduce another canonical representation.

### Proof claims

- A separate caller module generates, compiles, registers, and runs a `Workflow[CallerInput]` without adopting Gimble's builtin input shape.
- The application can inspect registered workflows before any run and retrieve the exact graph associated with a live or finished matching run.
- The builtin graph contains its sequential phases, parallel children and join, loop template, helper-owned commands, ancestor-owned sessions used in child scopes, and nested supervision.
- Group siblings have no false sequence edge; branches remain alternatives; repeated execution does not duplicate source templates.
- Constructing `exec.Cmd` does not claim execution, while `Run`, `Output`, `CombinedOutput`, and `Start` are represented correctly.
- Unsupported dispatch and nonconstant context keys leave inspectable diagnostics and fail the gate; dynamic runtime data remains valid.
- Two concurrent runs can share the runtime while cancellation of either leaves the other alive; runtime cancellation stops both.
- The graph crosses the existing skgo/polytype boundary without a hand-authored JSON model.

Routine build, lint, Go, web, and generation-drift checks run after these behavioral claims are demonstrated.

### Excluded

No graph UI, layout coordinates, runtime call-site reflection, `runtime.Caller`, source rewriting, hidden `init` registration, workflow wrappers, process telemetry, critical-path analysis, or exhaustive Go analysis.
