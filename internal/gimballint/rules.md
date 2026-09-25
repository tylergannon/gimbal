# Gimbal workflow lint rules

`gimbal lint` reports deterministic authoring mistakes in calls to Gimbal's
workflow API. The same analyzer runs through `go vet -vettool`.

Each diagnostic is an error report. The analyzer offers no automatic fixes.

## GIMBAL101: no dynamic workers

`GIMBAL101-SIMPLE-WORKFLOWS/NO-DYNAMIC-WORKERS` reports a context-bearing
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
checks functions that contain Gimbal workflow operations and follows direct,
source-visible calls to package-local helpers, including helpers without their
own context parameter. It does not perform whole-program or pointer analysis.

## GIMBAL102: constant context keys

`GIMBAL102-SIMPLE-WORKFLOWS/CONSTANT-CONTEXT-KEY` reports a `gimbal.Set`,
`gimbal.SetJSON`, or `gimbal.Check` key that is not a compile-time string
constant. Use a literal, a named constant, or a constant expression for the
key; put changing data in the value.

```go
// Reported.
gimbal.Set(ctx, fmt.Sprintf("check %d", i), result)

// Allowed.
gimbal.Set(ctx, "check", result)
```

A variable initialized with a literal is still a variable, not a compile-time
constant. Replacing a changing key with one fixed key also does not permit
repeating that key in one scope; GIMBAL103 still applies.

## GIMBAL103: one write per scope key

`GIMBAL103-SET-MISUSE/DUPLICATE-KEY` reports a second `Set`, `SetJSON`, or
`Check` with the same constant key and the same scope context when the earlier
write must run first. It also reports a write using a context defined outside a
loop, because that context can be reused across iterations. In either an
`Iterate` or `PromiseLoop.Tasks` range, a captured outer context is likewise
reused; use the yielded context.

```go
// Reported.
gimbal.Set(ctx, "result", first)
gimbal.SetJSON(ctx, "result", second)

// Also reported: repeated checks name their observations explicitly.
gimbal.Check(ctx, "tests.1", ".", "go", "test", "./...")
gimbal.Check(ctx, "tests.1", ".", "go", "test", "./...")

// Allowed: each taskCtx belongs to one task scope.
for taskCtx, task := range loop.Tasks {
	gimbal.Set(taskCtx, "result", task.Name)
}

// Allowed: two observations have two explicit keys.
gimbal.Check(ctx, "tests.1", ".", "go", "test", "./...")
gimbal.Check(ctx, "tests.2", ".", "go", "test", "./...")
```

Mutually exclusive branches are not reported by this rule.

## GIMBAL104: use the child scope context

`GIMBAL104-SET-MISUSE/WRONG-CONTEXT` reports a `Set`, `SetJSON`, or `Check`
inside a function-literal callback passed directly to `Run`, `Scope`, or
`Group.Go`, or an `Iterate` or `PromiseLoop.Tasks` body, when it writes through
a different context than that callback or task body received. Use that
parameter directly: aliases and derived contexts are not followed for this
rule.

```go
gimbal.Scope(parent, "child", func(child context.Context) error {
	// Reported: parent is not the child scope context.
	gimbal.Set(parent, "result", value)
	// Allowed.
	gimbal.Set(child, "result", value)
	return nil
})
```

## GIMBAL105: do not write from a raw goroutine

`GIMBAL105-SET-MISUSE/UNJOINED-GOROUTINE` reports a `Set`, `SetJSON`, or
`Check` in a raw `go` statement inside one of the recognized child-scope
boundaries. A raw goroutine can outlive its scope. Use a named `Group` child
instead.

```go
// Reported.
go func() { gimbal.Set(child, "result", value) }()

// Allowed.
group := gimbal.Group(ctx, "workers")
group.Go("worker", func(child context.Context) error {
	gimbal.Set(child, "result", value)
	return nil
})
return group.Wait()
```

## GIMBAL106: reserve the task key

`GIMBAL106-SET-MISUSE/RESERVED-TASK-KEY` reports `Set`, `SetJSON`, or `Check`
of the constant key `"task"` through the yielded context in a
`PromiseLoop.Tasks` body. The promise loop owns that key for its task record.
`Iterate` has no task record and does not reserve this key.

```go
for taskCtx, task := range loop.Tasks {
	// Reported.
	gimbal.Set(taskCtx, "task", task.Name)
	// Allowed.
	gimbal.Set(taskCtx, "result", task.Name)
}
```

## GIMBAL107: context must come from a scope

`GIMBAL107-SET-MISUSE/CONTEXT-NOT-FROM-SCOPE` reports `Set`, `SetJSON`, or
`Check` called directly with `context.Background()` or `context.TODO()`. Create
or enter a Gimbal scope and use the context it supplies.

```go
// Reported.
gimbal.Set(context.Background(), "result", value)

// Allowed inside a workflow scope.
gimbal.Set(ctx, "result", value)
```

## GIMBAL108: constant prompts

`GIMBAL108-SIMPLE-WORKFLOWS/CONSTANT-PROMPT` reports a `Session.Generate` call
whose prompt argument, or a `WithSupervisor` call whose instruction argument,
is not a compile-time string constant. A workflow's prompt must be readable
from its source; the run's data reaches the agent through the scope instead,
with `Set` or `SetJSON`, and `Generate` appends it to the prompt itself.

```go
// Reported.
session.Generate[gimbal.Text](ctx, prompt+"\n\n"+goal)

// Allowed.
gimbal.Set(ctx, "goal", goal)
session.Generate[gimbal.Text](ctx, prompt)
```

As with GIMBAL102, a constant expression built from literals and named
constants is still allowed; only a value that can change at runtime is
reported.

Package `github.com/tylergannon/gimbal/cmd/gimbal` is exempt: `cmd/gimbal/run_prompt.go`
runs a prompt given on the command line, so it cannot pass a constant. That
is the only exemption; no other mechanism is added.

## GIMBAL109: constant scope templates

`GIMBAL109-SIMPLE-WORKFLOWS/CONSTANT-SCOPE-TEMPLATE` reports a
`WithScopeTemplate` call whose template is neither a compile-time string
constant nor a variable of the same package declared with `//go:embed`. The
option changes what the agent is sent, so the template must be as readable
from the source as the prompt is. A long template reads better as a file,
and the directive names the file.

```go
// Reported.
session.Generate[Result](ctx, prompt, gimbal.WithScopeTemplate(shape+extra))

// Allowed: a constant.
const shape = `{{range .Values}}## {{.Key}}{{"\n\n"}}{{.Text}}{{end}}`
session.Generate[Result](ctx, prompt, gimbal.WithScopeTemplate(shape))

// Allowed: the file the directive names.
//go:embed shape.tmpl
var shape string
session.Generate[Result](ctx, prompt, gimbal.WithScopeTemplate(shape))
```

Gimbal parses the text once per template and keeps it, so the parse is not
repeated per turn. A template that cannot be parsed, or that cannot render
the scope, is the error `Generate` returns, before any model is called.

## Limits and runtime backstop

This is a source-level, report-only analyzer. It checks the patterns above in
the packages it analyzes; it does not prove all possible runtime control flow,
indirect dispatch, or goroutine lifetime. In particular, GIMBAL101 only
follows direct package-local helper calls from a recognized workflow domain,
and its worker signature test is the presence of a `context.Context`
parameter. Runtime misuse checks remain the backstop for states static analysis
cannot establish.
