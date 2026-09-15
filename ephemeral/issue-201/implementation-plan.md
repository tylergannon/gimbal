# Issue 201: typed workflow graph — implementation handoff for Sol

## Outcome and authority

Implement a source extractor that generates a Go `workflow.Graph` literal and
a typed workflow value. Generate application bindings for an explicit build-time
list of workflows; save the generated graph as JSON when each run begins. Workflow bodies
remain ordinary Go. This document is a proposed implementation design, not an
implementation or a claim that extraction already works.

**Design status:** the accepted nested model is recorded in
[graph-model.md](/Users/tyler/.codex/worktrees/a256/gimble/ephemeral/issue-201/graph-model.md).
It supersedes the former flat Graph declaration. The final production struct
awaits the recursive encoding decision described there; the approved command API
also needs its concrete contract before implementation.
This handoff is not yet a settled type contract for Sol to implement verbatim.

Read the locally cached issue and its owner comment:
`/Users/tyler/.codex/worktrees/a256/gimble/ephemeral/issue-201/issue-and-comments.md`.
The owner comment dated **2026-09-14 23:10:57 UTC**, with Tyler's subsequent
clarifications in this handoff, supersedes conflicting text:

- Generate **Go**, not a standalone JSON manifest. The Go type is canonical;
  skgo/polytype supplies its application serialization and TypeScript projection.
- Add `gimble.Workflow[T]`, generated workflow values, and typed `Runtime.Run[T]`.
  Tyler's subsequent direction replaces runtime workflow registration with an
  explicit build-time workflow list and generated concrete application bindings.
- Make the builtin sprint's two repository checks explicit constant-key writes.
  Preserve a dynamic-key negative fixture for #162; do not weaken that rule.
- Unresolved structure produces inspectable partial Go and a failing gate.
  Dynamic data expressions and runtime branch choices are valid.

The source baseline inspected for this plan is `0802bcf` on Go **1.27.1**.
This toolchain supports generic methods on concrete types; the existing
`Session.Generate[T]` already uses them. Keep the requested `Runtime.Run[T]`.
The earlier graph declarations and this generic call shape were compiler-checked
locally; passing the wrong input type failed compilation as intended. That probe
does not validate the newly selected nested graph model. Historical results are in
`/Users/tyler/.codex/worktrees/a256/gimble/ephemeral/issue-201/plan-verification.txt`.

One correction to the comment's proof wording is necessary: the actual sprint
has sequential phases, repeats, commands in helpers, an ancestor-owned validator,
and a supervised coder. It has **no Group and no nested supervision**. Extract
the sprint as written; prove parallelism and nested supervision in dedicated
fixtures and the caller example. Do not add those behaviors to the sprint merely
to satisfy that sentence.

### Governing workflow aesthetic — Tyler's clarification

Gimble deliberately favors workflows whose possible structure is explicit and
statically mappable from ordinary Go source. **As a rule of thumb, lint against
anything that prevents that mapping and advise the author to make it explicit.**
Dynamic task dispatch is an authoring violation, just like dynamic Set/SetJSON
keys. It is not a use case the extractor should grow increasingly sophisticated
machinery to accommodate. Direct helper calls and ordinary if/switch branches
express the intended aesthetic.

The exceptions are primitive runtime variability: the number of loop iterations,
the contents of `[]Task`, the branch taken, and data such as prompts and command
arguments. The extractor maps the loop/task template and possible branches;
execution supplies their instances and values. These exceptions do not permit
runtime data to hide which workflow functions may execute or how they relate.

This is an intentional authoring rule, not an unfortunate limitation to relax.
Keep its enforcement bounded and explain unsupported constructs clearly. Prefer
simplifying workflow source over generalizing the analyzer. A partial graph is
diagnostic evidence, not an accepted alternative to this aesthetic. This
clarification governs interpretation of the extraction boundary below.

### Updated evaluation of tradeoff (1): static structural recovery

I endorse this authoring rule and would keep it even without the issue's
constraint. My earlier evaluation gave arbitrary dynamic dispatch too much
weight as an idiomatic Go capability Gimble should accommodate. That was the
wrong frame for the intended workflow aesthetic.

