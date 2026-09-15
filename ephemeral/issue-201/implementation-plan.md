# Issue 201: typed workflow graph — implementation handoff for Sol

## Outcome and authority

Implement a source extractor that generates a Go `workflow.Graph` literal and
a typed workflow value. Register those graphs before the application starts;
associate each run with the exact registered graph revision. Workflow bodies
remain ordinary Go. This document is a proposed implementation design, not an
implementation or a claim that extraction already works.

Read the locally cached issue and its owner comment:
`/Users/tyler/.codex/worktrees/a256/gimble/ephemeral/issue-201/issue-and-comments.md`.
The owner comment dated **2026-09-14 23:10:57 UTC** supersedes conflicting text:

- Generate **Go**, not a standalone JSON manifest. The Go type is canonical;
  skgo/polytype supplies its application serialization and TypeScript projection.
- Add `gimble.Workflow[T]`, generated workflow values, `web.WithWorkflow[T]`, and
  typed `Runtime.Run[T]`, including registration, run association, and cancellation.
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

type OperationKind string

const (
	RegionEntry      OperationKind = "region_entry"
	RegionExit       OperationKind = "region_exit"
	SessionCreate    OperationKind = "session_create"
	SessionFork      OperationKind = "session_fork"
	AgentCall        OperationKind = "agent_call"
	PlannerCall      OperationKind = "planner_call"
	SupervisorLook   OperationKind = "supervisor_look"
	CommandConstruct OperationKind = "command_construct"
	CommandExecute   OperationKind = "command_execute"
	CommandWait      OperationKind = "command_wait"
	ValueWrite       OperationKind = "value_write"
	Branch           OperationKind = "branch"
	Merge            OperationKind = "merge"
	Repeat           OperationKind = "repeat"
	ParallelLaunch   OperationKind = "parallel_launch"
	ParallelJoin     OperationKind = "parallel_join"
	Unresolved       OperationKind = "unresolved"
)

type Operation struct {
	ID         string         `json:"id"`
	Scope      string         `json:"scope"` // Execution region, not session owner.
	Kind       OperationKind  `json:"kind"`
	Source     Source         `json:"source"`
	Order      int            `json:"order"` // Source/display order only.
	Label      string         `json:"label"`
	Expression Expression     `json:"expression"` // E.g. branch/repeat condition.
	Agent      *Agent         `json:"agent"`
	Command    *Command       `json:"command"`
	Value      *Value         `json:"value"`
}

type Agent struct {
	Session    string     `json:"session"` // Session ID used by this turn.
	Prompt     Expression `json:"prompt"`
	OutputType string     `json:"output_type"`
}

type Command struct {
	Method       string       `json:"method"`
	Construction string       `json:"construction"` // Construct op ID; self on construct.
	Start        string       `json:"start"`        // Start op ID for explicit Wait.
	Executable   Expression   `json:"executable"`
	Args         []Expression `json:"args"`
	Dir          Expression   `json:"dir"`
}

type Value struct {
	Key        Expression `json:"key"`
	Expression Expression `json:"expression"`
	JSON       bool       `json:"json"` // SetJSON versus Set.
}

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
later. Registration/run compatibility uses both ID and digest.

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
creation scope; `Operation.Scope` is the execution scope. `Operation.Agent.Session`
is the uses-session relation. For creation/fork operations, `Session.CreatedBy`
is the reverse reference; their Agent/Command/Value payloads are nil. An ordinary
helper inherits the caller's scope binding. Derived contexts preserve that binding
until an actual Gimble scope boundary changes it.

**Payloads.** AgentCall/PlannerCall/SupervisorLook have only an Agent payload;
command kinds have only Command; ValueWrite has only Value. Other kinds have no
payload. A small internal graph validator checks this and reference integrity;
the graph is not a bag of arbitrary properties. For a partial graph, leave an
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

**Commands.** Method is exactly Command/CommandContext for construction;
Run/Output/CombinedOutput/Start for execution; Wait for an explicit join. Execution
and Wait refer to the same construction site. Start.After means the Start call
returned, not that the process exited. Its explicit Wait joins that started process.
Run/Output/CombinedOutput are blocking executions. Keep executable/args/Dir as
expressions, following simple local assignments and helper argument bindings.
A construction with no execution remains a construction. No process outcome,
duration, stdout/stderr, or harness-internal shell command is inferred here.

**Supervision.** For supervisor S watching worker W, create one implicit look L:
`Supervision{Session: S, Target: W, Look: L}` and `L.Agent.Session = S`.
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
sprint package with an unexported zero-state implementation, an exported value
`var Workflow generatedWorkflow`, a compile-time `gimble.Workflow[Input]`
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

## 4. Application registration and run association

Keep the requested assembly:

```go
runtime, err := web.NewRuntime(ctx, project,
	web.WithWorkflow(sprint.Workflow),
	web.WithWorkflow(release.Workflow),
	web.WithPort(8080),
)
// Handle err before using runtime.
err = runtime.Run(ctx, sprint.Workflow, sprint.Input{Issue: issueFile})
```

Proposed signatures in web:

```go
func WithWorkflow[T any](w gimble.Workflow[T]) Option
func (r *Runtime) Run[T any](ctx context.Context, w gimble.Workflow[T], in T) error
```

`WithWorkflow` evaluates Name/Graph and stores only the graph in configuration.
Reject incomplete/diagnostic-bearing graphs, invalid references, name mismatch,
and duplicate registrations by ID or workflow name. Freeze registration before
serving. Configuration and the project observation registry hold graph values,
never runnable functions or `any` inputs. Keep private copies at read/write
boundaries; shared slices must not make a registered revision mutable.

