# Issue #162 audit

Integration note after Tyler's follow-up: the missing constant-key/name checks below are gaps relative to the historical design, not prerequisites for shipping #162. Tyler explicitly permits dynamic keys and prefers an incomplete linter. General provenance/escape analysis described here can be deferred; `delivery-slices.md` defines the current small first pass.

Source issue: `ephemeral/research/graphs/issue-162.json` (issue #162,
retrieved into this checkout before this audit). Current contract sources are
`ephemeral/research/api/API.md` and runtime files at repository head.

## Check-by-check corrections

1. **Duplicate key on one scope.** The intent is correct, and runtime enforces
   duplicate keys with a panic under `scope.go:163-182`; the issue's “same SSA
   value” test is insufficient for the contract. `context.WithCancel`,
   `WithTimeout`, `WithValue`, and the runtime's own context derivations retain
   the scope pointer, while producing new SSA values (`scope.go:78-85`,
   `group.go:39-42`, `loop.go:132`). The analyzer must normalize a context's
   scope provenance through recognized `context.With*` and Gimble scope
   constructors before comparing scope identity. Dominance is useful for
   straight-line duplicate calls, but it must not silently treat separate
   possible scope instances as one; report only definite duplicates or retain
   possible-scope diagnostics in the graph.

2. **Wrong ctx inside a child body.** The proposed direct parameter comparison
   catches the simple mistake in a `Scope`/`Group.Go` callback, but it is not a
   complete context rule. A derived child ctx (`context.WithTimeout(ctx, ...)`)
   still points to the same current scope; comparing the argument to the
   callback parameter would wrongly reject it. Conversely, a helper receiving a
   ctx can write through the parent or child and cannot be resolved by local
   `types.Info.Uses` alone. The API explicitly says ctx flows through parameters
   or lexical captures, is derived through `context.With*` and Gimble functions,
   and can have conditional possible scopes (`API.md:856-867`). Implement a
   provenance/dataflow model; use direct parameter identity only as one fact.

3. **Set in an unjoined goroutine.** The issue catches a raw `go` function
   literal containing Set, but its wording limits the violation to a scope
   body and says “not a Group.Go child.” The contract is broader: `Group.Go` is
   the only workflow API for starting goroutines; raw `go`, `sync.WaitGroup`,
   and `errgroup` are forbidden in workflow packages (`API.md:167-179,
   275-293`). A raw goroutine nested inside a `Group.Go` callback is still not a
   Gimble child and can outlive that callback. The analyzer should report every
   workflow raw goroutine that reaches a Set/node operation, then separately
   model the runtime's own goroutines as implementation code.

4. **Reserved `task` key.** The check is necessary: each Loop task scope
   stores `task` before invoking the callback (`loop.go:131-140`), and a user
   `Set(taskCtx, "task", ...)` would deterministically panic (`scope.go:173-175`).
   The issue's exact “range body parameter” recognition is a good positive
   case, but aliases and derived contexts from that task ctx must also be
   recognized. Current workflows use `taskCtx` correctly for `worker result`,
   `files`, and `result` (`ephemeral/attest/issue144/main.go:47-64`,
   `ephemeral/attest/just-attest/main.go:185-195`).

5. **Ctx not from a scope.** Direct `context.Background()`/`TODO()` arguments
   are a useful definite-error check, but this is only one way the scope chain
   can be severed. The API rule is that node/value paths must derive from
   `Run`/scope contexts; context values stored in structs/globals and unknown
   external context sources are prohibited or must remain unknown
   (`API.md:290-293`, `API.md:856-867`). Track aliases and all recognized
   `context.With*` calls. Do not flag the ordinary outer setup contexts in
   `ephemeral/attest/*` or adapter code: they are passed to `Run`, whose body
   receives a new scope ctx (`run.go:175-187`), and adapter cleanup deliberately
   uses independent background contexts (`scope.go:113-117`).

## Missing mandatory check

Issue #162 explicitly skips non-constant keys and names when the API contract
requires reporting them. `API.md:275-293` lists “a non-constant key or name” as
a build-failing lint, while `scope.next` and `store` currently accept ordinary
strings (`scope.go:52-59`, `scope.go:145-182`). A live counterexample is
`gimble.Set(ctx, fmt.Sprintf("attempt %d", i+1), outcome)` in
`ephemeral/attest/loop-practice/group-in-task/main.go:133-164`: runtime accepts
it, but a static graph cannot name a stable data node. The eventual analyzer
must include constant checks for every graph name/key-bearing API call, not just
Set/SetJSON: `Scope`, `Group`, `Group.Go`, `NewSession`, `Fork`, `Loop`, and
`Each` names as applicable. Keep dynamic values in ordinary prompt/data text.

## Runtime facts and gaps

