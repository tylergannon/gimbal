# Shape 4: a Group inside a task, one attempt killed by id

Second run `01M2EQK497YCFAC0JV2K7T66WV.group-in-task`, 2026-09-13 19:13:05 to
19:14:01 local (56.1s); first run `01M2EQFH6EPBFAB7KVE4RW6D4P.group-in-task`
(49.8s), kept as `first-run.jsonl` and `live-first-run.txt`. Planner and the
three attempt writers codex/gpt-5.6-luna. Goal: one `limerick.txt`, planned as
one task; the task runs three attempts in `gimble.Group(ctx, "attempts")`,
each in its own directory, each told to `sleep 6` before writing so there is
time to kill one. A second goroutine holding only the run id follows
run.jsonl and, on the first `turn_started` under `/attempt.2`, waits 3s and
calls `runtime.KillScope(runID, "practice.1/task.1/attempts.1/attempt.2",
"operator", reason)`. Program: `main.go`; console: `live.txt`; log dir:
`logs/runs/01M2EQK497YCFAC0JV2K7T66WV.group-in-task/`; page evidence:
`snapshot-midrun.json` (one second after the kill), `snapshot.json`,
`page.html`.

## Lap by lap (second run)

- Lap 1 (planner.1/turn.1, 20s): "Create limerick.txt", with a validation
  command (`wc -l` = 5) this workflow does not run. Scopes `attempts.1` and
  `attempt.1/2/3` began at 01:13:25.44; three writer turns started at
  01:13:25.7.
- The operator saw attempt 2's `turn_started`, slept 3s, and
  `KillScope` returned nil: console `[operator] KillScope(01M2EQK4...,
  "practice.1/task.1/attempts.1/attempt.2"): ok`. `killed` at seq 20
  (01:13:28.703); attempt 2's turn ended at seq 21 with `interrupted: true`
  and the `Killed` cause as its error, 3.0s in; its scope ended at seq 22
  with the same error; its session closed at seq 23. `attempt-2/` on disk is
  empty: the kill landed during the `sleep 6`.
- Attempt 3 finished at 18s, attempt 1 at 24s, each with a limerick.
- `group.Wait()` returned the Killed error (the `attempts.1` scope ended
  with it, seq 30); the task treated it as non-fatal, recorded `attempt 1`
  (finished), `attempt 2` (killed by operator: ...), `attempt 3` (finished)
  and `winner` (attempt 1; its file copied to the workspace) on `task.1`
  (seq 31-34), and `task.1` ended clean (seq 35).
- Planner.1/turn.2 (11s): `{"tasks":[],"next":null}`; dispatch ended; the
  run ended clean. The loop continued past the kill exactly as the shape
  asks: the task recorded it and the planner got its next decision.

## Which kill was sent and what happened

`runtime.KillScope(run, "practice.1/task.1/attempts.1/attempt.2", "operator",
"the operator killed attempt 2 by id from a second goroutine")`, from a
goroutine that had nothing but the run id and the log. The scope key was
predictable from the code (`practice` loop, first task, `attempts` group,
second `attempt` child), and the record confirms the target:

```
{"seq":20,"time":"2026-09-14T01:13:28.703261Z","scope":"practice.1/task.1/attempts.1/attempt.2","event":{"by":"operator","kind":"killed","reason":"the operator killed attempt 2 by id from a second goroutine","target":"practice.1/task.1/attempts.1/attempt.2"}}
{"seq":21,...,"turn":"practice.1/task.1/attempts.1/attempt.2/writer.1/turn.1","event":{"duration":3027236875,"error":"context canceled: gimble: practice.1/task.1/attempts.1/attempt.2 was killed by operator: the operator killed attempt 2 by id from a second goroutine","interrupted":true,"kind":"turn_ended",...}}
{"seq":22,...,"scope":"practice.1/task.1/attempts.1/attempt.2","event":{"error":"context canceled: gimble: ... was killed by operator: ...","kind":"scope_ended"}}
{"seq":30,...,"scope":"practice.1/task.1/attempts.1","event":{"error":"context canceled: gimble: ... was killed by operator: ...","kind":"scope_ended"}}
{"seq":32,...,"scope":"practice.1/task.1","event":{"key":"attempt 2","kind":"value_set","value":"\"killed by operator: the operator killed attempt 2 by id from a second goroutine\""}}
{"seq":35,...,"scope":"practice.1/task.1","event":{"error":"","kind":"scope_ended"}}
{"seq":38,...,"scope":"practice.1","event":{"kind":"planner_decision"}}
```

