# Issue 201: typed workflow graph — implementation handoff for Sol

## Outcome and authority

Implement a source extractor that generates a Go `workflow.Graph` literal and
a typed workflow value. Generate application bindings for an explicit build-time
list of workflows; associate each run with the exact graph revision. Workflow bodies
remain ordinary Go. This document is a proposed implementation design, not an
implementation or a claim that extraction already works.

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
The declarations below and this generic call shape were compiler-checked locally;
passing the wrong input type failed compilation as intended. Results are in
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

### Decision (3): Operation is a sealed union

Use a sealed Operation interface with concrete struct variants, projected by
polytype. Each variant declares the sealing method directly. Remove the separate
OperationKind and nullable Agent/Command/Value payload fields. The declarations
below implement this decision; Go uses a type switch and generated TypeScript
uses polytype's discriminator.

## 1. The concrete graph type

Put these declarations in package
`github.com/tylergannon/gimble/workflow`, in `workflow/graph.go`.
This is the proposed data contract. It separates containment, resource ownership,
control flow, and supervision; none is inferred from the ordering of a slice.
Supporting types below are part of the definition, not placeholders.

```go
package workflow

// Graph describes possible source structure, never an observed execution.
type Graph struct {
	ID               string        `json:"id"`
	Name             string        `json:"name"`
	SchemaVersion    int           `json:"schema_version"`
	GeneratorVersion string        `json:"generator_version"`
	Entrypoint       string        `json:"entrypoint"`
	Module           Module        `json:"module"`
	Build            Build         `json:"build"`
	SourceDigest     string        `json:"source_digest"`
	Root             string        `json:"root"`
	Regions          []Region      `json:"regions"`
	Sessions         []Session     `json:"sessions"`
	Operations       []Operation   `json:"operations"`
	Flow             []Flow        `json:"flow"`
	Supervision      []Supervision `json:"supervision"`
	Complete         bool          `json:"complete"`
	Diagnostics      []Diagnostic  `json:"diagnostics"`
}

type Module struct {
	Path    string `json:"path"`
	Version string `json:"version"` // Empty for an unversioned local module.
}

// Build records the effective extraction configuration, not the host's paths.
type Build struct {
	GoVersion  string    `json:"go_version"`
	GOOS       string    `json:"goos"`
	GOARCH     string    `json:"goarch"`
	Tags       []string  `json:"tags"`
	Settings   []Setting `json:"settings"`
}

type Setting struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Source locates a lexical site and its bounded static helper expansion.
type Source struct {
	SiteID   string   `json:"site_id"`
	File     string   `json:"file"`     // Slash-separated, module-relative.
	Function string   `json:"function"` // Qualified declaring function.
	Start    Position `json:"start"`
	End      Position `json:"end"`      // Exclusive.
	CallPath []string `json:"call_path"` // Outer-to-inner helper call SiteIDs.
}

type Position struct {
	Line   int `json:"line"`   // One-based.
	Column int `json:"column"` // One-based byte column.
}

// Expression preserves source text; Value is a Go constant's exact spelling.
// Constant=false means dynamic data, not incomplete structural recovery.
type Expression struct {
	Text     string `json:"text"`
	Constant bool   `json:"constant"`
	Value    string `json:"value"`
}

type RegionKind string

const (
	RootRegion  RegionKind = "root"
	ScopeRegion RegionKind = "scope"
	GroupRegion RegionKind = "group"
	LoopRegion  RegionKind = "loop"
	TaskRegion  RegionKind = "task"
)

// Regions are actual Gimble scope templates. Plain helpers/for loops do not
// acquire scope ownership simply because the extractor visits their bodies.
type Region struct {
	ID     string     `json:"id"`
	Parent string     `json:"parent"` // Empty only on Graph.Root.
	Kind   RegionKind `json:"kind"`
	Name   string     `json:"name"`
	Source Source     `json:"source"`
	Order  int        `json:"order"`  // Source/display order only.
	Entry  string     `json:"entry"`  // RegionEntry operation ID.
	Exit   string     `json:"exit"`   // RegionExit operation ID.
}

// Session is a resource owned by a region, not a node in the control flow.
type Session struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Owner      string     `json:"owner"`       // Region ID.
	CreatedBy  string     `json:"created_by"`  // SessionCreate/SessionFork op ID.
	ForkedFrom string     `json:"forked_from"` // Session ID, empty for creation.
	Adapter    Expression `json:"adapter"`
	Model      Expression `json:"model"`
	Workdir    Expression `json:"workdir"`
}

// Operation is a sealed union. Its variants declare operation directly.
type Operation interface {
	operation()
}

// Site is shared metadata, embedded in each variant. It is not a variant.
type Site struct {
	ID     string `json:"id"`
	Scope  string `json:"scope"` // Execution region, not session owner.
	Source Source `json:"source"`
	Order  int    `json:"order"` // Source/display order only.
	Label  string `json:"label"`
}

type RegionEntry struct{ Site }
type RegionExit struct{ Site }
type SessionCreate struct{ Site }
type SessionFork struct{ Site }
type Merge struct{ Site }
type ParallelLaunch struct{ Site }
type ParallelJoin struct{ Site }
type Unresolved struct{ Site }

type AgentCall struct {
	Site
	Session    string     `json:"session"`
	Prompt     Expression `json:"prompt"`
	OutputType string     `json:"output_type"`
}

type PlannerCall struct {
	Site
	Session    string     `json:"session"` // Session ID used by this turn.
	Prompt     Expression `json:"prompt"`
	OutputType string     `json:"output_type"`
}

type SupervisorLook struct {
	Site
	Session    string     `json:"session"`
	Prompt     Expression `json:"prompt"`
	OutputType string     `json:"output_type"`
}

type CommandConstruct struct {
	Site
	Method     string       `json:"method"` // Command or CommandContext.
	Executable Expression   `json:"executable"`
	Args       []Expression `json:"args"`
	Dir        Expression   `json:"dir"`
}

type CommandExecute struct {
	Site
	Method       string       `json:"method"`
	Construction string       `json:"construction"` // CommandConstruct ID.
	Executable   Expression   `json:"executable"`
	Args         []Expression `json:"args"`
	Dir          Expression   `json:"dir"`
}

type CommandWait struct {
	Site
	Construction string `json:"construction"` // CommandConstruct ID.
	Start        string `json:"start"`        // CommandExecute ID whose Method is Start.
}

type ValueWrite struct {
	Site
	Key        Expression `json:"key"`
	Expression Expression `json:"expression"`
	JSON       bool       `json:"json"` // SetJSON versus Set.
}

type Branch struct {
	Site
	Condition Expression `json:"condition"`
}

type Repeat struct {
	Site
	Condition Expression `json:"condition"`
}

func (RegionEntry) operation()      {}
func (RegionExit) operation()       {}
func (SessionCreate) operation()    {}
func (SessionFork) operation()      {}
func (AgentCall) operation()        {}
func (PlannerCall) operation()      {}
func (SupervisorLook) operation()   {}
func (CommandConstruct) operation() {}
func (CommandExecute) operation()   {}
func (CommandWait) operation()      {}
func (ValueWrite) operation()       {}
func (Branch) operation()           {}
func (Merge) operation()            {}
func (Repeat) operation()           {}
func (ParallelLaunch) operation()   {}
func (ParallelJoin) operation()     {}
func (Unresolved) operation()       {}

type Port string

const (
	Before Port = "before"
	After  Port = "after"
)

type Endpoint struct {
	Operation string `json:"operation"`
	Port      Port   `json:"port"`
}

type FlowKind string

const (
	SequenceFlow FlowKind = "sequence"
	BranchFlow   FlowKind = "branch"
	LaunchFlow   FlowKind = "launch"
	JoinFlow     FlowKind = "join"
	RepeatFlow   FlowKind = "repeat"
	ExitFlow     FlowKind = "exit"
)

type Flow struct {
	Kind      FlowKind   `json:"kind"`
	From      Endpoint   `json:"from"`
	To        Endpoint   `json:"to"`
	Condition Expression `json:"condition"` // Empty for an unconditional relation.
}

// An attachment describes a possible periodic look, not an approval dependency.
type Supervision struct {
	ID          string     `json:"id"`
	Source      Source     `json:"source"`  // WithSupervisor attachment site.
	Session     string     `json:"session"` // Supervisor session ID.
	Target      string     `json:"target"`  // Watched agent/planner/look op ID.
	Look        string     `json:"look"`    // Implicit SupervisorLook op ID.
	Instruction Expression `json:"instruction"`
	Interval    Expression `json:"interval"` // Duration expression; constant value in ns.
}

// All diagnostics in this first delivery are gate failures.
type Diagnostic struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Source  Source `json:"source"`
	Subject string `json:"subject"` // Related graph ID; empty for graph-level errors.
}
```

