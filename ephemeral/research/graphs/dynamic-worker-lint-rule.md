# Opinionated workflow rule: no dynamic worker dispatch

Tyler's direction, 2026-09-14: dynamic selection of a workflow function is against Gimble's authoring stance. It remains valid Go and may compile and run, but the workflow linter reports an error and exits nonzero. This is an authoring rule, not a claim of a runtime safety violation.

## Diagnostic

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

The policy concerns dispatch to workflow functions: selecting callable work from a map/slice, choosing among function values, or invoking an opaque worker callback. It does not ban maps, runtime task data, model/provider configuration, dynamic command arguments, or dynamic values stored under constant context keys. It does not ban Go interfaces generally; ordinary adapter methods are not workflow-function dispatch merely because their implementation is selected at runtime.

The first analyzer must catch direct collection lookup invocation and lookup followed by a simple local assignment/alias in a workflow body or recognized helper. Report the invocation with the lookup's location when useful. Document further supported forms; hard failure applies to every detected violation, while detection coverage may remain incomplete. Do not turn this into a whole-program pointer-analysis project or claim a clean lint result proves all indirect dispatch absent.

Use the actual workflow entrypoints/callbacks and their directly reachable helpers as the analysis domain; do not apply this as a blanket ban on callbacks throughout imported libraries. Known calls to Scope, Group.Go, and other supported Gimble constructs contribute explicit control-flow semantics to that domain.

## Acceptance

- Both failing examples compile under Go and run with a deterministic fake worker when the analyzer is not invoked; invoking the linter returns nonzero and prints the diagnostic identifier.
- The equivalent explicit switch passes lint, and extraction can show its alternatives without predicting task.Kind.
- Direct helpers, supported scope/group callbacks, and a known single-target function alias pass.
- Runtime model/command configuration and dynamic context values remain permitted; context keys must follow CONSTANT-CONTEXT-KEY.
- An unrelated map lookup or conventional callback in non-workflow/library code is not diagnosed as worker dispatch.

This belongs to Beta linter #162 and simplifies the supported authoring model for Beta graph extractor #201. The extractor may still describe unsupported source partially; extraction is not a substitute for running the linter, and the runtime must not start enforcing this authoring restriction.