Typed Run verifies registration and identity/digest before creating a run or
invoking the workflow. It creates the normal run/root scope, then calls
`w.Run(runCtx, in)` inside it. Wrong input types fail compilation. Unregistered
or mismatched graphs fail before workflow side effects. Replace the old web
Runtime.Run signature and migrate its callers; do not add a compatibility overload
or change the low-level `gimble.Run(ctx, name, body)` primitive.

Pass the verified graph reference through an internal context seam to the
low-level run creation. Add graph ID and digest to `RunStarted` and `RunRow`.
Record them in both lifecycle/project start records and in the normal observation
storage path. Existing `run.json`, reduced `observation.json`, durable deltas,
replay/fallback, and live snapshots must preserve the same fields. Do not implement
association as a live-only side map which disappears when the application restarts.

The existing observation registry should expose its registered graphs and resolve
a run's reference to one of them. A matching finished run resolves after restart;
a different/missing digest yields no compatible graph. A low-level unnamed-graph
run remains observable with no graph. Never select a graph by name alone or
reinterpret an old run with newer source. Historical graph archival is deferred;
if added later, serialize this same Graph value.

Add the smallest typed skgo queries needed to inspect registered graphs and a
run's matching graph before UI work. Return `workflow.Graph` values through the
existing generated boundary; regenerate bindings/TypeScript instead of authoring
a parallel DTO. Verify the actual serialized query result. No graph canvas,
workflow form, or generic input dispatcher is part of this task.

### Context lifetime

Retain `web.Runtime.ctx` as the runtime lifetime. Each Run gets a fresh child;
bridge synchronous caller cancellation with `context.AfterFunc`, preserving its
cause and cleaning up the callback. Handle an already-cancelled caller before
invoking work. A cancelled run context is never reused. Cancelling one run does
not cancel siblings; runtime cancellation reaches all runs.

HTTP-triggered work must start from the runtime lifetime, not a request context
which expires when the browser leaves. Preserve request cancellation for short
query/steer operations. Do not add an HTTP start endpoint just to demonstrate this;
exercise the lifetime rule with a small integration fixture if needed.

## 5. Delivery sequence

1. Land the graph type/invariants and generated typed workflow contract. Prove
   minimal generation from a separate module before growing extraction coverage.
2. Extract the actual sprint helper chain and structural fixtures. Replace only
   its fixed check loop with explicit `go vet ./...` and `go test ./...` calls and
   Set keys `"repository check 1"` / `"repository check 2"`. Preserve order,
   early-return behavior, pass/fail logic, per-check publication, and direct task
   values visible to the planner. Remove the now-unused mutable `checks` list;
   adapt its existing test instead of retaining a test-only production bypass.
3. Register generated graphs, migrate web Run and callers, persist graph references,
   and expose the typed inspection queries. Coordinate changed run fields with
   #173's observation work without implementing that issue's UI.
4. Demonstrate the claims below, then complete normal build/lint/Go/web and
   generation-drift checks. Report failures as failures, not as narrative success.

Likely implementation locations: new `workflow/graph.go`, root `workflow.go`,
`cmd/gimblegraph/`, and a small internal extraction package; existing
`internal/gimblelint/`, `internal/workflows/sprint/`, `cmd/sprint/main.go`,
`web/runtime.go`, `run.go`, `events.go`, `internal/observation/`, and typed query
declarations in `web/src/routes/`. Generated files are produced by their tools.
README explains the caller-facing generation and registration contract.

## 6. Behavioral acceptance

| Claim | Evidence the implementation must produce |
| --- | --- |
| Typed consumer works end to end | A separate Go module with its own unrelated Input generates from scratch, compiles, registers before any run, runs through Runtime, and resolves its recorded graph. A wrong Input type is rejected by the compiler. |
| The builtin graph reflects source | Inspect generated Sprint output against `Sprint`, `run`, `goalText`, `runTask`, `command`, and `git`: research/fork, outer rounds, inner planner/task repeat, coder/supervisor, task command, ancestor validator, both fixed checks, conditional commit, final validation/merge. Dry-run alternatives remain visible. |
| Flow is semantic | Fixtures demonstrate sequence, mutually exclusive branches, two launched group children and their join, repeat/exit including break/return, and continuation after join. There is no child-completion edge that serializes its sibling. |
| Resources and scopes are distinct | An ancestor session used in a child keeps its Owner while the child call has that child's Scope. A fork points to its origin and its own creation scope. |
| Both supervision targets survive | Worker W, look L, and higher look HL have attachments S→W/L and H→L/HL. A short live turn with an attachment and no look remains valid. |
| Commands retain meaning | Construct-only, blocking Run/Output/CombinedOutput, and Start→Wait examples produce distinct kinds and correct construction links; a command followed by a validator has a real sequence. Dynamic args remain expressions. |
| Identity and boundaries survive reuse | Same-named sites remain distinct, repeated tasks have one static template, two helper callers keep distinct continuations, and cross-scope edges retain their exact endpoints under a containment-based collapse/restore exercise. |
| Failure is inspectable and fails closed | Dynamic Set keys, map-dispatched workers/simple aliases, and unresolved structural targets produce anchored diagnostics and nonzero gates. A valid dynamic branch/data fixture passes. Failed regeneration cannot leave a stale clean graph presented as current. |
| Run association survives restart | Live and finished matching runs resolve to the registered graph; restart with a changed digest leaves the old run observable but unmatched. References survive durable snapshot, table, and supported replay paths. |
| Cancellation is independent | Two active runs share one runtime; cancel A and B remains active; another run can start after A ends. Runtime cancellation stops all active runs. An already-cancelled synchronous caller does not start workflow work. |
| Go remains the single graph model | Inspect the registered graph through a real skgo query and generated TypeScript/devalue boundary, comparing representative containment, flow endpoints, command payloads, and nested attachments. No hand-maintained JSON/TS graph model. |

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
