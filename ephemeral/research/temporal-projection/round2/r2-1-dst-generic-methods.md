# R2.1: does dst load, round-trip, and edit Go 1.27 generic-method code?

Date 2026-09-27. Reads: BRIEF.md, round1/SYNTHESIS.md, round1/go-rewrite-
tooling.md §1.4-1.5 and §5. All work done under
`/tmp/claude-0/.../scratchpad/dst-test`, a throwaway Go 1.27 module; nothing
was written under `/home/user/gimbal`.

## Verdict

**Risk eliminated.** `github.com/dave/dst` v0.28.0 (confirmed the latest
version via `proxy.golang.org/.../@latest` today, 2026-09-27, since
pkg.go.dev itself was not directly reachable from this sandbox — the proxy
endpoint is the same authoritative source pkg.go.dev reads from), loaded
through `decorator.Load` with the `go/packages`/`gopackages` resolver,
round-trips Gimbal's exact `Session.Generate[T Output]` shape byte-for-byte
with comments preserved, and performs the requested three-part typed edit
correctly, including in the awkward comment positions, with the edited
package still parsing, `gofmt`-clean, and passing `go vet`. One genuine dst
limitation was reproduced (issue #74) but it does not corrupt a same-
package restore, only cross-package node reuse — relevant to, not fatal
for, the sibling-package plan in go-rewrite-tooling.md §4.2.

A second, more consequential finding surfaced along the way, independent of
dst: **Go 1.27 changed the language, not just dst.** Generic methods with
their own type parameters on a concrete receiver — exactly
`Session.Generate[T Output]` — were rejected by every Go toolchain through
1.26 and are newly legal in 1.27. Round 1 could only infer this; this pass
verified it directly against both toolchains' `go/parser` and
`cmd/compile/internal/types2` source.

## Setup

A Go 1.27.1 module `dsttest` with package `target` mirroring Gimbal's
shapes: `Session` (concrete struct) with `Generate[T Output](ctx
context.Context, prompt string, opts ...Option) (T, error)`; an `Output`
union-constraint interface over three generated-looking types (`Plan`,
`Review`, `Text`); `Group` with `Go(name string, fn func(ctx
context.Context) error)` and `Wait`; a range-over-func `Iterate`; an entry
function `Run` calling `Generate` twice with an explicit type argument
(`s.Generate[Plan](ctx, "draft a plan")`, and a second, multi-line call with
a trailing same-line comment inside the arg list), two closures passed to
`Group.Go` (one single-line, one whose call has a comment between the
closure argument and the call's closing paren), a range-over-func loop
calling `Generate` again, a `//go:generate` directive comment, and
line/trailing comments before closing braces throughout. `dave/dst` and
`golang.org/x/tools` were added via `go get`; `x/tools` resolved to
`v0.50.0` — the exact version Gimbal itself pins per go-rewrite-tooling.md
§1.2.

One planned case did not survive contact with the compiler: an inferred
type argument (`var p Plan; p, err = s.Generate(ctx, ...)`). Built a
minimal generic function returning only `T` with no `T` in any parameter
and assigned its result to a typed variable; both `go build` and `go vet`
reject it with "cannot infer T". Go's type inference does not use
assignment-target context for a return-only type parameter, in Go 1.27 as
in every earlier generics release. Since `Session.Generate`'s `T` appears
only in the result, not in any parameter, **every real call site in
Gimbal's own shape must carry an explicit type argument** — the "if
inference applies" case in the brief does not apply, and a projection never
has to handle an inferred instantiation for this specific method shape.
This narrows, favorably, what a rewriter must recognize.

## Step 2: load and round-trip

`decorator.Load` (package `decorator`, using `packages.Config` with
`NeedSyntax|NeedTypes|NeedTypesInfo|NeedDeps` etc., the same mode shape
`internal/generate` already uses) loaded the package with zero errors on
`pkg.Errors`, including the generic method and every awkward comment
placement. Printing straight back with a restorer built via
`decorator.NewRestorerWithImports(pkg.PkgPath, gopackages.New(dir))` (the
import-aware pairing `decorator.Load` requires — a bare `decorator.Print`
panics on import-managed syntax with an explicit message pointing at this)
produced output **byte-for-byte identical** to the original source file.
No diff. This answers go-rewrite-tooling.md §1.4/1.5's open question
directly: dst does not merely tolerate a Go-1.27 generic method
declaration, it decorates and restores it with full fidelity on first try.

## Step 3: the real typed edit

Three transformations, driven only by `pkg.TypesInfo` (`Instances`,
`Selections`, `Uses`) correlated back to dst nodes via
`pkg.Decorator.Map.Ast.Nodes`, never by identifier text:

(a) Every parameter field whose type resolves (via `info.Uses` on the
selector's package identifier) to `context.Context` from the real `context`
package — found in `Run`'s own signature and in both closures passed to
`Group.Go` — had its type node replaced with a `workflow.Context`
reference.

(b) Every call site was tested against `pkg.TypesInfo.Instances` on the
selector's method identifier — the same technique `gimballint`'s
`isGenericInstantiation` already uses — to find the four real
`s.Generate[T](...)` calls (one at top level, one inside each closure, one
inside the range-over-func loop) and rewrite each to `ExecuteTurn[T](ctx,
s, prompt)`, keeping the original type-argument node, receiver node, and
prompt node in place (dropping the variadic `opts`, per the new
three-argument shape given in the task).

(c) Every call resolved (via `info.Selections`) to a method literally named
`Go` on a receiver whose named type is this package's own `Group` was
matched, and its closure's body was wrapped in a `workflow.Go(ctx,
func(ctx workflow.Context) error { ... })` call followed by `return nil`.

The editor is ~260 lines total (~190 non-blank, non-brace-only), of which
roughly 40 are `go/packages`/`decorator.Load` boilerplate, ~90 are the
three type-directed identification predicates, and the rest is the
`dstutil.Apply` tree surgery and printing. The output, swapped in for
`target.go` and checked in place, was already `gofmt`-clean (verified with
`go/format.Source`, not the `gofmt` binary — see workaround list), passed
`go build ./...`, passed `go vet ./...` with zero diagnostics, and actually
ran end to end (`go run` against a small `main` calling `target.Run`)
producing the expected result — this was a real execution, not just a
type-check.

Comment fidelity through the edit, checked line by line against the
original: the `/* the running context */` block comment stayed attached to
the reused `ctx` argument node inside the rewritten call; the trailing
`// trailing comment inside call args` line comment stayed attached to the
reused `prompt` argument even though the sibling `opts` argument next to it
was dropped; the `// trailing comment before closing brace of closure`
inside the "summarize" closure ended up before the closing brace of the
*inner*, more-deeply-nested closure created by the wrap — the same visual
position relative to the code it was commenting on, just one indent level
deeper, which is correct; the `// comment right after the closure arg,
before the call's closing paren` on the "validate" case's `g.Go(...)` call
survived unchanged, since that whole outer call node was never replaced,
only its argument's contents; and the `//go:generate stringer -type=Kind`
directive comment and every comment on untouched declarations were
byte-identical. No comment was lost, misplaced, or duplicated anywhere in
the diff.

## Step 4: dst issue #74 (generics under `ResolveLocalPath`)

Reproduced directly: with `decorator.NewDecoratorFromPackage` and
`ResolveLocalPath = true`, every ordinary local identifier in the package
(`Kind`, `Session`, `Group`, `Plan` and `Review`/`Text` used as ordinary
types, `ctx`, `err`, etc.) correctly gets `Ident.Path` set to
`"dsttest/target"`, the package's own import path — the documented
behavior of that flag. But every occurrence of the generic method name
`Generate` itself, its own type parameter `T` in some (not all) positions,
and `Plan`/`Review`/`Text` specifically where they appear as elements of
the `Output` union constraint, get `Path == ""` instead — the exact
inconsistency issue #74 reports, reproduced on Gimbal's own shape rather
than the reporter's. Printing this decorated-with-`ResolveLocalPath`
tree straight back through the same import-aware restorer, though,
produced output identical to the plain (non-`ResolveLocalPath`) round-trip
in step 2 — because `Path == ""` already means "print unqualified, no
import," which happens to be correct when restoring into the same package
the nodes came from. **The bug is real and reproducible, but it is latent
for same-package edit-and-restore** (this test's whole shape, and the
shape go-rewrite-tooling.md §4.1-4.2 recommends: read the workflow package,
edit it, emit a sibling). It would become live only if a future rewriter
lifts a generic-method identifier or a union-constraint element out of the
source package into a *different* generated package and trusts `Path` to
decide whether to qualify/import it — worth a guard (an explicit check, not
reliance on `Path`, for exactly these two node shapes) if that design is
ever taken.

## Workarounds and surprises

- **A stale `gofmt` binary produced a false alarm.** `/usr/local/go/bin/go`
  is version-managed (Go's toolchain auto-switch: `go.mod` said `go
  1.27.1`, so `go build`/`go run`/`go vet` transparently fetched and ran
  the real `golang.org/toolchain@v0.0.1-go1.27.1` module), but the
  `gofmt` binary sitting next to it was a leftover `go1.24.7` build and
  rejected the generic method with "method must have no type parameters,"
  discarding the type parameters and printing a parse error. Rebuilding
  `gofmt` from that same stale `/usr/local/go/src` tree reproduced the same
  wrong answer. Diagnosed by comparing `go/parser/parser.go`'s
  `parseFuncDecl` between `/usr/local/go/src` (1.24.7: has an explicit
  `if recv != nil && tparams != nil { error(...) }` block) and the real
  go1.27.1 toolchain module's copy of the same file under
  `$GOROOT/src/go/parser/parser.go` (that block is gone entirely; type
  parameters are now parsed unconditionally regardless of `recv`). Fixed by
  calling `go/format.Source` from a small `go run` program instead of the
  stray binary — guaranteed to use the toolchain the module actually
  selects. This was an environment artifact, not a dst or Go-1.27 finding,
  but it cost real time before the toolchain mismatch was spotted, so it is
  recorded here as a warning for anyone repeating this test in a similarly
  mixed environment.
- **`decorator.Print`/`Fprint` on import-managed syntax panics** unless
  paired with `NewRestorerWithImports` using the same resolver kind used to
  load — the panic message names the fix, but it is easy to hit on a first
  attempt (this pass hit it once).
- **Adding a new qualified reference must use a bare `*dst.Ident{Path:
  ...}`, never a hand-built `*dst.SelectorExpr`.** An initial attempt built
  `workflow.Context`/`workflow.Go` as `SelectorExpr{Ident("workflow"),
  Ident("Context")}` and separately tried to hand-edit the `import` block's
  `GenDecl.Specs`; the restorer silently ignored both — the import-
  management restorer derives the import block from which packages
  identifiers actually reference, not from the raw spec list, and treats an
  unqualified `SelectorExpr` built by hand as two ordinary local
  identifiers rather than a qualified reference. Switching to
  `&dst.Ident{Path: "dsttest/workflow", Name: "Context"}` (documented in
  the dst README's Imports section, found only after the first attempt
  failed) fixed both problems at once: the restorer added the import and
  printed the correct `workflow.Context`/`workflow.Go` qualification with
  no further code.
- Constructing the new, non-nested outer closure (`func(ctx
  workflow.Context) error { workflow.Go(...); return nil }`) needed a fresh
  `*dst.FuncType`/`*dst.BlockStmt` rather than reusing the original's,
  since a dst node can only have one parent; this is ordinary AST-surgery
  discipline, not a dst quirk.

## Effort estimate for a real rewriter

This test's editor (~260 lines) covers exactly three rewrite rules against
one function shape with no cross-package or cross-file reach, and took on
the order of twenty tool round-trips end to end, of which roughly a third
were spent on the stale-`gofmt` environment detour above, not on dst
itself. A real rewriter, per go-rewrite-tooling.md §4-5 and SYNTHESIS.md's
Finalist 2, needs at least: the same three rule shapes generalized past a
single closure/call-count assumption (this test's predicates hard-assert
"exactly one `ctx` field," "exactly two `Group.Go` args" — fine for a
fixture, not for arbitrary workflows); the inlining of same-package helper
calls go-rewrite-tooling.md §5.1 already documents as necessary and already
solved once in `internal/generate/expr.go`'s `inline`; a rule for
`PromiseLoop`/`Iterate` boundaries (untouched by this test, since the loop
variable's own type was deliberately left alone here); and the import-
management discipline above applied consistently everywhere a new
qualified reference is introduced. Given the three rules here took roughly
110 lines of actual tree surgery once the identification predicates
existed, a realistic four-to-six-rule catalogue against
`internal/generate`'s already-closed input set (go-rewrite-tooling.md §5.1)
reads as low hundreds of lines of rewriter code, not a framework — consistent
with round 1's own estimate that this is tractable by syntax-level,
type-informed dst rewriting rather than requiring `go/callgraph`'s RTA/VTA.

## Sources

- `proxy.golang.org/github.com/dave/dst/@latest` and `@v/list`, read
  2026-09-27: confirms `v0.28.0` (2026-09-10) is current; no newer tag
  exists.
- `github.com/dave/dst` v0.28.0 source read directly from the module cache:
  `decorator/load.go`, `decorator/restorer.go`, `decorator/decorator.go`,
  `decorator/resolver/gopackages`, `dstutil/rewrite.go`, and `README.md`'s
  "Imports" section (the `*dst.Ident{Path: ...}` guidance).
  `dave/dst` issue #74 (via `WebFetch`, since this sandbox's proxy routes
  `github.com` through a GitHub-App gate that blocks anonymous reads):
  "generics broken under `ResolveLocalPath`" — reproduced directly rather
  than taken on the issue's word alone.
- `/usr/local/go/src/go/parser/parser.go` (bundled go1.24.7 tree) versus
  `$(go env GOROOT)/src/go/parser/parser.go` and
  `.../cmd/compile/internal/types2/resolver.go` (the real go1.27.1
  toolchain module, `golang.org/toolchain@v0.0.1-go1.27.1`), read directly
  and diffed by hand for the "method must have no type parameters" check:
  present and firing in 1.24.7, absent in 1.27.1 for concrete-receiver
  methods (the interface-method restriction is unchanged).
- All other claims are this pass's own `go build`/`go vet`/`go run` output
  and the dst program's own printed diffs, under
  `/tmp/claude-0/.../scratchpad/dst-test` (not committed, per AGENTS.md).
