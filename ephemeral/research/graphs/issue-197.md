# Run page and snapshot show no steer or killed records: whether a steer landed is invisible on the page

URL: https://github.com/tylergannon/gimble/issues/197
State: open
Milestone: None
Updated: 2026-09-14T01:19:19Z

Found in the loop practice runs (#178): shape 3 run `01M2EQFFPMHJHM9MMZMP8RGT8H.supervisor-objects` and shape 4 run `01M2EQK497YCFAC0JV2K7T66WV.group-in-task`.

run.jsonl records every steer and kill with what an operator wants to know:

```
{"seq":16,...,"turn":"practice.1/task.1/worker.1/turn.1","event":{"kind":"steer","landed":true,"message":"Your supervisor objects:\n\n- Every file you write ...","source":"practice.1/task.1/supervisor.1","target":"practice.1/task.1/worker.1"}}
{"seq":20,...,"scope":"practice.1/task.1/attempts.1/attempt.2","event":{"kind":"killed","by":"operator","target":"practice.1/task.1/attempts.1/attempt.2","reason":"the operator killed attempt 2 by id from a second goroutine"}}
```

**Expected:** the run page (and `GET /api/runs/{id}`, which is the same data) shows each steer on the turn it was sent to, with its source and whether it landed, and each kill on the scope or turn it hit, with who and why; #107 and #176 made those records exist so the page could show them.

**What happened:** `GET /api/runs/01M2EQFFPMHJHM9MMZMP8RGT8H.supervisor-objects` (saved as `ephemeral/attest/loop-practice/supervisor-objects/snapshot.json`, keys run/scopes/sessions/turns/turn_usage/model_calls/totals/transcripts) contains no steer record and no `landed` field at all; the only trace of the three steers is the injected user message inside the worker transcripts, whose message id happens to start with `steer.`. The shape 4 snapshot contains no `killed` record either; the kill is visible only because the scope's and turn's error text quote the `Killed` cause ("context canceled: gimble: practice.1/task.1/attempts.1/attempt.2 was killed by operator: ..."). A steer that was dropped (landed=false) would leave no trace on the page whatsoever.

Held until #173 merges if it touches the run route; filed so the practice notes can point at it.