### Polytype projection of Operation

Keep `Graph.Operations` as the direct `[]Operation` field shown above. Add this
in `workflow/schema.go`, following the root package's existing lifecycle union:

```go
//go:build jsonschema

package workflow

import (
	"encoding/json"

	"github.com/tylergannon/polytype"
)

func (Graph) Schema() json.RawMessage     { panic("generated") }
func (Graph) ValidateJSON([]byte) error   { panic("generated") }

var (
	_ = polytype.Declare(Graph.Schema)
	_ = polytype.SealedUnion[Operation]("kind", polytype.Snake)
)
```

Use the existing polytype generation convention (`//go:generate go tool polytype
--validate` on one line in graph.go). It infers union members from the concrete
types and their direct value-receiver methods; do not maintain a second member
list. It supplies `kind: "agent_call"`, `kind: "command_execute"`, etc. on the
wire. No Go Kind field or handwritten discriminator codec is needed. Site's
embedded fields are common metadata; Site does not implement operation().

Marshal/unmarshal the owning Graph using its generated JSON codecs. Generate
TypeScript and devalue through polytype/skgo from the same declaration. Keep
the union field a direct slice and use concrete value variants in literals.
Verify generated schema, JSON, TypeScript, and devalue agree on the variants;
the invalid cases include unknown discriminators and fields from another variant.
These are type-projection checks, separate from graph reference validation.
The full proposed Graph was checked with pinned polytype v1.0.0: all 17 variants
round-trip through generated JSON and devalue codecs, invalid variants are rejected,
and a second generation is unchanged. Details and limits are recorded in
`/Users/tyler/.codex/worktrees/a256/gimble/ephemeral/issue-201/operation-union-verification.md`.

