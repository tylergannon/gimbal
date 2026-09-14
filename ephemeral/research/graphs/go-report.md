# Go static-analysis feasibility for Gimble workflow graphs

Research date: 2026-09-14. This is an analysis of the current checkout and
the official Go/x/tools APIs; it does not change production code.

## Current versions and evidence

The checkout declares `go 1.27.1` and `golang.org/x/tools v0.50.0` as an
indirect dependency (`go-local-gomod-lines.txt:1-3,26-40`). The local toolchain
is `go1.27.1` (`go-local-go-version.txt:1`), and `go list -m -json` reports
x/tools v0.50.0, created 2026-09-08, with module GoVersion 1.26.0
(`go-local-x-tools-module.json:1-12`). `go list -m -versions` and the local
proxy version list (`go-source-x-tools-proxy-versions.txt`) end at v0.50.0;
the official pkg.go.dev module page also shows v0.50.0 and Published Sep 8,
2026 (`go-source-x-tools-module-latest.html:560-590`). Thus v0.50.0 is the
latest tagged x/tools version available in the checked proxy corpus; there is
no v0.51 claim to make.

The Go release history records Go 1.27.0 on 2026-08-19 and Go 1.27.1 on
2026-09-01 (`go-source-go-release-history.html:473-491`). The local toolchain
matches the latest release listed there.

The checked-in public API and source establish the graph's semantic seams:

- `scope.do` installs one scope pointer in a derived context, while
  `store` retrieves that pointer with `ctx.Value(scopeKey{})`; any derived
  standard context can therefore still address the same runtime scope
  (`go-local-scope-lines.txt:45-49,76-89,163-181`).
- `Set` and `SetJSON` are set-once per runtime scope instance; the second key
  panics (`go-local-scope-lines.txt:140-153,168-181`). A lexical function or
  SSA value is not itself the runtime scope identity.
- `Scope` passes a callback to a new child scope
  (`go-local-scope-lines.txt:127-138`). `Group.Go` gives each callback a child
  scope and `Wait` joins goroutines before ending the group
  (`go-local-group-lines.txt:58-98`).
- `Loop.Tasks` passes a `yield` callback into a fresh task scope on each
  dispatch (`go-local-loop-lines.txt:127-164`). The current sprint source uses
  both a direct `Scope` callback and `for ctx, task := range loop.Tasks`
  (`go-local-loop-lines.txt:127-140`; the probe output below confirms both
  shapes).
- `Session.Generate[T]` is a generic method on the concrete `Session` type
  (`go-local-session-lines.txt:14-21,63-74`). Sessions are adopted by the
  creating scope and closed when that scope ends (`go-local-session-lines.txt:37-47`),
  so a useful static graph must distinguish session ownership, turn calls, and
  scope lifetime.

## Minimal stack

1. Use `golang.org/x/tools/go/packages` with a deliberate mode containing
   `NeedName`, `NeedFiles`, `NeedCompiledGoFiles`, `NeedImports`, `NeedDeps`,
   `NeedSyntax`, `NeedTypes`, and `NeedTypesInfo`. `packages.Load` returns the
   parsed syntax, imports, `*types.Package`, and complete `*types.Info`; its
   package list contains the matched packages while dependencies are reachable
   through `Imports` (`go-local-x-tools-packages-doc.txt:1-43`). The load is
   build-tag, module, overlay, and driver aware, so the graph must record the
   exact load configuration and package patterns.
2. Make one `analysis.Analyzer` and require only the passes needed by a check.
   `analysis.Pass` supplies `Files`, `Fset`, `Pkg`, `TypesInfo`, source
   positions, diagnostics, and `ResultOf`; `Requires` is an acyclic dependency
   graph and `ResultType` names a pass result
   (`go-local-x-tools-analysis-doc.txt:20-155`). The current x/tools module
   includes `inspect`, `ctrlflow`, and `buildssa` (`go-source-x-tools-module-latest.html:1484-1510`).
3. Use `typeutil.StaticCallee`/`Callee` for call identity. They handle functions,
   methods, and instantiated generic functions; for an instantiated call they
   return its generic origin (`go-local-x-tools-typeutil-all.txt:1-17,39-47`).
   Raw `types.Info.Uses` is still useful for identifiers, but selector calls
   must account for `Selections`; `types.Info` also exposes `Defs`, `Uses`,
   `Instances`, `Scopes`, and `FileVersions` (`go-local-go-types-doc.txt:693-820`).