| Rule | Documented contract | Runtime enforcement | Static gap / restriction |
| --- | --- | --- | --- |
| Set/SetJSON type | `Set` accepts only `~string \| ~int \| ~bool \| ~[]string`; `SetJSON` accepts `Output`; arbitrary maps/structs do not compile (`API.md:220-235`). | Go type checking enforces generic constraints; JSON marshaling can panic (`scope.go:145-160`). | No analyzer needed for type shape; report dynamic key separately. |
| Set once | One key once per scope instance; revision is child shadowing (`API.md:237-247`). | Mutex, `values` map, and `ended` enforce duplicate/ended/no-scope panics (`scope.go:163-182`). | SSA dominance alone misses derived-context aliases and interprocedural calls. |
| Scope ancestry | Scope ctx identifies current scope; Scope ends on body return (`API.md:155-219`). | `scope.parent` and runtime `scopeKey` implement nearest-scope lookup (`scope.go:31-64`, `184-201`). | Preserve possible scope sets at branches; do not invent one scope from a conditional. |
| Session ownership | New sessions belong to creating scope; scope end closes them (`API.md:180-192`). | `scope.adopt`, `scope.end`, `Session.closed`, and `usable` enforce post-end rejection (`scope.go:67-74`, `92-125`; `session.go:410-420`). | The Generate/Fork ctx need not equal owner scope; graph must show creation owner and use separately. |
| Fork | Fork copies conversation, same workdir, independent after fork (`API.md:90-99`). | Fork calls adapter, adopts new session in current ctx, records Parent (`session.go:487-512`). | Alias/receiver resolution and ownership across helper calls need call/dataflow facts. |
| Group join | Group.Go creates child scope; Wait is the join and must happen on every path (`API.md:167-179`, `433-510`). | `Wait` blocks `wg`, ends group; first non-killed error cancels siblings (`group.go:58-98`). | No runtime guard for never-called Wait; linter must model control flow, early returns, and raw goroutines. |
| Loop task lifetime | Each yielded ctx is a child task scope; values feed next planner decision (`API.md:515-598`). | `taskScope.do` ends before next planner turn; `task` is reserved (`loop.go:127-163`). | Range-over-function syntax and closure captures require AST/SSA handling; task ctx saved for later is a likely escape. |
| Supervisor lifetime | Supervisors are attached per turn, steer, never gate (`API.md:598-717`). | `supervise` starts internal goroutines, cancels and joins them after worker (`supervise.go:72-112`). | Validate caller-owned supervisor session is live long enough; no rule is currently enforced for cross-scope supervisor ownership. |

## Graph extraction shape

The static pass should produce a template graph keyed by source position and
constant names, with scope edges, session creation/parent/fork edges, turn
sites, supervisor attachments, value writes, and prompt reads. Runtime graph
identity is the scope key prefix tree and session/turn IDs (`API.md:720-799`);
static extraction should use the same conceptual containment while retaining
possible scopes for conditionals. A call to `Generate` does not implicitly read
scope data; only explicit `ScopeText`, `Get`, or `GetJSON` uses create data-read
edges (`scope.go:184-201`, `API.md:216-219`).

Recommended analysis layers:

1. AST/type pass: resolve Gimble callees, constant string arguments, callback
   parameters, range-over-function bodies, and source positions.
2. SSA pass: propagate context provenance through assignments, lexical captures,
   recognized `context.With*`/Gimble calls, and branch joins; use dominance only
   after provenance normalization.
3. Call/escape summaries: detect raw goroutine capture, Group.Wait coverage,
   context struct/global storage, Session aliases, and helper calls. Export
   serializable package facts when provenance crosses package boundaries; the
   Go analysis driver supports this through `FactTypes` and import/export facts
   (`rules-go-analysis.md`).
4. Driver: make diagnostics fatal in the repository's gate. `analysis` itself
   leaves severity/filtering to the driver (`rules-go-analysis.md`); the API
   requires every lint to fail the build (`API.md:275-279`).

## Current workflow examples

- Correct task scopes: the Sprint workflow ranges over `loop.Tasks` and writes
  task values with that callback ctx (`internal/workflows/sprint/sprints.go:95-109`,
  `209-260`).
- Correct fork ownership: a researcher is created/primed in the run scope,
  then forked as planner and coder from the active scope
  (`internal/workflows/sprint/sprints.go:80-105`, `211-218`; also
  `example_shapes_test.go:284-303`).
- Correct Group join: the bake-off creates all `Group.Go` children and calls
  `Wait` before consuming outcomes (`example_shapes_test.go:142-164`); the
  runtime tests assert first-error cancellation and joined return
  (`gimble_test.go:287-315`).
- Intentional dynamic-key gap: the loop practice writes one result per attempt
  using a formatted key (`ephemeral/attest/loop-practice/group-in-task/main.go:163`),
  which is accepted today but violates the static graph contract.
- Intentional raw goroutines outside workflow scopes: attestation programs use
  `go announce` and operator goroutines (`ephemeral/attest/just-attest/main.go:89`,
  `126`, `150`); a gate must define workflow package boundaries or these
  infrastructure goroutines will be false positives. The documented intended
  scope is workflow code, not adapters/servers/tests.

## Verdict

Issue #162 is a useful first Set misuse slice, but acceptance must be amended
before implementation: add non-constant names/keys, replace same-SSA ctx
identity with scope provenance, widen raw-goroutine detection, and define
workflow package boundaries. Keep unknown helper/interface flows as explicit
“unknown” graph edges unless a package fact or conservative diagnostic makes
the violation definite. Runtime remains the backstop for dynamic keys,
post-end session use, unknown helpers, and values that escape through unmodeled
interfaces.