### Invariants and interpretation

**Identity.** `Graph.ID` is derived from the qualified entrypoint and constant
workflow name; it is independent of the source revision. `SourceDigest` is a
SHA-256 over deterministic, sorted extraction inputs: selected package source
files and embed inputs, module/build selection, dependency versions/replacements,
and generator/schema version. Hash local replacement source when its workflow
code is followed. Exclude generated graph outputs and volatile timestamps or
absolute checkout paths. Build.Settings contains effective build-selection values
such as CGO_ENABLED and relevant GOFLAGS, not timestamps or Git dirty status.
Do not use Git HEAD alone: uncommitted source matters.
This fingerprints the extraction inputs, not arbitrary files an agent may read
later. Run-to-graph compatibility uses both ID and digest.

`Source.SiteID` identifies a lexical site using module-relative file, qualified
function, and AST position within that function. Line/column are navigation,
not identity. Graph element IDs add the static helper `CallPath`, relevant scope
binding, and a role suffix for synthetic entry/exit/planner/look nodes. IDs must
be deterministic across regeneration and checkout relocation; do not promise
identity across arbitrary source edits. Same names at distinct sites stay distinct.

One template is emitted per static site and call context, never per observed
iteration, task, re-ask, or supervisor tick. If two distinct callsites invoke the
same helper, their expansions can have different graph IDs while retaining the
same lexical `SiteID`. This avoids sharing one helper return among unrelated
continuations. Neither ID is a runtime scope/session/turn ID.

**Containment and use.** `Region.Parent` forms one tree. `Session.Owner` is the
creation scope; each variant's `Site.Scope` is the execution scope. The Session
field of AgentCall, PlannerCall, or SupervisorLook is the uses-session relation.
For creation/fork operations, `Session.CreatedBy` is the reverse reference. An ordinary
helper inherits the caller's scope binding. Derived contexts preserve that binding
until an actual Gimble scope boundary changes it.

**Variants.** The concrete Operation type determines its fields; there are no
nullable payload combinations or separate kind/payload consistency checks. A
small internal graph validator checks reference integrity, including whether a
referenced operation has the required type. For a partial graph, leave an
unresolved relation empty and attach a diagnostic instead of inventing an endpoint.
All known references must still resolve.

**Control flow.** Before/After refer to the invocation boundary of an operation,
not recorded timestamps. A sequence A.After → B.Before means B follows A if that
path is reached. Branch edges carry source predicates/case labels; outgoing
alternatives are OR choices, never all prerequisites. A Merge rejoins alternatives.
Keep normal/error returns and early exits when they alter the reachable workflow
structure. Ordinary statements can be elided only while preserving those paths.

`Group.Go` is a ParallelLaunch operation. Its After launches the child region's
Entry.Before and also permits the next parent-side operation. Child completion
does **not** precede the next launch. `Group.Wait` is a ParallelJoin: ordinary flow
reaches its Before; JoinFlow edges from launched child Exit.After constrain its
After. It waits for all children actually launched on that path, not every
possible alternative. Group siblings have display order but no invented serial
completion edge. Region entry/exit alone do not imply synchronous containment.