Explicit, statically mappable structure is part of what makes a Gimble workflow
well written. Rejecting dynamic worker dispatch is a useful authoring diagnostic,
like rejecting dynamic Set keys, rather than a deficiency to solve with deeper
analysis. Unknown loop counts, task contents, branch outcomes, and runtime data
remain valid within the visible structure. Design the extractor for these simple
workflow forms, keep lint detection bounded, and diagnose hidden structure rather
than letting it drive the architecture. No relaxation of requirement (1) is
recommended.

### Updated direction for (2): application bindings are generated at build time

Authors explicitly select the workflow types to expose. Generation resolves
each workflow's Go Input and emits its concrete start binding and type/codecs.
An author who wants web launching supplies an actual Svelte page for that
workflow and links to it from the application. There is no runtime workflow
registration prerequisite, generic launch-by-name dispatcher, or schema-driven
generic form. This replaces the earlier WithWorkflow/registration proposal;
section 4 describes the updated delivery.

### Updated decision (3): sealed operations in a nested graph

Operation remains a sealed interface projected by polytype. Its alternatives
represent AgentCall, Command, Set, SetJSON, Subgraph, and relevant Exit steps.
Concrete subgraphs are Sequence, Condition, Loop, Scope, and Group. Each concrete
subgraph can directly implement both sealing markers. Body order supplies
sequence; conditions supply alternatives, loops repetition, and groups concurrency.
Supervision is a separate unordered hierarchy attached beside the watched call.
This replaces the earlier flat 17-variant operation/edge design.

### Updated evaluation of (4): describe agent work, not the program

The remaining temptation to reconstruct Group launches, waits, and intervening
caller work was still excessive. Scoped groups of agent calls in the right order
are sufficient. Keep the nested description useful to a person following agent
work. Recognizable command calls belong at an approved execution boundary, with
direct os/exec use in workflow code linted against. There is no need to retain
universal ports or build a comprehensive program model to justify removing them.

### Decision (5): save the graph with the run

At run creation, serialize the supplied workflow.Graph to JSON and persist it
with that run. The viewer reads the saved shape. This replaces source fingerprints,
graph revision matching, and lookup of historical runs against compiled graphs.
Go remains canonical; JSON is the persisted run artifact, generated through
polytype's codecs. The generated Go delivery and saved JSON serve different stages.

## 1. Graph model and remaining type decisions

Read the accepted semantic contract in
[graph-model.md](/Users/tyler/.codex/worktrees/a256/gimble/ephemeral/issue-201/graph-model.md).
It defines the operation vocabulary, subgraphs, session declarations, context
writes, supervision hierarchy, scope ownership, and default visualization placement.
Graph is conceptually the root subgraph with workflow identity and source/build
metadata. It describes the selected structure connecting calls, commands, and
context writes, rather than every Go statement or a general control-flow graph.

The recursive encoding must be settled before writing the exact production
declarations in `github.com/tylergannon/gimble/workflow`. Both nested bodies and
nested supervisors are recursive; pinned polytype v1.0.0 rejects recursive
definitions. Choose recursive support in polytype or child references in the
canonical Go model. Neither choice changes the agreed conceptual structure.
Do not introduce a second handwritten JSON/TypeScript model.

The Group launch/Wait modeling requirement is withdrawn. Scoped agent work in
the right order is the target; a whole-program map or scheduling timeline is not.
Command calls will use the approved execution primitive described below, whose
public name, signature, and result contract still need to be defined.

The old Graph struct has been removed from this handoff so it cannot be mistaken
for the accepted implementation target. Its earlier compile and 17-variant codec
probes remain historical evidence only. Once the encoding is settled, include
all production declarations here and verify their actual polytype/skgo projections.

### Identity and completeness

Keep source identity distinct from runtime instances. One static template describes
a repeated site; repeated tasks and supervisor ticks do not duplicate that template.
Same-named sites remain distinct. Bounded helper expansions retain both the lexical
source site and their calling context, so two helper callers remain distinct.
IDs must be deterministic across regeneration and checkout relocation; they are not a promise of identity across arbitrary source edits.

Each run stores its own generated Graph as JSON when it begins. Read that saved
shape for run visualization, including after application changes. Workflow and
static site identities are still useful labels/references; source fingerprints
and graph revision hashes are not needed to select a run's shape. Generation
staleness is checked by regeneration and comparison during the build.

Completeness applies to the selected relevant workflow structure. Omitting an
ordinary calculation is valid; losing a branch or early return that changes which
agent calls can run is not. Unresolved relevant calls, session/context bindings,
or supervision produce anchored diagnostics and partial output with a failing
gate. Dynamic data is valid; dynamic structural dispatch and dynamic Set keys
remain authoring violations. Validate known references without inventing targets
for unresolved relationships.