4. Use `inspector.Cursor` for nested source navigation. It is public in
   v0.50.0 and supports `Parent`, `Enclosing`, `Preorder`, child/sibling
   navigation, and source-position lookup (`go-local-x-tools-inspector-all.txt:55-140,150-238`).
   Keep an AST/source index for the exact call, literal, identifier, and
   declaration positions that the graph presents.
5. Add `buildssa.Analyzer` when value flow, goroutine statements, callback
   functions, or CFG dominance matters. It returns `SSA{Pkg, SrcFuncs}` and
   requires `ctrlflow`; it only builds an error-free package
   (`go-local-x-tools-buildssa-all.txt:1-29`). x/tools SSA exposes basic-block
   predecessors/successors and `Dominates`, `DomPreorder`, call instructions,
   `StaticCallee`, `Go`, anonymous functions, `Origin`, `TypeArgs`, and
   `TypeParams` (`go-local-x-tools-ssa-all.txt:199-240,384-480,895-1075,1139-1160`).
   Build with `ssa.InstantiateGenerics` when generic instances must be present;
   otherwise retain the generic origin and use `types.Info.Instances` for the
   source call.
6. Use `ctrlflow.CFGs.FuncDecl` and `FuncLit` when a syntactic function CFG is
   enough. Its `NoReturn` fact is useful for exit reasoning, but it does not
   model Gimble's dynamic scope pointer or prove that a workflow reaches a
   particular runtime state (`go-local-x-tools-ctrlflow-all.txt:1-46`).

## Tiny executable probe

The probe is [go-feasibility-probe.go](go-feasibility-probe.go), marked
`//go:build ignore` so it is excluded from ordinary package walks. It is run
explicitly with:

```text
go run ./ephemeral/research/graphs/go-feasibility-probe.go
```

The exact saved output is [go-feasibility-probe-output.txt](go-feasibility-probe-output.txt):

```text
go=go1.27.1
loaded=github.com/tylergannon/gimble,github.com/tylergannon/gimble/internal/workflows/sprint
generate=func[T github.com/tylergannon/gimble.Output](ctx context.Context, prompt string, opts ...github.com/tylergannon/gimble.AgentOption) (T, error) typeparams=1
exec-call=.../internal/workflows/sprint/sprints.go:159 CommandContext
exec-call=.../internal/workflows/sprint/sprints.go:161 Output
exec-call=.../internal/workflows/sprint/sprints.go:289 CommandContext
exec-call=.../internal/workflows/sprint/sprints.go:291 CombinedOutput
exec-call=.../internal/workflows/sprint/sprints.go:348 CommandContext
exec-call=.../internal/workflows/sprint/sprints.go:350 CombinedOutput
scope-callback=.../internal/workflows/sprint/sprints.go:96 callee=Scope params=ctx:Context
scope-callback=.../internal/workflows/sprint/sprints.go:101 callee=range.Tasks params=ctx:Context,task:Task
ssa-package=github.com/tylergannon/gimble/internal/workflows/sprint functions-with-bodies=18 blocks=169
```

This demonstrates current loading, generic-method type information, typed
command-call lookup, direct callback/range recognition, and SSA construction.
It is a feasibility check, not proof that every workflow is captured or that
every inferred edge is behaviorally exact.

## What the graph can prove usefully

For a first pass, report source anchored facts:

- Direct `gimble.Run`, `gimble.Scope`, `Group.Go`, and `Loop.Tasks` callback
  sites, including the callback parameter type and source position.
- Direct `Session.Generate` calls, their generic output origin/type arguments,
  prompt expression, schema-bearing type, and attached option expressions.
- Direct `gimble.NewSession`, `Fork`, `WithSupervisor`, and `Steer` calls. A
  `WithSupervisor` edge can be drawn from the option
  expression to a worker call when both are syntactically visible, with a
  confidence/unknown marker when they are hidden behind an option slice or
  helper.
- Direct `os/exec` calls with the callee object and argument expressions. A
  constant command name or literal shell text is source evidence; a variable,
  helper, or shell expansion is only an unresolved command edge. The probe
  finds all six typed `os/exec` call sites in the sprint workflow.