Plain `for`/`range` control uses Repeat/Branch operations and RepeatFlow/ExitFlow
edges in its existing scope. It does not fabricate a new runtime scope. A
`Loop.Tasks` range has a LoopRegion, an implicit PlannerCall, and one TaskRegion:
planner selects task → task entry; task completion → next planner call; no task
or a range break → loop exit. `Loop(...)` constructs the iterator; the loop region
and planner work start when Tasks is consumed, as the runtime actually does.

**Commands.** CommandConstruct.Method is Command/CommandContext;
CommandExecute.Method is Run/Output/CombinedOutput/Start. CommandWait represents
the explicit Wait without a redundant Method field. Execution
and Wait refer to the same construction site. Start.After means the Start call
returned, not that the process exited. Its explicit Wait joins that started process.
Run/Output/CombinedOutput are blocking executions. Keep executable/args/Dir as
expressions, following simple local assignments and helper argument bindings.
A construction with no execution remains a construction. No process outcome,
duration, stdout/stderr, or harness-internal shell command is inferred here.

**Supervision.** For supervisor S watching worker W, create one implicit look L:
`Supervision{Session: S, Target: W, Look: L}` and the SupervisorLook L has `Session: S`.
For supervisor H watching S's looks, the second attachment is
`Supervision{Session: H, Target: L, Look: HL}`. Look operations use the watched
operation's execution scope, even if their sessions belong to an ancestor.
They have their own source identities rooted at the attachment, not made-up
Generate source lines. The default interval is the existing three-minute
runtime default. These relations do not add sequence/join edges between looks
and workers. An attachment can produce zero looks, and looks need not steer.

**Collapse.** Every endpoint retains its original operation ID and port. The
viewer can walk its operation's region ancestors to find the collapsed boundary;
it need not replace endpoints or persist layout-specific proxy nodes.

**Completeness.** Complete means every possible workflow structural relation in
the selected, supported source is recovered. Unknown targets/context/session/
command bindings that affect structure make Complete false and add diagnostics.
A nonconstant Set key can leave structure fully recovered but still adds the
existing hard authoring diagnostic. Success requires Complete and no diagnostics.
Neither a passing bounded #162 lint nor absence of runtime events proves coverage.

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

Use typed package loading, resolved function/method symbols, AST source anchors,
and CFG/SSA facts where needed. Reuse the already pinned x/tools and the existing
`internal/gimblelint` authoring rules. Share only the small symbol/binding facts
actually needed by both; do not rewrite #162 as a general analysis framework.

The required reachable helper chain is:

`Sprint → run → goalText / runTask → command / git`.

Follow direct, source-visible helpers in the selected workflow package with
context, session, command, and callback parameter bindings, including lexical
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

### Static graph availability and durable run association

The same selected workflow list can emit explicit graph references for pre-run
inspection and matching historical runs. This is compiled application data, not
a mutable registration service or a prerequisite for a direct Go Run call.
Keep graph values privately owned so a caller cannot mutate a shared revision.

Pass the verified graph reference through an internal context seam to the
low-level run creation. Add graph ID and digest to `RunStarted` and `RunRow`.
Record them in both lifecycle/project start records and in the normal observation
storage path. Existing `run.json`, reduced `observation.json`, durable deltas,
replay/fallback, and live snapshots must preserve the same fields. Do not implement
association as a live-only side map which disappears when the application restarts.

Resolve a run's reference against the compiled graph list. A matching finished
run resolves after restart;
a different/missing digest yields no compatible graph. A low-level unnamed-graph
run remains observable with no graph. Never select a graph by name alone or
reinterpret an old run with newer source. Historical graph archival is deferred;
if added later, serialize this same Graph value.

Add the smallest typed skgo queries needed to inspect compiled graphs and a
run's matching graph before UI work. Return `workflow.Graph` values through the
existing generated boundary; regenerate bindings/TypeScript instead of authoring
a parallel DTO. Verify the actual serialized query result. Graph rendering and
generic input dispatch remain outside this task.

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

1. Land the graph type/invariants and generated typed workflow contract. Prove
   minimal generation from a separate module before growing extraction coverage.
2. Extract the actual sprint helper chain and structural fixtures. Replace only
   its fixed check loop with explicit `go vet ./...` and `go test ./...` calls and
   Set keys `"repository check 1"` / `"repository check 2"`. Preserve order,
   early-return behavior, pass/fail logic, per-check publication, and direct task
   values visible to the planner. Remove the now-unused mutable `checks` list;
   adapt its existing test instead of retaining a test-only production bypass.
