# Shape 3: a supervisor objects mid-turn and the steer lands

Run `01M2EQFFPMHJHM9MMZMP8RGT8H.supervisor-objects`, 2026-09-13 19:11:05 to
19:16:37 local (5m32s). Planner codex/gpt-5.6-luna, worker codex/gpt-5.6-luna,
supervisor agy/gemini-3.8-flash-low attached with
`WithSupervisor(supervisor, rule, WithInterval(8*time.Second))`. Goal: six
one-sentence planet files, planned as one task; the worker is told to write
each file with its own shell command and `sleep 3` between; the supervisor's
instruction is a house rule the worker was never told (every file must end
with `-- source: memory`) and to object the first time it sees a file
without it. Program: `main.go`; console: `live.txt`; log dir:
`logs/runs/01M2EQFFPMHJHM9MMZMP8RGT8H.supervisor-objects/`; page evidence:
`snapshot-midrun.json` (taken as the first steer landed), `snapshot.json`,
`page.html`.

## Lap by lap

- Lap 1 (planner.1/turn.1, 11s): "Write six planet notes". `supervise_attached`
  at seq 12 (reviewer `practice.1/task.1/supervisor.1`, worker turn
  `practice.1/task.1/worker.1/turn.1`, interval 8s). Worker turn started
  19:11:17. First look at 19:11:24 (turn.1, 9.6s) returned one objection;
  the steer was written at 19:11:34.531 with `landed: true` on the worker's
  turn.1, and the console line `worker.1: steer landed: Your supervisor
  objects: ...` appeared at the same instant. The worker finished at 1m24s
  listing six files; every file ended with the footer. The supervisor looked
  8 times in that turn; looks 2 to 7 returned `{"objections":[]}`; look 8
  was cut off when the worker returned.
- Lap 2 (planner.1/turn.2, 11s): "Complete six planet notes": the planner
  wrote that "existing filenames alone do not demonstrate the required
  content, and the recorded footer metadata is not evidence of sentence
  correctness", with a validation query and no command. The worker rewrote
  the files (1m47s); the supervisor objected again at 19:13:35 (steer #2,
  landed=true) because the first rewritten file had no footer; the worker
  complied again. 9 looks.
- Lap 3 (planner.1/turn.3, 10s): the same task again, now "produce and
  demonstrate the complete contents". Worker 1m43s; steer #3 at 19:15:18
  landed=true; footer on all six again. 12 looks.
- Planner.1/turn.4 (5s): dispatched the same task a fourth time. The
  program's cap of 3 tasks was hit, the loop was left, and `task.4`'s scope
  began and ended with nothing in it (seq 114-116). The run ended clean
  with 4 tasks yielded, 3 worked, 3 steers, all landed.

## Which steer was sent and what landed said

Three steers, one per worked task, all from the task's supervisor to the
task's worker, all `landed: true`. The first:

```
{"seq":16,"time":"2026-09-14T01:11:34.531384Z","scope":"practice.1/task.1","session":"practice.1/task.1/worker.1","turn":"practice.1/task.1/worker.1/turn.1","event":{"kind":"steer","landed":true,"message":"Your supervisor objects:\n\n- Every file you write in this directory must end with a final line reading exactly `-- source: memory`. End every remaining file with that line and append `-- source: memory` as the final line to the files you already wrote.","source":"practice.1/task.1/supervisor.1","target":"practice.1/task.1/worker.1"}}
```