## 2. Typed workflow and generated object

The root package defines exactly the contract from the owner comment:

```go
type Workflow[T any] interface {
	Name() string
	Graph() workflow.Graph
	Run(context.Context, T) error
}
```

For `Sprint(context.Context, Input) error`, generate `workflow_gen.go` in the
sprint package with an exported zero-state type `SprintWorkflow`, an exported value
`var Workflow SprintWorkflow`, a compile-time `gimble.Workflow[Input]`
assertion, and these methods:

- `Name() string`: returns the selected constant name.
- `Graph() workflow.Graph`: returns the generated literal, with fresh backing
  storage for slices/payloads so callers cannot mutate a shared global graph.
- `Run(ctx context.Context, in Input) error`: returns `Sprint(ctx, in)` directly.

No context, runtime, input, or closure lives in that generated value. It neither
starts a run nor registers itself. `workflow` imports no runtime/web packages;
`gimble` can import `workflow`; `web` imports both without a cycle.

Expose a generator at `cmd/gimblegraph`. Proposed caller directive, executed in
the workflow package, using the dependency version selected in the caller's module:

```go
//go:generate go run github.com/tylergannon/gimble/cmd/gimblegraph -name sprint -entry Sprint -output workflow_gen.go
```

The flags explicitly select one package-local entry function and constant name.
Infer Input from its signature; require exactly `(context.Context, T) error`.
Do not require T to implement gimble.Output or share builtin input fields.
Document these flags and generation in README, including caller-module use.

Fresh generation must work with the output file absent, and regeneration must
work after Input changes. Exclude this generator's previous output while loading
the selected package. Load that package, not callers which depend on its generated
Workflow variable. Reserve `workflow_gen.go` for generated declarations and keep
handwritten references to Workflow out of the package being bootstrapped.

Write partial output with diagnostics before returning nonzero on structural or
authoring failure. Emit no runnable adapter if the entry signature itself cannot
be resolved. Never leave an older clean artifact looking like the new successful
result after a failed extraction. This is one Go emitter, not a second JSON model.

## 3. Extraction boundary

Use typed package loading, resolved function/method symbols, and AST source
anchors to recover scoped, ordered agent work. Do not build a whole-program
control-flow or dataflow model. Reuse the already pinned x/tools and the existing
`internal/gimblelint` authoring rules. Share only the small symbol/binding facts
actually needed by both; do not rewrite #162 as a general analysis framework.

The required reachable helper chain is:

`Sprint → run → goalText / runTask → command / git`.

Follow direct, source-visible helpers in the selected workflow package with
context, session, and callback parameter bindings, including lexical
captures and simple aliases. Treat known Gimble callbacks and implicit planner/
supervisor behavior as API semantics; do not descend into harness implementations
or Gimble runtime internals. Ordinary formatting, file reads, adapter construction,
and opaque runtime data expressions are not unresolved workflow dispatch.

Use branches as branches even if their condition is input-dependent. The two
`Sprint` entry branches select dry/live harnesses but preserve recoverable workflow
structure. Do not execute either branch during generation. Expand the necessary
acyclic helper calls; recursion and unresolved structural dispatch terminate that
path with evidence and a diagnostic. Unknown option bundles that could hide
supervisors likewise prevent a complete graph. No whole-program pointer analysis,
arbitrary Go evaluation, or exhaustive ownership proof.

The existing authoring linter remains bounded. For a selected extraction target,
both generation and the lint/build gate must reject incomplete structural analysis;
do not advertise an ordinary `gimble lint ./...` pass as proof of graph coverage.
Use explicit selected-entry checks rather than trying to extract every arbitrary
Go function. Add a generator `-check` mode which performs the same extraction and
compares the expected Go output without rewriting it; wire the selected builtin
and caller-fixture checks into the lint/build gate. It exits nonzero for either
diagnostics or drift. Normal generation remains the way to write partial output.
Regeneration/drift checks must ensure edited source cannot silently
ship with a stale graph; `go build` alone does not run generators.

### Command authoring boundary

Provide one approved, opinionated Gimble command execution function. Lint against
direct os/exec use in workflow authoring code and direct authors to that function.
Recognize and capture its call sites in their scope and order. Migrate the
selected workflows' command sites to it; do not teach the extractor to reconstruct
exec.Cmd construction, mutation, Start/Wait, or subprocess lifecycles. The
implementation of this primitive and harness internals may still use os/exec.