3. Generate the explicit workflow/Input bindings, migrate web Run and callers,
   persist graph references, and expose typed start/inspection functions. Demonstrate
   one authored workflow-specific Svelte page. Coordinate changed run fields with
   #173's observation work without implementing that issue's UI.
4. Demonstrate the claims below, then complete normal build/lint/Go/web and
   generation-drift checks. Report failures as failures, not as narrative success.

Likely implementation locations: new `workflow/graph.go`, root `workflow.go`,
`cmd/gimblegraph/`, a small internal extraction package, and the
`generate-bindings` subcommand in `cmd/`; existing
`internal/gimblelint/`, `internal/workflows/sprint/`, `cmd/sprint/main.go`,
`web/runtime.go`, `run.go`, `events.go`, `internal/observation/`, and typed query
declarations in `web/src/routes/`. Generated files are produced by their tools.
README explains the explicit workflow list, generation, and authored-page contract.

## 6. Behavioral acceptance

| Claim | Evidence the implementation must produce |
| --- | --- |
| Typed consumer works end to end | A separate Go module with its own unrelated Input generates from scratch, compiles, runs through Runtime without registration, and resolves its recorded graph. A wrong Input type is rejected by the compiler. |
| Web entrypoints are concrete | An explicit list of two workflows with different Inputs generates distinct typed remote callers and decoders. An authored Svelte page links to and starts its specific workflow, receives its run ID, and navigates away while the run continues. Invalid payloads fail before work starts. No generic form or runtime workflow selector is involved. |
| The builtin graph reflects source | Inspect generated Sprint output against `Sprint`, `run`, `goalText`, `runTask`, `command`, and `git`: research/fork, outer rounds, inner planner/task repeat, coder/supervisor, task command, ancestor validator, both fixed checks, conditional commit, final validation/merge. Dry-run alternatives remain visible. |
| Flow is semantic | Fixtures demonstrate sequence, mutually exclusive branches, two launched group children and their join, repeat/exit including break/return, and continuation after join. There is no child-completion edge that serializes its sibling. |
| Resources and scopes are distinct | An ancestor session used in a child keeps its Owner while the child call has that child's Scope. A fork points to its origin and its own creation scope. |
| Both supervision targets survive | Worker W, look L, and higher look HL have attachments S→W/L and H→L/HL. A short live turn with an attachment and no look remains valid. |
| Commands retain meaning | Construct-only, blocking Run/Output/CombinedOutput, and Start→Wait examples produce distinct kinds and correct construction links; a command followed by a validator has a real sequence. Dynamic args remain expressions. |
| Identity and boundaries survive reuse | Same-named sites remain distinct, repeated tasks have one static template, two helper callers keep distinct continuations, and cross-scope edges retain their exact endpoints under a containment-based collapse/restore exercise. |
| Failure is inspectable and fails closed | Dynamic Set keys, map-dispatched workers/simple aliases, and unresolved structural targets produce anchored diagnostics and nonzero gates. A valid dynamic branch/data fixture passes. Failed regeneration cannot leave a stale clean graph presented as current. |
| Run association survives restart | Live and finished matching runs resolve to the compiled graph; restart with a changed digest leaves the old run observable but unmatched. References survive durable snapshot, table, and supported replay paths. |
| Cancellation is independent | Two active runs share one runtime; cancel A and B remains active; another run can start after A ends. Runtime cancellation stops all active runs. An already-cancelled synchronous caller does not start workflow work. |
| Go remains the single graph model | Inspect the compiled graph through a real skgo query and generated TypeScript/devalue boundary, comparing representative containment, flow endpoints, command payloads, and nested attachments. No hand-maintained JSON/TS graph model. |
| Operations remain a sealed union across projections | Every concrete Operation variant round-trips through Graph's generated JSON and devalue codecs with its kind and fields intact. Generated TS discriminates on kind; unknown kinds and fields belonging to another variant are rejected at the decoding boundary. |

Use source fixtures for static claims and an isolated executable caller module for
runtime claims. A harmless real command and a cheap native agent turn can demonstrate
the command/turn distinction without running Sprint's repository-modifying workflow
against this repository. Use Codex `gpt-5.6-luna`, Claude Haiku, or Gemini flash for
attestation and state which ran. The builtin dry run is prompt inspection, not
proof that its commands or live agent work executed. No production implementation
or live workflow execution has been performed as part of this planning task.

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