Siblings were untouched: attempts 1 and 3 ended with no error, as `Group`
promises for a child killed with the `Killed` cause.

## What went wrong

- First run: no kill was sent. The operator goroutine's `runlog.Read` opened
  `run.jsonl` before the run had written it and returned ENOENT at once, so
  it never saw a `turn_started`; all three attempts finished, attempt 1 won,
  the planner ended dispatch (`live-first-run.txt`, `first-run.jsonl`, 42
  records, no `killed`). The run directory exists before the first record
  (#192). `waitRunID` now waits for `run.jsonl`.
- The killed turn's usage is zero: `turn_usage` for attempt 2's turn is all
  zeros and so is its session total, although the model had received the
  prompt and run a tool call. The two finished attempts report 14609 and 5028
  input tokens (#195).
- The kill itself appears on the page only inside error strings; there is no
  `killed` record in the snapshot, so who killed what and why is on the page
  only because the `Killed` cause text was quoted into the scope's and turn's
  error and the workflow copied it into the `attempt 2` value (#197).
- `Group.Wait` returns the Killed error, so the `attempts.1` scope shows as
  ended with an error while its parent `task.1` is clean. That is the
  contract, and the task's `winner` value explains it, but on the page the
  group reads like a failed subtree under a task that succeeded; an operator
  has to know that a killed loser is the design.

## What run.jsonl shows

43 records: run_started, 7 scope_began / 7 scope_ended, 4 session_created /
4 session_closed, 5 turn_started / 5 turn_ended, 2 planner_decision, 1
killed, 5 value_set, run_ended, complete. The `killed` record (seq 20) is
the first record after the three turn_starteds and precedes the killed
turn's turn_ended by 20ms. Usage sums (48047 in, 1619 out, 665 reasoning,
212480 cache read) match the snapshot totals and the page header, with
attempt 2 contributing nothing.

## What the page showed

Fetched with HTTP GET (no browser available); snapshot with
`GET /api/runs/{id}`.

- Shown well: the scope tree `practice.1` > `task.1` > `attempts.1` >
  `attempt.1/2/3`, each attempt carrying the inherited `task` value; the
  `attempt 1`, `attempt 2`, `attempt 3`, `winner` values on `task.1`;
  `attempt.2` "ended context canceled: gimble: practice.1/task.1/attempts.1/attempt.2
  was killed by operator: ..." and `attempts.1` with the same error while
  `task.1` shows ended clean; two decisions on `practice.1`.
- Not shown: any `killed` record; `snapshot.json` has none (#197). The
  killed attempt's usage as 0 (#195).
- Shown badly: values as JSON-escaped strings (#193).
- `snapshot-midrun.json` (one second after the kill): run running, attempt.2
  scope `ended` with the Killed error, attempt.1 and attempt.3 `running`,
  attempt.2's turn with `error` set and `interrupted: true`, the other two
  writer turns with `ended: 0`, transcripts for the planner turn and all
  three writer turns (attempt 2's holding the prompt and its `sleep 6` call).

## Issues filed

- #192 runlog.Read fails at once when the run directory exists but
  run.jsonl is not yet written
- #193 Run page shows scope values as raw JSON text, not as the value
- #195 A killed turn records zero usage
- #197 Run page and snapshot show no steer or killed records
