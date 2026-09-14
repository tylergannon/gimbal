# Shape 2: a validation command decides when a task is done

Second run `01M2EQK5RH2Y04HM8XPWE7P640.validation-command`, 2026-09-13
19:13:06 to 19:14:02 local (55.5s); first run
`01M2EQC7S1CTSVRBX7KRR83SXN.validation-command` (50.8s), kept as
`first-run.jsonl` and `live-first-run.txt`. Planner codex/gpt-5.6-luna,
worker agy/gemini-3.8-flash-low. Goal: `data.txt` with twelve fruit lines and
an executable `count.sh` that prints the line count; every task must carry
`validation.command`; the workflow runs it with `sh -c` in the workspace after
the worker and the exit code alone decides. Program: `main.go`; console:
`live.txt`; log dir: `logs/runs/01M2EQK5RH2Y04HM8XPWE7P640.validation-command/`;
page evidence: `snapshot-midrun.json` (lap 2), `snapshot.json`, `page.html`.

## Lap by lap (second run)

- Lap 1 (planner.1/turn.1, 24s): "Create fruit data file". The planner chose
  the twelve fruits itself and wrote a command that compares the whole file:
  `test "$(cat data.txt)" = "apple<newline>banana<newline>...strawberry"`, with
  real newlines inside the quotes. Worker 7s: wrote data.txt and said it
  "verified it against the validation command". The workflow ran the command:
  exit 0; `validation command` and `passed=true` recorded on task.1.
- Lap 2 (planner.1/turn.2, 10s): "Create executable line-count script",
  command `test -x count.sh && test "$(./count.sh 2>&1)" = 12`. Worker 11s:
  wrote `#!/usr/bin/env bash` + `wc -l < data.txt | tr -d ' '`, chmod +x.
  Exit 0; passed=true. Mid-run snapshot saved at the start of this lap.
- Planner.1/turn.3, 3s: `{"tasks":[],"next":null}`. tasks=2 passed=2
  failed=0 rejected=0, run ended clean.

## What went wrong

- First run: the goal said "plan small tasks" and the planner made one task
  for the whole project with a five-clause command (`live-first-run.txt`).
  It passed, and the planner ended after one lap, but the shape wanted two
  laps, so the goal now says "plan the data file and the script as separate
  tasks, the data file first". The planner follows the goal's words, not what
  the workflow silently expects.
- First run: the watcher printed no `[log]` lines at all. `runlog.Read`
  opened `run.jsonl` before the run had written it, got ENOENT and returned;
  the run directory exists before the first record (#192). `waitRunID` now
  waits for `run.jsonl` itself.
- The worker sees the validation command, because `ScopeText` hands it the
  whole task record, and it ran the command itself before answering. Here
  that is harmless; a worker that can read the test can also teach to it.
- The reject path (a task with no command) was never taken: the planner
  complied in both runs, so "rejected without running a worker" is untested
  by these runs.

No steer or kill was sent in this shape.

## What run.jsonl shows

38 records: run_started, scope_began x4, session_created x3, turn_started x5,
turn_ended x5, planner_decision x3, value_set x8, scope_ended x4,
session_closed x3, run_ended, complete.

The verdict as recorded, beside the worker's claim (seq 13, not quoted):

```
{"seq":14,...,"scope":"practice.1/task.1","event":{"key":"validation command","kind":"value_set","value":"\"$ test \\\"$(cat data.txt)\\\" = \\\"apple\\nbanana\\ncherry\\ndate\\nfig\\ngrape\\nkiwi\\nlemon\\nmango\\norange\\npear\\nstrawberry\\\"\\nexit 0\\n\""}}
{"seq":15,...,"scope":"practice.1/task.1","event":{"key":"passed","kind":"value_set","value":"true"}}
{"seq":27,...,"scope":"practice.1/task.2","event":{"key":"validation command","kind":"value_set","value":"\"$ test -x count.sh \\u0026\\u0026 test \\\"$(./count.sh 2\\u003e\\u00261)\\\" = 12\\nexit 0\\n\""}}
{"seq":33,...,"scope":"practice.1","event":{"kind":"planner_decision"}}
```

The planner's turn.3 took 3s to end dispatch once two `passed=true` values
were on the record; its prompt says recorded deterministic results are
authoritative and it behaved that way. Usage sums (121778 in, 1785 out, 798
reasoning, 124565 cache read) match the snapshot totals and the page header.

## What the page showed

Fetched with HTTP GET (no browser available), snapshot with
`GET /api/runs/{id}`.

- Shown well: `passed · true` on each task scope, the command transcript and
  the worker's claim side by side under the task, three decisions on
  `practice.1` ending with "ended dispatch", the planner's full prompt.
- Shown badly: the command transcript is the value an operator most wants
  to read and it is rendered as one JSON-escaped line,
  `"$ test -x count.sh && test \"$(./count.sh 2>&1)\" = 12\nexit 0\n"`
  (#193). Nothing on the page says which task's verdict is a command result
  and which is prose except the key names the workflow chose.
- `snapshot-midrun.json`: run running, task.1 ended with `task`, `worker
  claim`, `validation command`, `passed`; task.2 begun with `task` only;
  planner.1/turn.1-2 and task.1/worker.1/turn.1 with transcripts.

## Issues filed

- #192 runlog.Read fails at once when the run directory exists but
  run.jsonl is not yet written
- #193 Run page shows scope values as raw JSON text, not as the value
