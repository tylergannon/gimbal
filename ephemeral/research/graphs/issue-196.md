# The supervisor's last look ends as an errored, interrupted turn when the worker finishes

URL: https://github.com/tylergannon/gimble/issues/196
State: open
Milestone: None
Updated: 2026-09-14T01:19:00Z

Found in the loop practice runs (#178): shape 3 run `01M2EQFFPMHJHM9MMZMP8RGT8H.supervisor-objects`.

Each task ran a Codex worker (gpt-5.6-luna) with an Antigravity supervisor (gemini-3.8-flash-low) attached through `WithSupervisor(..., WithInterval(8*time.Second))`. `Generate` cancels the supervisor's look context when the worker returns (`supervise.go`: "Generate cancels and joins its supervisors before returning"), which is right; the supervisor has nothing left to look at.

**Expected:** a look cut short because the worker finished is not a failure of anything; the turn is recorded as ended without error (or not shown as a turn at all), and the page does not list it beside real failures.

**What happened:** every task's final look is a `turn_ended` record with `error: "context canceled"` and `interrupted: true`:

```
31 01:12:41.147 turn_ended practice.1/task.1/supervisor.1/turn.8 err='context canceled' interrupted=True
65 01:14:39.775 turn_ended practice.1/task.2/supervisor.1/turn.9 err='context canceled' interrupted=True
105 01:16:32.732 turn_ended practice.1/task.3/supervisor.1/turn.12 err='context canceled' interrupted=True
```

Three of the run's 29 supervisor turns are "errors" that were nothing of the kind, and the same turn shape (`error: context canceled, interrupted: true`) is what a killed turn looks like (shape 4, `attempt.2/writer.1/turn.1`), so an operator scanning for kills and failures sees the supervisor's routine wind-down among them. A `Killed` cause names who and why; the supervisor wind-down should carry its own cause ("worker finished") or end clean.
