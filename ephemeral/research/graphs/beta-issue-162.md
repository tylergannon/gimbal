# #162: Workflow lint: constant context keys, Set misuse, and explicit worker dispatch

Captured 2026-09-14 from https://github.com/tylergannon/gimble/issues/162. Read beta-implementation-handoff.md for final session decisions and coordination notes.

Companion to #159 (infallible `Set`). Once misuse of `Set`/`SetJSON` panics, a live sprint that misuses it dies twenty minutes in, after spending model time. Every deterministic misuse can be reported by a `go/analysis` pass at edit time and in CI. The original five Set misuse checks are runtime errors at `2a52971`, so this does not depend on #142 or #159 and can be built in parallel.

## Set/SetJSON checks

These checks concern calls to `gimble.Set` and `gimble.SetJSON` (resolve with `typeutil.Callee`). The constant-key authoring rule below must diagnose every nonconstant key; only duplicate-key comparison skips keys it cannot compare.

1. **Duplicate key on one scope.** Two calls with equal constant keys whose ctx arguments are the same SSA value, where one call's block dominates the other's (`buildssa`, `Function.DomPreorder` / `Dominates`). Dominance keeps `if/else` arms from being reported. Also report a call inside a loop whose ctx value is defined outside that loop: it repeats the key on the second iteration. Range-over-func bodies (`for ctx, task := range loop.Tasks`) receive a fresh ctx parameter per iteration and are not reported.
2. **Wrong ctx inside a child body.** A call inside a `func(ctx context.Context) error` literal passed to `gimble.Scope`, `gimble.Run`, `(*group).Go`, or used as the range body of `(*loop).Tasks`, whose ctx argument is not that literal's own parameter. Writes go to the parent scope: a race with siblings under `Group`, or a write after the parent ended. Pure AST with `inspector.Cursor.Enclosing(...)` and `types.Info.Uses` on the ctx identifier.
3. **`Set` in an unjoined goroutine.** A call inside a `go` statement's function literal, inside a scope body, that is not a `Group.Go` child. The goroutine can outlive the scope. Message: use `Group`.
4. **Reserved key.** `Set(ctx, "task", ...)` where ctx is the parameter of a `Tasks` range body (`loop.go:135` sets `task` in every task scope).
5. **ctx not from a scope.** The ctx argument is a call to `context.Background` or `context.TODO`.

Broader scope-misuse analysis remains incomplete for helpers and writes across functions. Runtime misuse checks remain a backstop. Nonconstant keys are a separate, directly detectable authoring violation and must hard-fail lint; they do not themselves trigger a runtime panic.

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

The policy concerns dispatch to workflow functions: selecting callable work from a map/slice, choosing among function values, or invoking an opaque worker callback. It does not ban maps, runtime task data, model/provider configuration, dynamic command arguments, or changing values stored under constant context keys. It does not ban Go interfaces generally; ordinary adapter methods are not workflow-function dispatch merely because their implementation is selected at runtime.

The first analyzer must catch direct collection lookup invocation and lookup followed by a simple local assignment/alias in a workflow body or recognized helper. Report the invocation with the lookup's location when useful. Document further supported forms; hard failure applies to every detected violation, while detection coverage may remain incomplete. Do not turn this into a whole-program pointer-analysis project or claim a clean lint result proves all indirect dispatch absent.

Use the actual workflow entrypoints/callbacks and their directly reachable helpers as the analysis domain; do not apply this as a blanket ban on callbacks throughout imported libraries. Known calls to Scope, Group.Go, and other supported Gimble constructs contribute explicit control-flow semantics to that domain.

## Acceptance

- Both failing examples compile under Go and run with a deterministic fake worker when the analyzer is not invoked; invoking the linter returns nonzero and prints the diagnostic identifier.
- The equivalent explicit switch passes lint, and extraction can show its alternatives without predicting task.Kind.
- Direct helpers, supported scope/group callbacks, and a known single-target function alias pass.
- Runtime model/command configuration and dynamic context values remain permitted. Set/SetJSON keys must satisfy the separate constant-key rule.
- An unrelated map lookup or conventional callback in non-workflow/library code is not diagnosed as worker dispatch.


## Shape