Settle the function's name, signature, and result contract before implementing it.
Command results are secondary detail; this decision does not demand a process
trace or extensive result UI. The primitive must remain a direct command operation,
not grow into wrappers for workflow tactics.

## 4. Build-time application bindings and run association

Execution needs no prior workflow registration:

```go
runtime, err := web.NewRuntime(ctx, project, web.WithPort(8080))
// Handle err before using runtime.
err = runtime.Run(ctx, sprint.Workflow, sprint.Input{Issue: issueFile})
```

Retain the typed synchronous entrypoint in web:

```go
func (r *Runtime) Run[T any](ctx context.Context, w gimble.Workflow[T], in T) error
```

Typed Run obtains the graph from the supplied workflow, creates the normal
run/root scope, and calls `w.Run(runCtx, in)` inside it. Wrong input types fail
compilation. Reject incomplete/diagnostic-bearing graphs and invalid graph
references without consulting a registration table. Replace the old web Run
signature and migrate its callers; preserve the low-level gimble.Run primitive.

### Explicit workflow list and generated start functions

Add the requested `gimble generate-bindings` command. The author's directive
selects concrete workflow types, for example:

```go
//go:generate gimble generate-bindings -workflow-type MyType,MyOtherType -pkg ./workflows -out web/src/routes/new/generated_[type].remote.go
```

The optional `-pkg` selects the source package, defaulting to the current package.
`-out` selects the generated binding location; `[type]` expands per workflow.
Use skgo's `*.remote.go` convention for emitted remote declarations, including
the default filename, so the existing generator discovers them. Authors choose
page paths and links explicitly. Workflow selection is resolved at build time.
The exported SprintWorkflow type generated in section 2 is one such selection.

For each selected type, infer its concrete Input from Run's signature and emit
a separate typed start function. For example, a sprint binding has this shape:

```go
func startSprint(ctx context.Context, in sprint.Input) (string, error)
```

Its generated body starts exactly sprint.Workflow with exactly sprint.Input
through the runtime and returns the new run ID. Mark it with skgo.Command and
use the existing skgo pipeline to generate the TypeScript caller, strict input
decoder, and response encoder. No function dispatches by a submitted workflow
name or converts every Input into a common request type.

Keep the generated route package independent of `web`: web already imports the
generated skgo application, so importing web from a route would create a cycle.
Provide the shared typed start bridge through the root package, available to
caller modules, with the owning runtime reached through its request context.
The runtime supplies that bridge's execution machinery. Concrete Input values
remain typed throughout; the bridge does not require workflow registration.

The workflow-specific page imports that concrete caller and generated Input
type. Its fields, defaults, help text, validation presentation, layout, and
navigation are authored Svelte. Link to that workflow's new-run page from
elsewhere in the application. Generating bindings does not generate a form or
automatically add a navigation link. Prove this with a small caller-example page;
building a general launcher or redesigning the builtin UI is outside this task.

### Reuse polytype and skgo

The pinned polytype v1.0.0 exposes `codegen.Generate`/`codegen.Gen`, including
JSON Schema, Go JSON support, TypeScript, and devalue output options. Drive these
as a library for each concrete Input; group generation requests by declaring
package. A small generated build-time driver can call `polytype.Declare[Input]`
for types discovered by the command. JSON support for ordinary structs uses Go's
normal encoding; polytype adds the codecs required for its enums/unions. Input
must be a shape polytype supports to cross the web boundary.

skgo v0.4.1 already uses polytype's grammar, TypeScript, and devalue generators
for remote parameters and results. Generate the workflow-specific Go declarations
and let that existing pipeline own their wire codecs and TS callers. Do not emit
competing copies of those artifacts or implement a second remote protocol.
JSON Schema/Go JSON generation and skgo transport generation use the same Input.
The command coordinates these stages; a generic schema-to-form renderer is absent.

Generation order is explicit: workflow types/graphs, Input schema/JSON support
and concrete remote declarations, then skgo's linked packages and TS/devalue
bindings. Avoid the route-module bootstrap cycle already handled by skgo. Fresh
generation and regeneration after an Input edit must both work. The build fails
on unsupported Input shapes, unresolved selected types, or output/name collisions.

### Save the graph when the run begins

