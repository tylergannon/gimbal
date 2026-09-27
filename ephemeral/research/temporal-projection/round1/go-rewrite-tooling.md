# Go source-to-source rewriting and subset enforcement, as of 2026-09-27

Scope: can Gimbal mechanically project a workflow function (ordinary Go
against `github.com/tylergannon/gimbal`) into a Temporal workflow, and what
tooling exists today to do the rewrite and to keep the input inside a safe
subset. Tooling only, not a projection design. Claims are marked
**Verified** (checked against a live source this pass, 2026-09-27) or
**Inferred** (reasoned from verified facts, not directly observed).

## Findings

### 1. dave/dst (Decorated Syntax Tree)

1.1 **Verified.** Latest version `v0.28.0`, published 2026-09-10 — 17 days
before this pass. Before that, `v0.27.4` (2026-04-14), and before that a
long gap back to 2022 (`v0.27.3`, 2022-12-09) and 2020. Source: pkg.go.dev
version list for `github.com/dave/dst`.

1.2 **Verified.** The `v0.27.4 -> v0.28.0` diff is a modernization pass, not
a feature pass: bumps vendored `golang.org/x/tools` from `v0.1.12` to
`v0.50.0` (the exact line Gimbal pins), moves CI to GitHub Actions testing
"the two currently supported Go releases (1.26 and 1.27)," adds support for
`ast.File.GoVersion`, fixes gofmt comment/blank-line bugs. No commit
mentions generics, type parameters, or `IndexListExpr`. Source: GitHub
compare `dave/dst@v0.27.4...v0.28.0`.

1.3 **Verified.** Generics support is older and thinner than the fresh
release date suggests: issue #63 ("Dst doesn't support generics") closed
2022-05-29, same day as `v0.27.0`. Issue #77 (panic decorating a call to a
generic function) closed 2025-01-19. Issue #74 (generics broken under
`ResolveLocalPath`), opened 2023-06-18, is **still open**. Four years of
thin, reactive maintenance with at least one known rough edge. Source:
`dave/dst` issues, query "generic".

1.4 **Verified the shape, unverified the specific case.** dst decorates
`*ast.IndexExpr`/`*ast.IndexListExpr` (single/multi type-argument
instantiation), and Gimbal's own extractor already special-cases both when
unwrapping a method value from a generic instantiation
(`internal/generate/read.go`, `receiver()`). What no document confirms is
dst's behavior on a **method declaration with its own type parameters** —
legal only since Go 1.27 — the shape behind
`func (s *Session) Generate[T Output](...)`. No dst issue, changelog, or
release note names this case. **Inferred**: `ast.FuncType.TypeParams` is
the same field Go has populated for ordinary generic functions since 1.18,
and dst's decorator/restorer is generated from `go/ast` node shapes
(`decorations-node-generated.go`), not a hand-maintained legality list — so
it plausibly already round-trips this without new work. This is inference,
not verification.

1.5 **Not run: the one-afternoon test that resolves 1.4.** Write a `go
1.27` package: a concrete type with a generic method (mirroring
`Session.Generate[T Output]`), a caller using an explicit type argument
(`s.Generate[Plan](ctx, p)`) and one relying on inference. Load with
`decorator.Load`, print straight back with `decorator.Print`, diff
byte-for-byte including comments/blank lines. Repeat after renaming the
method and changing the type argument, to check a real edit, not just a
round trip. Small enough for an afternoon; settles 1.4 either way.

1.6 **Verified.** Typed resolution: `decorator.NewDecoratorWithImports`
takes a `resolver.DecoratorResolver`; dst ships `goast.New()` (AST-only,
documented to panic on a dot-import) and a `go/packages`/`go/types`-backed
path via `decorator.Load` — equivalent in spirit to Gimbal's own
`packages.Load` with `NeedTypes|NeedTypesInfo`. Source: `dave/dst/decorator`
docs, `decorator/resolver/goast/resolver.go`.

1.7 **Verified.** Beyond the dot-import panic, the README itself credits an
outside contributor with "taking on the task of adding generics
compatibility to dst," with no further technical detail — read as
tacked-on, not designed-in.

