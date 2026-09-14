# Shape 1: the planner ends the loop on its own

Run `01M2EQC626WZFR69JBN913T41M.planner-ends`, 2026-09-13 19:09:17 to 19:10:36
local (1m18.7s). Planner codex/gpt-5.6-luna, worker agy/gemini-3.8-flash-low.
Goal: three haiku files in a scratch workspace, one file per task, dispatch
ends when all three exist. Program: `main.go`; console: `live.txt`; log dir:
`logs/runs/01M2EQC626WZFR69JBN913T41M.planner-ends/`; page evidence:
`snapshot-midrun.json` (taken at lap 2), `snapshot.json`, `page.html`.

## Lap by lap

- Lap 1 (planner.1/turn.1, 32s): task "Create spring haiku". The planner
  wrote a full three-task backlog and chose index 0. Worker turn 8s; the
  worker's claim and the workspace listing `spring.txt (72 bytes)` were
  recorded on `practice.1/task.1`.
- Lap 2 (planner.1/turn.2, 12s): "Create summer haiku", with "do not modify
  the existing spring file" added by the planner. Worker 10s; listing shows
  two files. Mid-run snapshot saved here.
- Lap 3 (planner.1/turn.3, 4s): "Create autumn haiku". Worker 9s; listing
  shows three files.
- Planner.1/turn.4, 3s: `{"tasks":[],"next":null}`; dispatch ended; the loop
  returned nil and the run ended clean. Three files, each one haiku.

## What went wrong

Nothing in the loop itself; this shape did what it says. Two things worth
knowing:

- The planner filled `validation` on every task (`test -f spring.txt` plus a
  query) although this workflow never runs either; the planner writes
  validation because the schema has it, not because anyone asked. It ended
  dispatch on the workflow's own evidence (the `files in the workspace`
  listing), which is what the goal told it to go by.
- `backlog.md` in the run's scope dir ends as `"tasks": []`: the planner
  drops each finished task from the "full revised list", so after the run
  the backlog says nothing about what was done (#194).

No steer or kill was sent in this shape.

## What run.jsonl shows

48 records. By kind: run_started, scope_began x5, session_created x4,
turn_started x7, turn_ended x7, planner_decision x4, value_set x9,
scope_ended x5, session_closed x4, run_ended, complete.

The four decisions, one per planner turn (seq 7, 19, 31, 43); the last one
has no task:

```
{"seq":7,...,"scope":"practice.1","event":{"kind":"planner_decision","task":{"name":"Create spring haiku",...}}}
{"seq":43,"time":"2026-09-14T01:10:36.116693Z","scope":"practice.1","event":{"kind":"planner_decision"}}
```

The evidence the planner ended on, as the workflow set it (a JSON string
value with newlines):

```
{"seq":38,...,"scope":"practice.1/task.3","event":{"key":"files in the workspace","kind":"value_set","value":"\"autumn.txt (84 bytes)\\nspring.txt (72 bytes)\\nsummer.txt (82 bytes)\""}}
```

Every turn_ended carries `usage`; the sum over the seven turns (235301 in,
2899 out, 1505 reasoning, 99328 cache read) equals the snapshot's root
totals and the page header, so the accounting is consistent.

## What the page showed

No browser was available in this session (the Chrome extension was not
connected), so the page was fetched with an HTTP GET the way curl would and
the snapshot JSON with `GET /api/runs/{id}`.

- Shown well: the run header ("Run planner-ends completed", usage line), the
  scope tree (`.` > `practice.1` > `task.1..3`), the four decisions listed on
  `practice.1` by task name with "ended dispatch" last, each task scope with
  its `task`, `worker result` and `files in the workspace` values, the
  planner session with its prompt (the whole planner prompt and backlog are
  readable) and each turn's result.
- Shown badly: scope values are the raw JSON of the value, so the listing
  reads `"spring.txt (72 bytes)\nsummer.txt (82 bytes)"` with quotes and
  `\n` on one line, and the task record is a one-line JSON blob (#193).
- The server-rendered header says "connecting" beside "completed": that is
  the SSE indicator before any script runs, not a fault of the data.
- `snapshot-midrun.json` (lap 2, before the second worker ran) had `run`
  status running, scopes `""`, `practice.1` (one decision), `practice.1/task.1`
  ended with its three values, `practice.1/task.2` running with only `task`
  set, sessions planner.1 and task.1/worker.1, turns planner.1/turn.1-2 and
  the first worker turn, transcripts for each of those turns.

## Issues filed

- #193 Run page shows scope values as raw JSON text, not as the value
- #194 backlog.md is empty after the planner ends dispatch