Typed Run obtains the generated Graph from its supplied workflow and persists
that shape as `graph.json` in the run directory when creating the run, before
workflow work starts. Use the canonical Go Graph and its polytype-generated JSON
support. If writing the graph fails, return the error without executing the
workflow. The run's saved shape stays unchanged for its lifetime.

The viewer reads this saved graph for both live and finished runs. Application
restarts or a newly compiled workflow do not replace an earlier run's shape.
There is no graph revision matching, historical registry, or source fingerprint
lookup. A low-level run without a supplied graph remains observable without one;
never fill in a missing saved shape from today's compiled workflow.

The explicit build-time workflow list can still expose current generated graphs
for pre-run inspection. Historical run inspection reads the run's own graph.json.
Keep this file alongside the run's durable records; observation-table rebuilding
must preserve it. No digest fields or graph copies in every lifecycle event or
table are required for this association.

Add the smallest typed skgo queries needed to inspect the current generated
shape and a run's saved shape. Return workflow.Graph through the existing generated
boundary. Verify the actual serialized query result. No second handwritten graph
DTO, graph canvas implementation, or generic input dispatcher is part of this work.

### Context lifetime

Retain `web.Runtime.ctx` as the runtime lifetime. Each Run gets a fresh child;
bridge synchronous caller cancellation with `context.AfterFunc`, preserving its
cause and cleaning up the callback. Handle an already-cancelled caller before
invoking work. A cancelled run context is never reused. Cancelling one run does
not cancel siblings; runtime cancellation reaches all runs.

HTTP-triggered work must start from the runtime lifetime, not a request context
which expires when the browser leaves. Preserve request cancellation for short
query/steer operations. A generated start function returns once the run has been
created, with its ID; it does not wait for the entire workflow. The runtime owns
the background execution and records its eventual result/error. Closing the
request or navigating to the run page must not cancel that execution. Use one
shared runtime start implementation underneath the concrete generated bindings.

## 5. Delivery sequence

1. Settle the recursive encoding and approved command API contract against the
   accepted model; write and verify the complete Graph declarations. Then land the
   graph invariants and generated typed workflow contract. Prove minimal generation
   from a separate module before growing extraction coverage.
2. Extract the actual sprint helper chain and structural fixtures. Replace only
   its fixed check loop with explicit `go vet ./...` and `go test ./...` calls and
   Set keys `"repository check 1"` / `"repository check 2"`. Preserve order,
   early-return behavior, pass/fail logic, per-check publication, and direct task
   values visible to the planner. Remove the now-unused mutable `checks` list;
   adapt its existing test instead of retaining a test-only production bypass.
   Route workflow commands through the approved primitive and enforce its lint rule.
3. Generate the explicit workflow/Input bindings, migrate web Run and callers,
   persist each run's graph JSON, and expose typed start/inspection functions. Demonstrate
   one authored workflow-specific Svelte page. Coordinate run storage integration with
   #173's observation work without implementing that issue's UI.
4. Demonstrate the claims below, then complete normal build/lint/Go/web and
   generation-drift checks. Report failures as failures, not as narrative success.

Likely implementation locations: new `workflow/graph.go`, root `workflow.go`,
`cmd/gimblegraph/`, a small internal extraction package, and the
`generate-bindings` subcommand in `cmd/`; existing
`internal/gimblelint/`, `internal/workflows/sprint/`, `cmd/sprint/main.go`,
`web/runtime.go`, `run.go`, `internal/observation/`, and typed query
declarations in `web/src/routes/`. Generated files are produced by their tools.
README explains the explicit workflow list, generation, and authored-page contract.

## 6. Behavioral acceptance

