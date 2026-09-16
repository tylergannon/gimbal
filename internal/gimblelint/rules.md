# Gimble workflow lint rules

`gimble lint` reports deterministic authoring mistakes in calls to Gimble's
workflow API. The same analyzer runs through `go vet -vettool`.

Each diagnostic is an error report. The analyzer offers no automatic fixes.

## GIMBLE101: no dynamic workers

`GIMBLE101-SIMPLE-WORKFLOWS/NO-DYNAMIC-WORKERS` reports a context-bearing
worker callable selected from a collection, passed as an opaque function
parameter, or assigned from more than one known function before it is called.
Keep the call visible in the workflow.

```go
// Reported.
return workers[kind](ctx)

// Allowed.
switch kind {
case "implement":
	return implement(ctx)
case "review":
	return review(ctx)
}
```

The analyzer recognizes a worker callable by a `context.Context` parameter. It
checks functions that contain Gimble workflow operations and follows direct,
source-visible calls to package-local helpers, including helpers without their
own context parameter. It does not perform whole-program or pointer analysis.

## GIMBLE102: constant context keys

`GIMBLE102-SIMPLE-WORKFLOWS/CONSTANT-CONTEXT-KEY` reports a `gimble.Set` or
`gimble.SetJSON` key that is not a compile-time string constant. Use a literal,
a named constant, or a constant expression for the key; put changing data in
the value.

```go
// Reported.
gimble.Set(ctx, fmt.Sprintf("check %d", i), result)

// Allowed.
gimble.Set(ctx, "check", result)
```

A variable initialized with a literal is still a variable, not a compile-time
constant. Replacing a changing key with one fixed key also does not permit
repeating that key in one scope; GIMBLE103 still applies.

## GIMBLE103: one write per scope key

`GIMBLE103-SET-MISUSE/DUPLICATE-KEY` reports a second `Set` or `SetJSON` with
the same constant key and the same scope context when the earlier write must
run first. It also reports a write using a context defined outside a loop,
because that context can be reused across iterations. In a `Loop.Tasks` range,
a captured outer context is likewise reused; use the yielded task context.

```go
// Reported.
gimble.Set(ctx, "result", first)
gimble.SetJSON(ctx, "result", second)

// Allowed: each taskCtx belongs to one task scope.
for taskCtx, task := range loop.Tasks {
	gimble.Set(taskCtx, "result", task.Name)
}
```

Mutually exclusive branches are not reported by this rule.

## GIMBLE104: use the child scope context

`GIMBLE104-SET-MISUSE/WRONG-CONTEXT` reports a `Set` or `SetJSON` inside a
function-literal callback passed directly to `Run`, `Scope`, or `Group.Go`, or
a `Loop.Tasks` body, when it writes through a different context than that
callback or task body received. Use that parameter directly: aliases and
derived contexts are not followed for this rule.

```go
gimble.Scope(parent, "child", func(child context.Context) error {
	// Reported: parent is not the child scope context.
	gimble.Set(parent, "result", value)
	// Allowed.
	gimble.Set(child, "result", value)
	return nil
})
```

## GIMBLE105: do not write from a raw goroutine

`GIMBLE105-SET-MISUSE/UNJOINED-GOROUTINE` reports a `Set` or `SetJSON` in a
raw `go` statement inside one of the recognized child-scope boundaries. A raw
goroutine can outlive its scope. Use a named `Group` child instead.

```go
// Reported.
go func() { gimble.Set(child, "result", value) }()

// Allowed.
group := gimble.Group(ctx, "workers")
group.Go("worker", func(child context.Context) error {
	gimble.Set(child, "result", value)
	return nil
})
return group.Wait()
```

## GIMBLE106: reserve the task key

`GIMBLE106-SET-MISUSE/RESERVED-TASK-KEY` reports `Set` or `SetJSON` of the
constant key `"task"` through the yielded context in a `Loop.Tasks` body. The
iterator owns that key for its task record. Use another key for task results.

```go
for taskCtx, task := range loop.Tasks {
	// Reported.
	gimble.Set(taskCtx, "task", task.Name)
	// Allowed.
	gimble.Set(taskCtx, "result", task.Name)
}
```

## GIMBLE107: context must come from a scope

`GIMBLE107-SET-MISUSE/CONTEXT-NOT-FROM-SCOPE` reports `Set` or `SetJSON` called
directly with `context.Background()` or `context.TODO()`. Create or enter a
Gimble scope and use the context it supplies.

```go
// Reported.
gimble.Set(context.Background(), "result", value)

// Allowed inside a workflow scope.
gimble.Set(ctx, "result", value)
```

## GIMBLE108: constant prompts

`GIMBLE108-SIMPLE-WORKFLOWS/CONSTANT-PROMPT` reports a `Session.Generate` call
whose prompt argument, or a `WithSupervisor` call whose instruction argument,
is not a compile-time string constant. A workflow's prompt must be readable
from its source; the run's data reaches the agent through the scope instead,
with `Set` or `SetJSON`, and `Generate` appends it to the prompt itself.

```go
// Reported.
session.Generate[gimble.Text](ctx, prompt+"\n\n"+goal)

// Allowed.
gimble.Set(ctx, "goal", goal)
session.Generate[gimble.Text](ctx, prompt)
```

As with GIMBLE102, a constant expression built from literals and named
constants is still allowed; only a value that can change at runtime is
reported.

Package `github.com/tylergannon/gimble/cmd` is exempt: `cmd/run_prompt.go`
runs a prompt given on the command line, so it cannot pass a constant. That
is the only exemption; no other mechanism is added.

## Limits and runtime backstop

This is a source-level, report-only analyzer. It checks the patterns above in
the packages it analyzes; it does not prove all possible runtime control flow,
indirect dispatch, or goroutine lifetime. In particular, GIMBLE101 only
follows direct package-local helper calls from a recognized workflow domain,
and its worker signature test is the presence of a `context.Context`
parameter. Runtime misuse checks remain the backstop for states static analysis
cannot establish.