The steer's effect is provable from outside the model: the `files in the
workspace` value the workflow set after each task lists `footer=true` for
all six files, and the files on disk end with the line.

## What went wrong

- The loop never ended on its own; the cap did. The planner's evidence each
  lap was the worker's list of filenames and the workflow's `footer=true`
  flags; neither shows a sentence about a planet, so the planner's
  definition of done ("demonstrated by inspecting the file contents") was
  never met on the record and it re-dispatched the same task three times.
  Its session's working directory is the workspace and its prompt says
  "inspect the workspace only to plan", but it did not read the files. The
  management lesson: record the evidence the definition of done needs (the
  contents, or a command's result), or the planner spins to the cap.
- The supervisor's rule contradicts the goal. "One sentence each" plus a
  footer line is two lines, and the planner never sees steers, so from its
  side the footer is unexplained "metadata" it had to reason around. A house
  rule enforced by a supervisor has to be in the goal too, or the planner
  and the supervisor pull the worker in different directions.
- The supervisor objected once per task, then kept looking every 8s and
  found nothing: 29 supervisor turns against 4 planner and 3 worker turns,
  and 356750 of the run's 484685 input tokens went to the supervisor
  sessions. For a worker whose steps are three seconds apart, 8s is a lot of
  looking.
- Each task's final look ended `error: "context canceled", interrupted:
  true` because `Generate` cancels the looks when the worker returns; three
  routine wind-downs are recorded as errored turns, indistinguishable in
  shape from a killed turn (#196).
- The worker on laps 2 and 3 rewrote all six files from scratch instead of
  checking them, and each time the supervisor had to object again. A task
  that says "ensure" reads as "redo" to this model.

## What run.jsonl shows

121 records: 4 planner_decision, 3 supervise_attached, 3 steer, 36
turn_started / 36 turn_ended (4 planner, 3 worker, 29 supervisor), 10
value_set (task.4 got only its `task`), 6 scope_began / 6 scope_ended, 7
session_created / 7 session_closed, run_started, run_ended, complete.

The attach, the last look, and the decision the cap cut off:

```
{"seq":12,...,"turn":"practice.1/task.1/worker.1/turn.1","event":{"instruction":"House rule for this directory: every file the agent writes must end with a final line reading exactly `-- source: memory`. ...","interval":8000000000,"kind":"supervise_attached","reviewer":"practice.1/task.1/supervisor.1","worker":"practice.1/task.1/worker.1/turn.1"}}
{"seq":31,...,"turn":"practice.1/task.1/supervisor.1/turn.8","event":{"duration":4236547750,"error":"context canceled","interrupted":true,"kind":"turn_ended","result":"","usage":[{"model":"gemini-3.8-flash-low",...}]}}
{"seq":113,...,"scope":"practice.1","event":{"kind":"planner_decision","task":{"name":"Complete six planet notes","description":"Complete and demonstrate the contents of planet1.txt through planet6.txt ... The recorded result confirms only filenames and footer metadata, so the required note content ...
```

Usage sums over the 36 turns (484685 in, 10684 out, 12002 reasoning,
2477825 cache read) match the snapshot totals and the page header. The
supervisor's cut-off looks do carry usage.

## What the page showed

Fetched with HTTP GET (no browser available); snapshot with
`GET /api/runs/{id}`; `page.html` is 632K because every supervisor look's
prompt and transcript are inlined.

- Shown well: four decisions on `practice.1`, the three worked tasks with
  `task`, `worker result` and `files in the workspace`, the empty `task.4`,
  each supervisor session with its looks, and the worker's transcript in
  which the steer is visible as an injected user message ("Your supervisor
  objects: ...", message id beginning `steer.`).
- Not shown: any steer record. `snapshot.json` has no `kind: steer` and no
  `landed` anywhere; the only trace of the three steers is the injected
  message inside the worker transcripts. A dropped steer would leave nothing
  on the page (#197). The `supervise_attached` record is not shown either;
  the supervisor session is listed with no link to the turn it watched.
- Shown badly: the `files in the workspace` value as a JSON-escaped one-liner
  (#193); the three cancelled looks as errored turns (#196).
- `snapshot-midrun.json` (taken as steer #1 was written): run running,
  `practice.1/task.1` running with only `task` set, the worker's turn.1
  running (`ended: 0`), supervisor turn.1 ended with the objection as its
  result, supervisor turn.2 started at 01:11:34.531, the same millisecond
  as the steer, transcripts for the planner turn, both supervisor turns,
  and the worker turn so far.

## Issues filed

- #193 Run page shows scope values as raw JSON text, not as the value
- #196 The supervisor's last look ends as an errored, interrupted turn when
  the worker finishes
- #197 Run page and snapshot show no steer or killed records: whether a
  steer landed is invisible on the page