- One analyzer, `Requires: inspect, buildssa`, in an internal package with `analysistest` fixtures using `// want` comments for each Set check, the dynamic-worker authoring check, and the non-reports (if/else arms, range-over-func bodies, closure's own ctx).
- A `main` via `singlechecker`, added to the `tool` block in `go.mod` so it runs as `go tool <name> ./...`, the same way `polytype` and `skgo` run today. `just vet` (or the equivalent gate) runs it after `go vet`.
- Runs against `internal/workflows/sprint/sprints.go`, `loop.go`, and both live-run workflows under `ephemeral/review/` as the first real inputs; the current builtin sprint must report its formatted repository-check key. Check every fixture against its actual rule violations rather than assuming builtins or old probes are clean.

Tooling facts checked on 2026-09-13: Go 1.27.1 and `golang.org/x/tools v0.50.0` in the module; `inspector.Cursor` is public, `typeindex` is still internal there; Go 1.26 converged `go vet` and `go fix` on the analysis framework, and report-style analyzers belong behind `go vet -vettool` (`go tool` works as well).

## Acceptance

- `analysistest` passes with at least one positive and one negative fixture per check.
- The current `sprints.go` fails with CONSTANT-CONTEXT-KEY at its `fmt.Sprintf("repository check %d", i+1)` Set key. No builtin exemption or premature workflow rewrite may conceal that failure. `loop.go`'s constant `"task"` write remains valid for the constant-key rule.
- Introducing a second `gimble.Set(ctx, "role", ...)` in the same scope body of `sprints.go` is reported before `go test` is run.
- It runs in the repository's vet gate.

This analyzer includes both runtime-misuse checks and opinionated workflow-authoring checks. Dynamic worker dispatch is the latter: Go compilation and runtime behavior remain unchanged, but the linter hard-fails on detected violations. Related graph extraction: #201.

## Beta authoring principle and proposed follow-up rules

Workflow shapes should be comprehensible by static analysis as a proxy for complexity: possible work, scopes, concurrency, and supervision should be visible in source. Runtime data can choose branches and repetition counts. For now, lint-clean workflows must have completely statically recoverable workflow shape. Unknown workflow operations or structural relationships fail the gate until supported or deliberately permitted; dynamic data and unknown runtime branch choices are not unknown structure.

Dynamic worker dispatch and nonconstant context keys are accepted hard diagnostics. Additional candidates for review, not yet acceptance requirements:

- **Constant structural names:** enforce the existing repository stance for Scope, Group/Group.Go, Loop, NewSession, and Fork. Named constants/constant expressions pass. Constant Set/SetJSON keys are now independently required by the accepted rule below.
- **Structured workflow concurrency:** extend the existing raw-go Set check to recognizable workflow operations, and report known missing Group.Wait joins on ordinary analyzed paths. Do not apply this to ordinary goroutines in adapters/libraries or attempt exhaustive lifetime proofs.
- **No recursive workflow orchestration:** prefer visible ordinary Go loops or Loop.Tasks over recursive helper cycles that themselves perform workflow operations. Recursive data processing is unaffected. This is a new proposal that needs boundary review before enforcement.

Every new rule should have a clear forbidden construct and a simple permitted rewrite. Require complete recovery of workflow structure, without demanding prediction of runtime data or general lifetime proofs. Do not add hard whole-program analysis to permit opaque authoring patterns; keep those patterns outside the lint-clean subset for now.


## Accepted authoring check: constant context keys

Every call to `gimble.Set` or `gimble.SetJSON` must use a compile-time constant string key. String literals, named Go constants, and constant expressions pass. Formatted strings, runtime concatenation, variables, and function-returned keys fail. Values remain dynamic.

Proposed stable diagnostic:

`[GIMBLE102-SIMPLE-WORKFLOWS/CONSTANT-CONTEXT-KEY]: Context keys must be compile-time constants so a workflow's recorded fields are explicit in its source. Use a named constant or string literal; keep changing data in the value.`

Resolve the actual Gimble symbols with Go type information. Inspect the key expression's compile-time value using `go/types`; do not use pointer analysis, execute code, or treat a variable initialized with a literal as a Go constant. Apply to both Set and SetJSON. Do not skip a call merely because its key is nonconstant: that is exactly the diagnostic.

## Real negative acceptance case

The current builtin sprint must fail lint at `internal/workflows/sprint/sprints.go:257`:

```go
gimble.Set(ctx, fmt.Sprintf("repository check %d", i+1), commandText(check, code, output))
```

It loops over the fixed commands `go vet ./...` and `go test ./...` and invents numbered field names. No workflow requirement necessitates those dynamic keys. The straightforward later rewrite executes the checks explicitly and records them immediately under constant keys such as `"repository vet"` and `"repository tests"`. This preserves earlier evidence if the second check aborts; an aggregate-after-the-loop rewrite is not required.

Do not rewrite or exempt the builtin to conceal this acceptance failure. Demonstrate the current source failing with the named diagnostic. A subsequent constant-key workflow change can make it pass. The existing task-specific command already records dynamic command content under the constant key `"task command"` and should pass this rule.

## Fixtures

- Fail the current sprint expression and an equivalent SetJSON call with a runtime key.
- Fail a key read from input, a formatted key, a variable initialized with a literal, and a function-returned key.
- Pass a literal, named constant, and constant concatenation.
- Pass dynamic values, command arguments, task content, and model selection when the context key is constant.
- Prove the negative example still compiles/runs without the analyzer; the linter exits nonzero with the diagnostic at the key expression/callsite.
- Keep the separate duplicate-key rule: two writes to one constant key on the same scope are still misuse. Changing a dynamic key to one repeated constant does not make the original loop valid.