- CFG blocks and SSA call/goroutine instructions inside each source function.
  `Go` instructions identify a goroutine creation site. `DomPreorder` and
  `Dominates` support a bounded same-function ordering check, while preserving
  the exact source position for review.
- Facts attached to exported objects or packages can summarize a package's
  declared workflow entry points or stable function summaries. Facts must be
  serializable and are only exportable for the current package/objects; they
  are imported from dependencies (`go-local-x-tools-analysis-doc.txt:185-261`).

## Set/session scoping: useful first pass versus exact proof

Issue #162's proposed checks are implementable as a useful diagnostic first
pass, but its same-SSA-value plus dominance rule is not a complete proof of
Gimble's set-once contract ([go-local-issue-162-lines.txt:1]). It should be
described as “definite or likely misuse found” and tested against negative
cases, not as “no diagnostics proves safe.”

The first pass can soundly flag many definite local cases:

- Resolve `Set`/`SetJSON` by `typeutil.Callee` and inspect constant string keys.
- Within one source function, match calls whose context value is the same
  SSA value and whose call instructions are ordered in one basic block or one
  block dominates another. Same-block ordering needs instruction indices or
  source positions; `Dominates(b,b)` alone does not establish call order.
- Flag a call in a loop when the context is defined outside the loop and a
  same-key write is reached on an iteration. This is a conservative repeated
  execution warning.
- Recognize direct scope callbacks and treat the callback parameter as the
  child context. For `range.Tasks`, Go 1.23's iterator syntax and x/tools SSA's
  synthetic yield function make the fresh per-iteration callback visible.
- Flag a scope write inside every raw `go` function literal, including one
  nested in a `Group.Go` callback. The `Group.Go` submission itself is the
  modeled joined boundary; a raw goroutine launched by that callback can still
  outlive the callback's child scope. This is a lifetime warning requiring a
  manual or runtime check for helper indirection.
- Flag the reserved `"task"` write in a `Loop.Tasks` body and direct
  `context.Background`/`context.TODO` writes. The latter needs to include
  derived no-scope contexts in a later pass.

The exact contract has important counterexamples to same-value identity:

- `ctx2 := ctx` and parameter passing can yield different SSA values while
  retaining the same runtime scope pointer.
- `context.WithCancel(ctx)`, `context.WithValue(ctx, ...)`, and other derived
  contexts retain the `scopeKey` value, so `Set(ctx2, ...)` still writes the
  parent runtime scope even though the SSA value is a new context value.
- A call through a helper can write the caller's scope or an unknown scope;
  facts or function summaries are needed to carry that contract across
  packages.
- Two calls in different branches may both execute on different paths. A
  dominance-only rule misses partial-path duplicates. Reporting every pair
  would produce false positives for mutually exclusive branches; proving the
  path intersection requires CFG reachability/path reasoning and still does
  not solve context aliasing.
- A loop body can execute zero or many times. A pre-loop write plus a body
  write is conditional, while a body write can repeat; the diagnostic should
  say which bounded condition was found.
- `Group.Go` and `Loop.Tasks` create runtime scopes inside library code. The
  caller's callback parameter is the right source-level seam, but closure
  capture, stored callbacks, aliases, and reflection can hide the relation.
- `Group.Wait` is the join boundary. A static pass can find direct `Wait`
  calls and missing obvious paths, but aliases, early returns, panic/recover,
  and helper calls prevent an exact proof of “wait on every exit path” without
  a deliberately bounded interprocedural analysis.

For this reason, the recommended initial linter reports definite local misuse,
keeps unknown/indirect cases visible, and uses live run events as the runtime
authority for the graph. Dynamic Set keys remain analyzable at runtime and are
not rejected merely because a static pass cannot resolve them. The linter
should not claim lifetime safety, race freedom, or complete scope identity
from an SSA graph alone. This incomplete-first-pass boundary is the current
dated decision (2026-09-14), rather than a reason to add hard analysis gates.

## Control, supervision, and command graph limits

`ctrlflow` is syntactic and package-local. SSA gives a resolved value graph and
CFG, but function bodies can be absent for external packages. Call graphs are
also approximations: the x/tools contract defines a sound graph as an
over-approximation of all dynamic calls (`go-local-x-tools-callgraph-all.txt:1-30`).

