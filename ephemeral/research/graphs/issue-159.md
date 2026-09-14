# Infallible Set/SetJSON: no error return, panic on misuse, runtime panic boundary writes the terminal record

URL: https://github.com/tylergannon/gimble/issues/159
State: closed
Updated: 2026-09-14T00:56:44Z

Follow-up to #141 item 8, decided by Tyler on 2026-09-12: make `Set` and `SetJSON` infallible. The joint review had withdrawn this on the strength of `Set(ctx, k, math.NaN())` being a real marshal failure and the runtime having no panic boundary. Neither is a reason to keep the error return: nothing uses the `float64` arm, and the boundary is small. Horses, not zebras.

## Problem

Every `Set` in a workflow is wrapped in three lines of `if err != nil { return err }` that carry no information. Measured at `2a52971`: 18 of 210 code lines in `internal/workflows/sprint/sprints.go` and 42 of 193 in Codex's live workflow ([ceremony.txt](https://github.com/tylergannon/gimble/blob/claude/loop-review-20260912/ephemeral/review/claude-loop-api/ceremony.txt)). The eye skips every third line of what should read like pseudocode.

The observed failure this causes is worse than the noise. Authors swallow the error because it is noise: Claude's probe code has `_ = gimble.Set(...)` three times; Codex's probe code did the same. A swallowed `Set` failure silently leaves a value out of every later prompt. One real collision is already built in: every task scope reserves the key `task` (`loop.go:135`), so a workflow `Set(ctx, "task", ...)` inside a Loop body fails with "already set", and with `_ =` nobody finds out.

## What `Set` can fail on today (`scope.go:111-153`)

| Cause | Kind |
| --- | --- |
| No scope in ctx | Programmer error, deterministic, fails on first run |
| Key already set in this scope | Programmer error, deterministic in sequential code; a design error if two goroutines race on one key |
| Scope already ended | Programmer error, dynamic: a goroutine outlived its scope |
| `json.Marshal` failure | Only via the `float64` arm (NaN, ±Inf) or a polytype `Output` with a non-finite float field (tylergannon/polytype#126) |

Disk failure is not on the list. `store` never returns a write error; the journal write goes through `run.event`, which records failures for `Complete{RecordingError}`. That stays as it is.

## Decision

```go
func Set[V ~string | ~int | ~bool | ~[]string](ctx context.Context, key string, value V)
func SetJSON[V Output](ctx context.Context, key string, value V)
```

- No error return. Drop the `float64` arm; nothing in the repo uses it, and it is the only marshal horse. A `SetJSON` marshal failure on a non-finite float field is a zebra and panics like any other misuse.
- Misuse panics with a message naming the key and the scope, as the errors do today: `gimble: "task" is already set in scope "delivery.1/task.2"`.
- **Panic boundary in the runtime, so the durable record is complete when the process dies.** `Run` defers a recover that writes `RunEnded{Error}` and `Complete`, then re-panics with the original value. `Group.Go` recovers into an error value that carries the panic and its stack, cancels the group as any child error does, and `Wait` returns it; when `Run` sees that value in the body's result it records the terminal events and re-panics. Misuse anywhere therefore means one thing: a complete `run.jsonl` and a dead process. About 25 lines.
- The process dying is acceptable. A later storage implementation that wants to retry writes does so under `run.event`, not in `Set`.

Static checking: #162 adds a `go/analysis` pass for the deterministic misuses (duplicate key on one scope, wrong ctx in a child body, unjoined goroutine, reserved key, Background ctx). It does not depend on this change and can land before it, so a live sprint never has to discover these by panicking.

## Where it touches

- `scope.go`: `Set`, `SetJSON`, `store` panic instead of returning.
- `run.go`: the deferred recover and the re-panic on a panic-carrying error.
- `group.go`: recover in `Go`.
- `internal/workflows/sprint/sprints.go`: 6 calls lose their wrappers. Also `loop.go`'s own `store(ctx, "task", raw)`.
- `API.md` / Godoc: "Set is set-once per scope; misuse is a programming error and panics" replaces "errors are loud".
- Lands after #142 (same files).

## Acceptance

- `sprints.go` compiles with no `if err` around any `Set`/`SetJSON`, and its line count drops by about 18.
- A test that calls `Set` twice on one key in the main goroutine observes a panic *and* a `run.jsonl` ending in `run_ended` with the panic message and `complete`.
- The same misuse inside a `Group.Go` child produces the same log shape and the same process outcome.
- `go vet`, `go test -race`, and a live sprint on the cheap tier unchanged.


