# backlog.md is empty after the planner ends dispatch: finished tasks leave no record in the backlog

URL: https://github.com/tylergannon/gimble/issues/194
State: open
Milestone: None
Updated: 2026-09-14T01:14:26Z

Found in the loop practice runs (#178): shape 1 run `01M2EQC626WZFR69JBN913T41M.planner-ends` (also shapes 2 and 4).

The planner did what was asked: it planned three tasks, dispatched them one at a time, and ended dispatch when all three files existed. On each lap it returned the "full revised task list" as the planner prompt asks, and each time it dropped the task that had just been done; on its last lap it returned `{"tasks":[],"next":null}`. `Loop` writes that answer to `scopes/practice.1/backlog.md`, so after the run the file is:

```
---
{
  "goal": "...",
  "tasks": []
}
---
```

**Expected:** an operator who opens `backlog.md` after a run (or while one is going) can see what the loop has done and what is left. `go doc` says the Loop "keeps a revisable backlog in its run scope".

**What happened:** the backlog has no notion of a finished task, so the only way the planner can keep the list honest is to remove what it finished; the record of the run's work then lives only in `run.jsonl` (`planner_decision` records) and on the page. Mid-run the file is equally silent about what was done: after task 1 it held tasks 2 and 3 only.

Either a task carries a status the planner sets (done, failed, deferred) and `backlog.md` keeps every task, or `backlog.md` is documented as "what is left" and the loop writes a companion record of decisions taken; the first matches the planner prompt's "keep deferred defects visible".
