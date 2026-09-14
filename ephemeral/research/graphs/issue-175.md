# Cancel by id with a cause: the runtime keeps its cancel funcs, one ctx per turn, Killed replaces Session.Interrupt

URL: https://github.com/tylergannon/gimble/issues/175
State: closed
Updated: 2026-09-14T00:46:44Z

The trade for `Session.Interrupt` (#165). Tyler's rule stands: cancelling the ctx is the interrupt. What was missing is that the person who wants to stop an agent usually does not hold the pointer. An operator watching the run page has ids. So the runtime keeps the cancel funcs it already creates, reachable by id, and cancels with a cause so the workflow can tell a kill from an ordinary shutdown.

## Facts at `ad6f2f6`

- `scope.do` (`scope.go:78`) already creates one child ctx per scope with `context.WithCancel` and defers `end(cancel)`. `Group` does the same (`group.go:36`). Workflow code never types `cancel`; the library owns every cancel func because it creates every child ctx.
- `Generate` passes its ctx straight to `RunTurn` (`session.go:189`). There is no per-turn ctx, so one turn cannot be stopped without ending its scope.
- `run` (`run.go:34`) has no table of live scopes. The cancel funcs live on the stack of `do` and are unreachable.
- `Group` cancels every sibling on the first child error (`group.go:51`), the errgroup rule.
- `Loop` runs each task in a task scope (`loop.go:132`) and treats any error the same way.
- `web.Runtime.Run` (`web/runtime.go:187`) already wraps each run in `context.WithCancelCause`.

## Change

Stdlib only. No wrapper around `context.Context`, no new interface, no change to `HarnessAdapter`.

1. **Cause, not bare cancel.** `scope.do` and `Group` use `context.WithCancelCause`. Normal end calls `cancel(nil)` as today. A kill calls `cancel(err)` where `err` is:

   ```go
   // Killed is the cause of a ctx that an operator cancelled on purpose.
   // context.Cause(ctx) returns it in every scope and turn under the target.
   type Killed struct {
       Target string // scope key, or turn id
       By     string // who; "" when unknown
       Reason string
   }
   func (k Killed) Error() string
   ```

   Every descendant ctx reports the same cause, because a child inherits the parent's cause. Workflow code that cares checks `errors.As(context.Cause(ctx), &killed)`; code that does not care sees `ctx.Err()` as before.

2. **A table of live scopes and turns on `run`.** `scope` gains a `cancel context.CancelCauseFunc`; `run` gains `scopes map[string]*scope` (registered in `do`, removed in `end`) and `turns map[string]context.CancelCauseFunc` keyed by turn id.

3. **One ctx per turn in `Generate`.** Before `RunTurn`: `turnCtx, cancel := context.WithCancelCause(ctx)`, registered under the turn id, deregistered when `RunTurn` returns. Killing a turn ends only that turn: `Generate` returns an error wrapping the `Killed` cause, the scope keeps running, and the workflow decides whether to re-ask. This is what a "kill this agent" button on a turn row does.

4. **Two functions on `run`, unexported here, exported by the web runtime in the registry issue:** `cancelScope(key string, cause error)` and `cancelTurn(id string, cause error)`. Unknown id is an error. Both record a lifecycle event so the log says who killed what: `Killed{Target, By, Reason}` replaces the `Interrupt` lifecycle event that #165 deletes (the schema entry moves with it).

5. **Loop treats a kill as a failed task, not a broken loop.** When a task scope returns an error whose cause is `Killed`, the task is recorded failed with the reason and the planner sees it on the next lap. That is "the workflow reschedules and restarts the task".

6. **Group treats a killed child as gone, not as an abort.** A child whose error's cause is `Killed` does not cancel its siblings. `Wait` still returns the error. A bake-off survives one killed attempt.

7. **#165 in the same PR.** Delete `Session.Interrupt`, the adapter type assertion, `(*adapter).Interrupt` in `codex/` and `claude/`, and the `Interrupt` lifecycle event and schema entry. Add the missing test: after a turn is cancelled through its ctx, the same `Session` runs another turn.

## Done when

- `go test ./...` has: kill a scope from another goroutine and every session inside it is closed and the scope returns an error with the cause; kill one turn and the same session completes a second turn in the same scope; a `Group` of three with one killed child returns that child's error and the other two finish; a `Loop` with a killed task records the task failed and runs another lap.
- One live run on the cheap tier where a turn is killed by id from a second goroutine, with the run log under `ephemeral/attest/cancel-by-id/`.
- `Session.Interrupt` and the adapters' `Interrupt` methods are gone.