| Claim | Evidence the implementation must produce |
| --- | --- |
| Typed consumer works end to end | A separate Go module with its own unrelated Input generates from scratch, compiles, runs through Runtime without registration, and reads its saved graph. A wrong Input type is rejected by the compiler. |
| Web entrypoints are concrete | An explicit list of two workflows with different Inputs generates distinct typed remote callers and decoders. An authored Svelte page links to and starts its specific workflow, receives its run ID, and navigates away while the run continues. Invalid payloads fail before work starts. No generic form or runtime workflow selector is involved. |
| The builtin graph reflects source | Inspect generated Sprint output against `Sprint`, `run`, `goalText`, `runTask`, `command`, and `git`: research/fork, outer rounds, inner planner/task repeat, coder/supervisor, task command, ancestor validator, both fixed checks, conditional commit, final validation/merge. Dry-run alternatives remain visible. |
| Scoped agent work is legible | Fixtures show agent calls in their scopes and source order, alternative branches, repeated bodies, and grouped concurrent work. Sibling list order does not imply sequential completion. A full program map, unrelated caller work, and a launch/join timeline are not acceptance requirements. |
| Sessions, calls, and scopes are distinct | Two AgentCalls reference one conversation. An ancestor-owned session retains its owner when called in a child execution scope. A fork retains its origin and creation scope. Conditions and ordinary Go loops do not manufacture Gimble scopes. |
| Context evolution follows source | Set and SetJSON stay in order within each branch/body, retaining constant keys and value expressions. Actual scopes determine ownership; stored values are not assumed to have appeared in a prompt. |
| Supervision is hierarchical and local | An attachment targets a worker call; its unordered supervisors may themselves have supervisors watching their looks. An ancestor-owned reviewer appears beside a deeper child call while referencing its original session. Sequential tasks can reuse that conversation; task-local creation gives fresh conversations. A short live turn can finish with zero looks. No synthetic look nodes or sequential approval gates are required. |
| Commands use the approved boundary | Workflow commands use the opinionated Gimble function and their calls are captured in scope and source order. Direct os/exec use in workflow code produces an actionable lint diagnostic. A harmless command demonstrates capture through the function. No arbitrary exec.Cmd lifecycle analysis is required. |
| Identity survives reuse and placement | Same-named sites remain distinct, repeated tasks have one static template, and two helper callers retain distinct call sites. Local appearances of an ancestor-owned supervisor retain one session identity and do not duplicate recorded turns or usage. Ownership remains inspectable. |
| Failure is inspectable and fails closed | Dynamic Set keys, map-dispatched workers/simple aliases, and unresolved structural targets produce anchored diagnostics and nonzero gates. A valid dynamic branch/data fixture passes. Failed regeneration cannot leave a stale clean graph presented as current. |
| Each run retains its shape | Start a run, change/rebuild the workflow, and restart the application: the old run still displays its original graph.json and a new run saves the new shape. A graph write failure prevents workflow execution. Table rebuilding preserves the saved graph; a run without a graph is never assigned today's shape. |
| Cancellation is independent | Two active runs share one runtime; cancel A and B remains active; another run can start after A ends. Runtime cancellation stops all active runs. An already-cancelled synchronous caller does not start workflow work. |
| Go remains the single graph model | Inspect a run's saved graph through a real skgo query and generated TypeScript/devalue boundary, comparing nested bodies or their encoded child references, command semantics, session ownership, and hierarchical attachments. No hand-maintained JSON/TS graph model. |
| Operations remain a sealed union across projections | Every concrete Operation/Subgraph variant and multiple levels of supervision round-trip through the chosen Graph encoding and generated JSON/devalue codecs. Generated TS discriminates on kind; unknown kinds and fields belonging to another variant are rejected at the decoding boundary. The old nonrecursive 17-variant probe does not satisfy this claim. |

Use source fixtures for static claims and an isolated executable caller module for
runtime claims. A harmless real command and a cheap native agent turn can demonstrate
the command/turn distinction without running Sprint's repository-modifying workflow
against this repository. Use Codex `gpt-5.6-luna`, Claude Haiku, or Gemini flash for
attestation and state which ran. The builtin dry run is prompt inspection, not
proof that its commands or live agent work executed. No production implementation
or live execution of the proposed typed-workflow API has been performed as part
of this planning task. The separate Fable review used the existing run-prompt API;
its outcome and harness error are recorded in fable-review-result.md.

## Research context, already cached locally

The pinned research is useful for semantic rationale but contains superseded
policy. It does not override the issue's latest owner comment or this plan's
explicit corrections:

- `/Users/tyler/.codex/worktrees/a256/gimble/ephemeral/issue-201/research-recommendation.md`
- `/Users/tyler/.codex/worktrees/a256/gimble/ephemeral/issue-201/research-ui-graph-extraction-notes.md`
- `/Users/tyler/.codex/worktrees/a256/gimble/ephemeral/issue-201/research-ui-design-brief.md`

Keep the delivery bounded: no high-level workflow tactics, layout fields,
reflection/runtime.Caller, hidden init registration, source rewriting, exact
source-to-runtime instance correlation, process telemetry, general recursive
analysis, or critical-path calculation.