Available algorithms in v0.50.0:

- `static` follows only static calls, useful for direct Gimble API and
  `os/exec` calls but incomplete for interface dispatch
  (`go-local-x-tools-callgraph-static-all.txt:1-20`).
- `cha` is conservative over interfaces/types and is sound for partial
  programs, at the cost of spurious edges for uninstantiated types
  (`go-local-x-tools-callgraph-cha-all.txt:1-20`).
- `rta` starts from complete-program roots and reaches a fixed point; it
  requires SSA built with `InstantiateGenerics`, and excludes reflection edges
  (`go-local-x-tools-callgraph-rta-all.txt:1-48`).
- `vta` propagates SSA types and function literals, but is explicitly
  experimental and still only sound modulo reflection/unsafe when its initial
  graph is sound (`go-local-x-tools-callgraph-vta-all.txt:1-60`).

Use static call identity plus direct AST facts for the first graph. Add CHA or
VTA only when an actual workflow needs interface/higher-order expansion; do
not present their over-approximation as the exact runtime supervisor or
session graph. Supervisor attachment is especially runtime-shaped: the option
is data passed to `Generate`, and `apply` later turns it into a supervisor
session/turn. Static syntax can show an option was supplied, but only events
show whether a look ran, an objection was produced, or a steer landed.

## Recent Go changes that matter

- Go 1.23 added `for range` over iterator functions
  (`go-source-go1.23-release-notes.html:461-471`) and `go/ast.Preorder`
  (`go-source-go1.23-release-notes.html:711`). It also added the stdversion vet
  check (`go-source-go1.23-release-notes.html:519-523`). This is why
  `Loop.Tasks` can be modeled as a source callback and why version-aware
  analysis is already part of the Go toolchain.
- Go 1.26 rebuilt `go fix` on the same Go analysis framework as `go vet`, so a
  diagnostic can eventually carry a `SuggestedFix` and participate in fix
  tooling (`go-source-go1.26-release-notes.html:502-518`). A Set linter should
  remain report-only until a mechanically safe rewrite exists.
- Go 1.27 added generic methods, with the restriction that interface methods
  cannot declare type parameters or be implemented by generic methods
  (`go-source-go1.27-release-notes.html:456-466`). This directly enables the
  current `Session.Generate[T]` API. Go 1.27 also makes `go test` invoke
  stdversion by default (`go-source-go1.27-release-notes.html:496-500`).

## Running in generation and CI

`go generate` is explicit and never run by `go build`/`go test`; it processes
directives sequentially by package/file and sets the `generate` build tag
(`go-local-go-help-generate.txt:1-55`). It is suitable for a deliberately
named graph snapshot generator, but the snapshot must be treated as generated
source/data and checked for drift.

`go vet` is a diagnostic heuristic and does not guarantee correctness
(`go-local-cmd-vet-doc.txt:1-18`). The current command supports
`-vettool`, `-fix`, and `-diff`; alternative vet tools should use
`unitchecker.Main`, while a standalone `singlechecker.Main` tool can run its
own package-pattern command (`go-local-go-help-vet.txt:1-32`). Therefore:

- A custom analyzer that should plug into `go vet -vettool` must use the
  unitchecker protocol (or a driver that supports it), be built as a binary,
  and be passed to `go vet -vettool=...`.
- A `singlechecker` command is appropriate for `go tool <name> ./...` or a
  direct analyzer CLI. The existing `tool` block demonstrates the repository's
  `go tool` convention (`go-local-gomod-lines.txt:42-48`). Do not call a
  singlechecker binary a vettool without the unitchecker protocol.
- CI should run package loading/analyzer tests with `analysistest` and then the
  chosen analyzer command against explicit workflow packages. Keep ordinary
  `go vet ./...` and `go test ./...` as separate gates; green gates alone do
  not prove runtime graph completeness.

## Corpus/index decision

The official API corpus plus the local x/tools source-derived docs, release
notes, module metadata, current API excerpts, issue #162, and the executable
probe are small enough for direct retrieval. A semantic index is not
warranted for this leaf; the parent integration may index the combined research
corpus if multiple leaves make retrieval materially expensive.
