Companion to #159 (infallible `Set`). Once misuse of `Set`/`SetJSON` panics, a live sprint that misuses it dies twenty minutes in, after spending model time. Every deterministic misuse can be reported by a `go/analysis` pass at edit time and in CI. Every Set check below is already a runtime error at `2a52971`, so this does not depend on #142 or #159 and can be built in parallel.

## Set/SetJSON checks

These checks concern calls to `gimble.Set` and `gimble.SetJSON` (resolve with `typeutil.Callee`; skip calls whose key is not a constant).

1. **Duplicate key on one scope.** Two calls with equal constant keys whose ctx arguments are the same SSA value, where one call's block dominates the other's (`buildssa`, `Function.DomPreorder` / `Dominates`). Dominance keeps `if/else` arms from being reported. Also report a call inside a loop whose ctx value is defined outside that loop: it repeats the key on the second iteration. Range-over-func bodies (`for ctx, task := range loop.Tasks`) receive a fresh ctx parameter per iteration and are not reported.
2. **Wrong ctx inside a child body.** A call inside a `func(ctx context.Context) error` literal passed to `gimble.Scope`, `gimble.Run`, `(*group).Go`, or used as the range body of `(*loop).Tasks`, whose ctx argument is not that literal's own parameter. Writes go to the parent scope: a race with siblings under `Group`, or a write after the parent ended. Pure AST with `inspector.Cursor.Enclosing(...)` and `types.Info.Uses` on the ctx identifier.
3. **`Set` in an unjoined goroutine.** A call inside a `go` statement's function literal, inside a scope body, that is not a `Group.Go` child. The goroutine can outlive the scope. Message: use `Group`.
4. **Reserved key.** `Set(ctx, "task", ...)` where ctx is the parameter of a `Tasks` range body (`loop.go:135` sets `task` in every task scope).
5. **ctx not from a scope.** The ctx argument is a call to `context.Background` or `context.TODO`.

Not detectable, left to the runtime panic: non-constant keys; a helper that receives a ctx and `Set`s on it (the analyzer cannot know which scope it belongs to); a key set by two different functions on the same scope.

## Workflow authoring check: no dynamic worker dispatch

`[GIMBLE101-SIMPLE-WORKFLOWS/NO-DYNAMIC-WORKERS]: Workflow control flow must be visible in the source. Call worker functions directly in an if or switch instead of selecting a function dynamically.`

The stable identifier is proposed for implementation; keep it consistent in diagnostics, documentation, and fixtures.

## Examples

Fail, including a simple alias of the lookup:

```go
workers[task.Kind](ctx)

worker := workers[task.Kind]
worker(ctx)
```

Pass:

```go
switch task.Kind {
case "implement":
    return implement(ctx, task)
case "review":
    return review(ctx, task)
default:
    return fmt.Errorf("unsupported task kind %q", task.Kind)
}
```

The runtime choice remains dynamic, but the possible workflow calls and their branch conditions are visible. Direct helper functions remain supported. A simple alias to one statically known function does not select among workers and need not fail. Recognized Gimble callbacks such as Scope and Group.Go remain valid.

## Scope and delivery

The policy concerns dispatch to workflow functions: selecting callable work from a map/slice, choosing among function values, or invoking an opaque worker callback. It does not ban maps, runtime task data, model/provider configuration, dynamic command arguments, or dynamic Set keys. It does not ban Go interfaces generally; ordinary adapter methods are not workflow-function dispatch merely because their implementation is selected at runtime.

The first analyzer must catch direct collection lookup invocation and lookup followed by a simple local assignment/alias in a workflow body or recognized helper. Report the invocation with the lookup's location when useful. Document further supported forms; hard failure applies to every detected violation, while detection coverage may remain incomplete. Do not turn this into a whole-program pointer-analysis project or claim a clean lint result proves all indirect dispatch absent.

Use the actual workflow entrypoints/callbacks and their directly reachable helpers as the analysis domain; do not apply this as a blanket ban on callbacks throughout imported libraries. Known calls to Scope, Group.Go, and other supported Gimble constructs contribute explicit control-flow semantics to that domain.

## Acceptance

- Both failing examples compile under Go and run with a deterministic fake worker when the analyzer is not invoked; invoking the linter returns nonzero and prints the diagnostic identifier.
- The equivalent explicit switch passes lint, and extraction can show its alternatives without predicting task.Kind.
- Direct helpers, supported scope/group callbacks, and a known single-target function alias pass.
- Dynamic Set keys and runtime model/command configuration remain permitted.
- An unrelated map lookup or conventional callback in non-workflow/library code is not diagnosed as worker dispatch.


## Shape

- One analyzer, `Requires: inspect, buildssa`, in an internal package with `analysistest` fixtures using `// want` comments for each Set check, the dynamic-worker authoring check, and the non-reports (if/else arms, range-over-func bodies, closure's own ctx).
- A `main` via `singlechecker`, added to the `tool` block in `go.mod` so it runs as `go tool <name> ./...`, the same way `polytype` and `skgo` run today. `just vet` (or the equivalent gate) runs it after `go vet`.
- Runs against `internal/workflows/sprint/sprints.go`, `loop.go`, and both live-run workflows under `ephemeral/review/` as the first real inputs; expected to report nothing on the first two and the three `_ = gimble.Set(...)` lines in the probe code are not misuse (they are correct scopes) so they should not be reported either.

Tooling facts checked on 2026-09-13: Go 1.27.1 and `golang.org/x/tools v0.50.0` in the module; `inspector.Cursor` is public, `typeindex` is still internal there; Go 1.26 converged `go vet` and `go fix` on the analysis framework, and report-style analyzers belong behind `go vet -vettool` (`go tool` works as well).

## Acceptance

- `analysistest` passes with at least one positive and one negative fixture per check.
- The analyzer reports nothing on `sprints.go` and `loop.go` at head.
- Introducing a second `gimble.Set(ctx, "role", ...)` in the same scope body of `sprints.go` is reported before `go test` is run.
- It runs in the repository's vet gate.

This analyzer includes both runtime-misuse checks and opinionated workflow-authoring checks. Dynamic worker dispatch is the latter: Go compilation and runtime behavior remain unchanged, but the linter hard-fails on detected violations. Related graph extraction: #201.

## Beta authoring principle and proposed follow-up rules

Workflow shapes should be comprehensible by static analysis as a proxy for complexity: possible work, scopes, concurrency, and supervision should be visible in source. Runtime data can choose branches and repetition counts. Unsupported but straightforward source is an analyzer gap, not automatically a lint violation.

Dynamic worker dispatch is the newly accepted hard diagnostic above. Additional candidates for review, not yet acceptance requirements:

- **Constant structural names:** enforce the existing repository stance for Scope, Group/Group.Go, Loop, NewSession, and Fork. Named constants/constant expressions pass; dynamic Set keys remain permitted.
- **Structured workflow concurrency:** extend the existing raw-go Set check to recognizable workflow operations, and report known missing Group.Wait joins on ordinary analyzed paths. Do not apply this to ordinary goroutines in adapters/libraries or attempt exhaustive lifetime proofs.
- **No recursive workflow orchestration:** prefer visible ordinary Go loops or Loop.Tasks over recursive helper cycles that themselves perform workflow operations. Recursive data processing is unaffected. This is a new proposal that needs boundary review before enforcement.

Every new rule should have a clear forbidden construct and a simple permitted rewrite. Do not make “analysis could not resolve this” a generic hard error or add hard whole-program analysis to enforce these rules exhaustively.
