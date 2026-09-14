# Delete Session.Interrupt and the adapter Interrupt methods: cancelling the ctx already interrupts the turn

URL: https://github.com/tylergannon/gimble/issues/165
State: closed
Updated: 2026-09-14T00:46:43Z

Tyler's standing direction: cancelling the context is the interrupt. The code already implements that, and `Session.Interrupt` is a second, unused way to say the same thing. Remove it.

## Facts at `2a52971`

- `harness.go:19`, the `RunTurn` contract: "Cancelling ctx interrupts the native turn and returns ctx.Err()."
- Codex adapter honours it: `codex/codex.go:146` sends `turn/interrupt` when ctx ends, drains the turn on a short background context, clears the active turn, and returns `ctx.Err()`.
- Claude adapter honours it: `claude/claude.go:251` interrupts the stream with a receipt when ctx ends first.
- `Session.Interrupt` (`session.go:348-364`) has **no callers** outside itself. It reaches the adapter through an optional type assertion; both adapters carry an `Interrupt` method only to satisfy it. It also emits a lifecycle event `Interrupt{Target, Source}` (`events.go:142`).

Interrupting one turn without ending its scope is ordinary Go:

```go
tctx, cancel := context.WithCancel(ctx)
go func() { <-stopSignal; cancel() }()
result, err := worker.Generate(tctx, prompt)
```

## Change

Delete `Session.Interrupt`, the adapter type assertion, `(*adapter).Interrupt` in `codex/` and `claude/`, the `Interrupt` lifecycle event and its schema entry (`jsonschema_gen.go`), and any web reducer case for it. `AgentEvent.Interrupted` (`events.go:117`) stays: it reports what the harness did, which is how a cancelled turn shows up in the record.

Add the one test that is missing: after a turn is cancelled through its ctx, the same `Session` runs another turn successfully (the Codex adapter's drain-and-clear path at `codex.go:146-150` is meant to guarantee this, and nothing proves it).

## Corrects

#141 item 5 and the joint recommendations document proposed the opposite, promoting `Interrupt` into the required adapter interface. Both reviewers missed that the ctx contract already exists and is implemented. That item is withdrawn.