1.8 **Maintenance, verified.** 1.4k stars, 64 forks, 11 open issues, 221
commits on master. History is bursty: dormant 2020-2022 and 2023-2026,
short bursts (mid-2022; again in 2026 around Go 1.26/1.27 compatibility).
Consistent with "revived when a Go release breaks it," not ongoing feature
investment. It remains the only full-fidelity decorated-rewrite tool in the
ecosystem, so "thinly maintained, no substitute" is the honest read.

### 2. Alternatives to dst

2.1 **Verified.** `go/ast`+`go/printer` has no comment-to-node model, only a
flat position-sorted comment list printed by proximity. golang/go#20744,
"free-floating comments are single-biggest issue when manipulating the
AST," is still open. `astutil.Apply` gives ergonomic pre/post-order
rewriting over plain `go/ast` but inherits the same problem, and its own
documented gotcha is that replacing a node does not re-walk the
replacement — a caller must call `Apply` again on it.

2.2 **Verified.** `ast.PreorderStack` (Go 1.25, golang/go#73319) is `Inspect`
plus the enclosing-node stack at each callback — a traversal convenience
(the kind of thing `gimballint`'s hand-built `si.parents` map already does),
not a comment-fidelity or rewriting improvement.

2.3 **Verified.** `x/tools/go/ast/inspector` (already a Gimbal dependency in
`gimballint`) is read-only, built for `go/analysis` passes; not a rewriter,
no comment model beyond plain `go/ast`.

2.4 **Verified.** `gofmt -r` is still present and documented; nothing in
2026 release notes signals removal. Untyped, single-expression/statement
pattern only — cannot express "any call to `Session.Generate` regardless of
receiver." `gofumpt` dropped its own `-r` "in favor of gofmt -r," and as of
`gofumpt` 1.27.0 it is now a **fork of `cmd/gofmt` itself**, requiring Go
1.26+, tracking the real formatter's internals rather than reimplementing
on `go/printer`.

2.5 **Verified.** `go/analysis` diagnostics carry `SuggestedFixes` (concrete
text edits); `analysistest.RunWithSuggestedFixes` checks them. This backs
`go fix`, rebuilt for Go 1.26 into "the home of Go's modernizers,"
including a `//go:fix inline` source-level inliner for project-authored API
migrations. Go 1.27 added 4 more modernizers, dropped `fmtappendf`, renamed
`waitgroup`->`waitgroupgo`. Gimbal's `go.mod` already pins the `modernize`
tool. Typed (`go/types`), but inherits the comment-fidelity caveat (2.1)
whenever a fix moves code rather than editing a text span in place — most
modernizers are single-span and sidestep it.

2.6 **Verified.** `x/tools/refactor/eg` ("example-based refactoring",
typed, before/after function pairs) still exists; no deprecation notice
found, but no evidence of recent activity either. Treat as
present-but-dormant; its "one API's call sites, matched by pattern" fit for
`context.Context`->`workflow.Context` is plausible but unconfirmed against
generic code.

2.7 **Verified.** `gopls` ships fixed `refactor.rewrite.*` LSP actions
(fillStruct, invertIf, moveParam, addTags, implementInterface, etc.),
reachable via `gopls codeaction -exec -kind`. Hand-written, editor-scoped
refactorings, not a programmable rewrite engine — not a substrate for this
project, only a reference for how gopls edits while preserving comments.

2.8 **Verified.** `uber-go/gopatch` matches/transforms at the AST level via
a patch-file syntax (Unix `patch`-like), explicitly designed to work on
code that may not yet compile. That design choice means it is untyped —
same tier as `gofmt -r` but richer patterns. Latest `v0.4.0` (2024-04-03),
self-described "beta," roughly annual cadence with one nine-month gap:
maintained, not actively developed.

2.9 **Verified.** `comby` is language-agnostic structural search/replace
(syntax-skeleton aware — balanced delimiters, strings, comments — not a
real Go parser): no `go/types`, no notion of method sets, generics, or
import identity. Issues opened into 2026 (Apr/May), not abandoned, but
wrong tool for anything needing Go semantics.

2.10 **Verified.** Semgrep has GA Go support; its Pro Engine adds
cross-file (interprocedural) Go analysis. OSS rule matching is
pattern-based over a parse tree, not `go/types`-typed — no method-set or
generic-instantiation resolution. `dgryski/semgrep-go` targets both semgrep
and ruleguard. Good for linting/first-draft rules, not a typed-rewrite
substitute.

2.11 **Verified.** `ruleguard` (in `go-critic`, via `golangci-lint`) is
"gogrep adapted for CI," DSL-based but `go/types`-aware (can match "type
implements interface X") — more typed than Semgrep OSS. It is a
pattern-matcher/linter, not a rewriter; fixes, where offered, ride the same
`SuggestedFixes` mechanism as `go fix`. One recent stability blip inside
`golangci-lint` (disabled v1.54.0, re-enabled v1.54.1) reads as ordinary
integration churn.

### 3. Subset enforcement

3.1 **Verified from source.** `internal/gimballint` is already a
`go/analysis` analyzer requiring both `inspect.Analyzer` and
`buildssa.Analyzer`, mixing two techniques by rule shape: a hand-built
syntax index (`indexSyntax` — parent links, per-unit "contains a Gimbal
operation" reachability by fixed-point over direct, source-visible calls
only, explicitly *not* whole-program or pointer analysis per its own doc
comment) for GIMBAL101 (no dynamic worker dispatch); and `go/ssa` (via
`buildssa.Analyzer`) for GIMBAL103 (one `Set`/`SetJSON`/`Check` write per
scope key), walking SSA blocks, dominance between call sites sharing a
context value and key, hand-computed natural loops (`naturalLoops`, not a
stock x/tools API), and Go's synthetic range-over-func yield functions for
loop/per-iteration reuse. Two analysis styles already coexist, chosen per
rule.

3.2 **Verified from source.** `go/analysis` Facts are unused in
`gimballint` today — everything is single-package. Gimbal's domain is
package-local by convention: a call into a package that itself imports
gimbal is explicitly *not* followed, only reported (`importsGimbal` in
`internal/generate/read.go`, mirrored in `gimballint`'s reachability walk).
The harder cross-package problem is sidestepped by rule, not solved.

3.3 **Verified.** Temporal's `workflowcheck` is the closest existing
analog. `go/analysis`-based; its `determinism` package's `Checker`/`Config`
pair: `IdentRefs` (known-nondeterministic identifiers, e.g. `time.Now`,
overridable), `AcceptsNonDeterministicParameters` (functions whose
nondeterminism depends on their own arguments), `SkipFiles`, and
`EnableObjectFacts` — turning on **`go/analysis` Facts** so a finding is
exported per object/package (`*NonDeterminisms`, `*PackageNonDeterminisms`)
and consumed by dependent packages' passes via Facts' standard vertical
dependency (an analyzer requires its own pass to have already run on every
import). Walks typed AST, not SSA. Documented gap: cannot see through
interface dispatch or bare function values; explicitly says it "will not
catch all cases of non-determinism such as global var mutation" and is
"just a helper." False positives on rarely-nondeterministic stdlib calls
are an accepted cost, escaped with `//workflowcheck:ignore`.

3.4 **Verified — the structural fact underlying GIMBAL101 and any future
dispatch rule.** `gimballint` distinguishes a generic instantiation
(`Generate[Plan]`) from a real collection lookup (`workers[kind]`) purely
via `pass.TypesInfo.Instances` on the base expression
(`isGenericInstantiation`). This is the same generics-vs-indexing
disambiguation dst's decorator must get right structurally (1.4), from the
types side rather than the syntax-preservation side. A projection needs
both.

3.5 **Verified.** `x/tools/go/callgraph` ships three whole-program,
`go/ssa`-based algorithms of increasing precision and cost: **CHA** (class
hierarchy — conservative, sound on a library with no `main`, many spurious
interface edges); **RTA** (rapid type analysis — builds the "implements"
relation as it discovers code reachable from `main`; requires a whole
program, cheaper and more precise than CHA for a real binary); **VTA**
(variable type analysis — refines an initial, typically CHA, graph via
interprocedural type flow; most precise, most expensive). None integrates
with `go/analysis` Facts — separate, heavier `go/ssa`-based API over an
`ssa.Program`. RTA fits "what can reach `Generate` transitively through
interfaces/function values" if ever run over a compiled binary; CHA is the
library-only fallback, over-approximating interface calls.

### 4. Code-generation patterns: sibling package vs. in-place rewrite

4.1 **Verified, by pattern.** The dominant idiom for "read dialect A, emit
compilable dialect B" is a generated **sibling package** via `go generate`,
never in-place rewrite under a build tag: sqlc emits a separate typed
query package (its Go backend, `sqlc-gen-go`, is now an externally pluggable
generator); ent (`entc/gen`) emits a generated `ent` subpackage; mockgen
emits a mock type into a sibling file/package from a `//go:generate`
directive next to the interface. None rewrites the file that named them.
Matches Gimbal's own `internal/generate` (reads a workflow, emits a graph
plus generated registration code; per `AGENTS.md`, `generated/` is
`go generate`-written, never by hand) — the existing precedent is already
"read source, emit a sibling."

4.2 **Inferred, from Gimbal's own constraints.** For a Temporal projection,
sibling-package is the only workable shape regardless of external
precedent: `AGENTS.md` states "the page's Go imports `gimbal`, so `gimbal`
never imports the page," and the brief requires "the workflow source itself
must stay simple and locally runnable." An in-place rewrite under a build
tag means the checked-in source imports both `gimbal` and Temporal's
`workflow` package, or two build-tag-gated copies of one function body —
either defeats "stay simple," or reintroduces the dual-path shim
`AGENTS.md` argues against elsewhere in spirit. A generated sibling package
(built from the same `workflow.Graph` `internal/generate` already extracts)
leaves the original untouched and lets only the Temporal-side package
import `go.temporal.io/sdk/workflow`. This is closer to `wire`'s pattern
(hand-written intent under `//go:build wireinject`, paired with a generated
`wire_gen.go` that never touches the injector's declared signature) than to
sqlc/ent/mockgen, since Gimbal's case transforms a function that already
has a body rather than scaffolding from a declarative spec.

4.3 **Verified, load-bearing for a "hypothetical rewrite" step.**
`packages.Config.Overlay` (path -> in-memory contents) lets `packages.Load`
type-check a package with files replaced in memory, nothing touching disk —
and Gimbal's `internal/generate/graph.go` already threads an `overlay`
parameter through its internal `extract` (present in the signature, unused
by the exported `Extract` today). Use: generate a candidate rewritten file
in memory, load it as an overlay on the real module, get real `go/types`
checking of the hypothetical result — including whether rewritten calls
still resolve, generic instantiation still succeeds, an import swapped from
`context` to `go.temporal.io/sdk/workflow` doesn't collide — before writing
anything to disk. Already half-wired in Gimbal's own code.

4.4 **Verified.** Emission choices: `text/template`+`go/format`, or
`dave/jennifer` (fluent Go API, renders correctly formatted/import-qualified
Go directly). Jennifer's README confirms Go 1.18+ generics (`Types()`,
`Union()`, `any`/`comparable`); repo shows continuing low-volume activity
(255 commits, open issues/PRs). Trade-off: templates read like their own
output but produce unreadable diffs and get fragile as they nest; jennifer
is mechanically correct (never invalid syntax, never a missing import) at
the cost of generation code that looks nothing like its output. Neither
preserves *source* comments from the workflow being projected — that
belongs to the reading/rewriting stage (dst/`go/ast`), not emission;
comments would need deliberate re-attachment if they must survive.

### 5. The specific rewrite: `context.Context` -> `workflow.Context`

5.1 **Verified from source — the central finding.** Gimbal's analyzer
already enumerates every shape a context reaches a call that matters: a
direct `Scope`/`Group.Go`/`Iterate` function-literal callback (bound via
`syntaxInfo.boundaries`, keyed on the callback's first parameter object); a
package-local helper called with a context argument, inlined at the call
site (`internal/generate/expr.go`'s `inline`, rebinding session parameters
positionally and walking the callee's block in place); a
`PromiseLoop.Tasks`/`Iterate` range, boundary at the yielded variable.
Everything else is a hard stop, reported rather than guessed: a call
through a function value (`takesContext` against an unresolved
`typeutil.StaticCallee`), a call into a package that itself imports gimbal,
a recursive helper, a `select`/`go`/`defer`/type switch holding an
operation, a callback body that is neither a literal nor a same-package
function. GIMBAL101 independently forbids the two constructs that would
otherwise defeat a type-directed rewrite: a worker selected from a
collection, and one taken as an opaque parameter. Together, the *inputs a
projection has to handle* are already a closed, enumerable set — direct
calls and same-package inlining, nothing dynamic — which is what makes
"find every call chain from the entry" tractable by a syntax-level,
`go/types`-informed rewrite (dst) rather than requiring `go/callgraph`'s
heavier RTA/VTA (3.5).

5.2 **Inferred.** What would break a type-directed rewrite, in Gimbal
terms: (a) a call through a function value or interface method — already
forbidden by GIMBAL101/the extractor, so absent from a lint-clean workflow;
(b) a call into a helper in a *different* package — already refused and
reported by `importsGimbal` reachability, so also absent from a lint-clean
workflow, though this is stricter than the rewrite problem strictly needs:
a helper in another package that does *not* import gimbal but threads a
`context.Context` through is still walkable by `go/types` — Gimbal's rule
trades that possibility away for authoring clarity; (c) a generic method
call whose type argument is only inferred, not explicit
(`s.Generate(ctx, p)`, inference alone picking `Plan`) — dst should still
print this correctly since it decorates the AST as parsed rather than
re-deriving instantiation, but a rewriter that needs the concrete type
(e.g. to pick a per-type helper) must consult `pass.TypesInfo.Instances`,
exactly as `gimballint`'s `isGenericInstantiation` already does (3.4). This
is the one genuinely new wrinkle Go 1.27 methods add beyond generic
functions: the receiver expression still has to resolve to a session
binding the way `internal/generate/read.go`'s `receiver()`/`binding()`
already do, unaffected by the method itself being generic.

## Comparison table

| Tool | Typed (go/types)? | Preserves comments? | Maintained (2026-09-27)? | Latest release / activity |
|---|---|---|---|---|
| dave/dst | Yes, via `decorator.Load` or an AST-only resolver | Yes — its purpose | Thin, reactive; generics has an open gap (#74) | v0.28.0, 2026-09-09/10 |
| go/ast + go/printer + astutil.Apply | Only if caller adds go/types | No — golang/go#20744 open | Yes (stdlib/x/tools core) | Tracks each Go release |
| gofmt -r | No | Best-effort via go/printer | Yes (stdlib) | Ships with Go |
| go fix / modernize (SuggestedFixes) | Yes | Only for single-span edits | Yes, growing (Go 1.26 rebuild, +4 fixers in 1.27) | Go 1.27, 2026 |
| x/tools/refactor/eg | Yes | Inherits go/printer limits | Present, low visible activity | Unclear |
| gopls refactor.rewrite.* | Yes | Editor-grade | Yes | Rolling with gopls |
| uber-go/gopatch | No (works on non-compiling code by design) | Generally, via go/printer | Yes, slow cadence | v0.4.0, 2024-04-03 |
| comby | No (language-agnostic structural) | Structure- not comment-aware | Yes | Issues active into 2026 |
| semgrep (Go) | No in OSS; Pro adds cross-file | Not applicable | Yes | Go GA current |
| ruleguard / go-critic | Yes (types-aware DSL) | N/A, linter | Yes | Ongoing |
| gofumpt | N/A (formatter) | Yes | Yes; forked from cmd/gofmt as of 1.27.0 | Tracks Go 1.26+ |
| jennifer | N/A (emitter, not ingest+preserve) | N/A | Yes, low-volume | Ongoing, generics supported |
| text/template + go/format | N/A | N/A | Yes (stdlib) | Standard |
| go/analysis Facts (custom) | Yes | N/A, diagnostics | Yes (x/tools core) | v0.50.0 pinned by Gimbal |
| go/callgraph (cha/rta/vta) | Yes, via go/ssa | N/A | Yes (x/tools core) | v0.50.0 pinned by Gimbal |
| temporal workflowcheck | Yes (typed AST + Facts) | N/A, diagnostics | Yes (Temporal SDK contrib) | Tracks sdk-go |

## Recommendation

**Reading/rewriting**: use dst via `decorator.Load` (the same
`go/packages` loading style `internal/generate` already uses, including its
already-wired but unused overlay hook). It is the only tool here that both
understands Go's generic syntax at the node level and preserves comments
through an edit; every syntactic alternative (gofmt -r, gopatch, comby) is
untyped and would need to re-derive the receiver-is-a-session-binding and
generic-instantiation-vs-indexing distinctions Gimbal's own `go/types`-based
code already computes. Verify with the 1.5 afternoon test before treating
dst as load-bearing. If it finds a gap, fall back to plain `go/ast` +
`go/printer` and accept comment disturbance — tolerable, since the
projected output is a generated sibling package (4.1-4.2), not hand-edited
source.

**Subset enforcement**: extend `gimballint` rather than adopting
`workflowcheck` wholesale — Gimbal's analyzer already covers the same
ground (direct-call reachability, syntax index, SSA for ordering). The one
technique worth borrowing outright is Facts-based cross-package propagation
(3.3), since "a call into a package that imports gimbal" is currently
rejected rather than followed (3.2); model any extension on
`workflowcheck`'s `NonDeterminisms`/`PackageNonDeterminisms` pair. Reach for
`go/callgraph` RTA/VTA (3.5) only if a future rule needs interface dispatch
or function-value reasoning that GIMBAL101 currently forbids outright —
for today's subset, syntax-level reachability is sufficient and cheaper.

**Code generation**: generate a sibling package from the same
`workflow.Graph` `internal/generate` already extracts; never rewrite the
workflow file in place or under a build tag — both the universal idiom
(sqlc/ent/mockgen/wire) and the only shape consistent with "gimbal never
imports the page." Use `packages.Config.Overlay` to type-check the
candidate before writing it to disk (plumbing already half-built). Prefer
`jennifer` over `text/template` once the target shape stabilizes; templates
are fine for first drafts while that shape is still being discovered.

## Risks

- **dst's generic-method fidelity is unverified**, and it is exactly the
  syntax Gimbal's public API depends on (`Session.Generate[T Output]`). A
  gap found by 1.5 narrows the promise from "comment-preserving rewrite" to
  "best-effort," which matters less for a purely generated, never-hand-
  edited sibling package than it would for hand-maintained source.
- **dst is thinly maintained.** A future Go AST-shape change dst does not
  track promptly would block the pipeline until upstream catches up; worth
  watching, worth vendoring/forking (as `sirkon/dst` has already done) if a
  gap opens and does not close.
- **Gimbal's subset enforcement is intentionally not whole-program**, by
  both analyzers' own doc comments. A Temporal projection needing stronger
  guarantees than "no dynamic dispatch found by direct-call reachability"
  is new work (Facts, possibly callgraph), not reuse — a lint-clean
  workflow today is not automatically "provably reachable by a
  type-directed rewrite" for constructs not yet checked (5.2b: a
  same-module helper that doesn't itself import gimbal is walkable today by
  the extractor's inliner but fenced by no lint rule against later calling
  back into gimbal).
- **Untyped tools (gopatch, gofmt -r, comby) are tempting for a demo and
  wrong for the real pipeline** — they will "succeed" on subtly wrong
  results (rewriting an unreachable function, missing one reachable only
  via inlining) because none shares Gimbal's `go/types`-informed definition
  of reachable. Discard any such prototype once the typed approach is
  validated, don't harden it.
- **The brief's Temporal architecture (EKS namespace, GitHub App clones,
  per-project worker images) is entirely out of this note's scope** —
  nothing above validates or challenges that part of the brief.

## Sources

All read 2026-09-27; version/date given inline above at first mention.

- pkg.go.dev, `github.com/dave/dst` versions — `https://pkg.go.dev/github.com/dave/dst?tab=versions`
- GitHub, `dave/dst` compare `v0.27.4...v0.28.0` — `https://github.com/dave/dst/compare/v0.27.4...v0.28.0`
- GitHub, `dave/dst` commits — `https://github.com/dave/dst/commits/master`
- GitHub, `dave/dst` releases — `https://github.com/dave/dst/releases`
- GitHub, `dave/dst` README — `https://github.com/dave/dst`
- GitHub, `dave/dst` issues (#63, #59, #74, #77, #85) — `https://github.com/dave/dst/issues`
- pkg.go.dev, `github.com/dave/dst/decorator` — `https://pkg.go.dev/github.com/dave/dst/decorator`
- GitHub, `dave/dst/decorator/resolver/goast/resolver.go` — `https://github.com/dave/dst/blob/master/decorator/resolver/goast/resolver.go`
- golang/go #20744, free-floating comments — `https://github.com/golang/go/issues/20744`
- golang/go #73319, `ast.PreorderStack` — `https://github.com/golang/go/issues/73319`
- Go 1.25/1.26/1.27 release notes — `https://go.dev/doc/go1.25`, `https://go.dev/doc/go1.26`, `https://go.dev/doc/go1.27`
- Go blog, "Using go fix to modernize Go code" — `https://go.dev/blog/gofix`
- Go blog, "Generic Methods" — `https://go.dev/blog/generic-methods`
- golang/go #77273, "spec: generic methods for Go" — `https://github.com/golang/go/issues/77273`
- pkg.go.dev, `golang.org/x/tools/go/analysis` — `https://pkg.go.dev/golang.org/x/tools/go/analysis`
- pkg.go.dev, `golang.org/x/tools/go/analysis/passes/modernize` — `https://pkg.go.dev/golang.org/x/tools/go/analysis/passes/modernize`
- pkg.go.dev, `golang.org/x/tools/cmd/eg` / `refactor/eg` — `https://pkg.go.dev/golang.org/x/tools/cmd/eg`
- GitHub, `golang/tools`, `refactor/eg/eg.go` — `https://github.com/golang/tools/blob/master/refactor/eg/eg.go`
- GitHub, `golang/tools`, `gopls/doc/features/transformation.md` — `https://github.com/golang/tools/blob/master/gopls/doc/features/transformation.md`
- go.dev tip, gopls transformation features — `https://tip.golang.org/gopls/features/transformation`
- GitHub, `uber-go/gopatch` CHANGELOG — `https://github.com/uber-go/gopatch/blob/main/CHANGELOG.md`
- pkg.go.dev, `github.com/uber-go/gopatch` — `https://pkg.go.dev/github.com/uber-go/gopatch`
- GitHub, `comby-tools/comby` issues — `https://github.com/comby-tools/comby/issues`
- comby.dev — `https://comby.dev/`
- Semgrep docs, Go support — `https://semgrep.dev/docs/languages/go`, `https://docs.semgrep.dev/languages/go`
- Semgrep blog, Go in Pro Engine — `https://semgrep.dev/blog/2023/golang-in-pro-engine/`
- GitHub, `dgryski/semgrep-go` — `https://github.com/dgryski/semgrep-go`
- GitHub, `golangci-lint` issues #3136/#3586/#3107 (go-critic/ruleguard) — `https://github.com/golangci-lint/golangci-lint/issues/3136`
- quasilyte.dev blog, ruleguard — `https://www.quasilyte.dev/blog/post/ruleguard/`
- GitHub, `mvdan/gofumpt` README, issue #275 — `https://github.com/mvdan/gofumpt`, `https://github.com/mvdan/gofumpt/issues/275`
- pkg.go.dev, `go.temporal.io/sdk/contrib/tools/workflowcheck` — `https://pkg.go.dev/go.temporal.io/sdk/contrib/tools/workflowcheck`
- pkg.go.dev, `.../workflowcheck/determinism` — `https://pkg.go.dev/go.temporal.io/sdk/contrib/tools/workflowcheck/determinism`
- GitHub, `temporalio/sdk-go` workflowcheck README — `https://github.com/temporalio/sdk-go/blob/main/contrib/tools/workflowcheck/README.md`
- pkg.go.dev, `golang.org/x/tools/go/callgraph`, `.../cha`, `.../rta`, `.../vta` — `https://pkg.go.dev/golang.org/x/tools/go/callgraph`
- pkg.go.dev, `golang.org/x/tools/go/ssa` — `https://pkg.go.dev/golang.org/x/tools/go/ssa`
- pkg.go.dev, `golang.org/x/tools/go/packages` (Overlay) — `https://pkg.go.dev/golang.org/x/tools/go/packages`
- sqlc blog, "Announcing sqlc-gen-go" — `https://sqlc.dev/posts/2023/11/06/publishing-sqlc-gen-go/`
- pkg.go.dev, `entgo.io/ent/entc/gen` — `https://pkg.go.dev/entgo.io/ent/entc/gen`
- pkg.go.dev, `github.com/derision-test/go-mockgen` — `https://pkg.go.dev/github.com/derision-test/go-mockgen`
- GitHub, `dave/jennifer` README — `https://github.com/dave/jennifer`
- Local repo files (not web sources): `/home/user/gimbal/ephemeral/research/temporal-projection/BRIEF.md`, `/home/user/gimbal/internal/generate/{read,stmt,expr,graph}.go`, `/home/user/gimbal/internal/gimballint/{analyzer.go,rules.md}`, `/home/user/gimbal/go.mod`.
