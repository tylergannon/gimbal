# Loop practice runs: four shapes on the cheap tier (#178)

Four small programs, each a `Loop` run through `web.NewRuntime` on a loopback
port against a throwaway goal in a scratch `workspace/` (gitignored), on
codex/gpt-5.6-luna and agy/gemini-3.8-flash-low. Each shape's directory has
`main.go`, `notes.md` (lap by lap, what went wrong, what the log and the page
showed, issues filed), `live.txt` (the console), `logs/` (the run directory
as committed), `snapshot-midrun.json` and `snapshot.json`
(`GET /api/runs/{id}`), and `page.html` (the run page as fetched; no browser
was available in this session, so the page was read as server-rendered HTML
plus the snapshot JSON). Run any shape with `go run ./ephemeral/attest/loop-practice/<shape>`.

**planner-ends** (`01M2EQC626WZFR69JBN913T41M.planner-ends`, 1m19s): a Codex
planner dispatches three haiku files one task at a time to a flash worker and
ends dispatch on its own after the third. The workflow records the worker's
claim and a listing of the workspace after each task; the planner's fourth
turn returned `next: null` in three seconds on that listing. Nothing went
wrong; the notes record that the planner fills `validation` on every task
whether or not anyone runs it, and that `backlog.md` is empty after the run
because finished tasks are dropped from the revised list (#194).

**validation-command** (`01M2EQK5RH2Y04HM8XPWE7P640.validation-command`,
56s): every task must carry `validation.command`; after the worker, the
workflow runs it with `sh -c` in the workspace and records the command, its
exit code and output, and `passed`. Two tasks (data file, then script), both
exit 0, and the planner ended dispatch three seconds after the second
`passed=true`. The first run made one task for the whole project because the
goal only said "plan small tasks"; the goal now names the two tasks. The
first run also exposed the watcher race (#192): `runlog.Read` opened
`run.jsonl` before the run wrote it.

**supervisor-objects** (`01M2EQFFPMHJHM9MMZMP8RGT8H.supervisor-objects`,
5m32s): a flash supervisor on an eight-second interval watches a Codex worker
for a house rule the worker was never told; it objected once per task and
each steer landed (`landed: true` on the worker's turn, and the files on disk
prove the worker obeyed). The loop did not end on its own: the planner had
only filenames and footer flags as evidence, never read the files itself, and
re-dispatched the same task until the program's cap. The supervisor's rule
also contradicted the goal, which the planner cannot see steers to
understand. Three routine wind-down looks are recorded as errored turns
(#196), and the page has no steer record at all (#197).

**group-in-task** (`01M2EQK497YCFAC0JV2K7T66WV.group-in-task`, 56s): the one
task runs three Codex attempts in a `Group`; a second goroutine that knows
only the run id follows the log and kills attempt 2 by scope key through
`runtime.KillScope` three seconds in. The `killed` record names by, target and
reason; the attempt's turn and scope end with the `Killed` cause; siblings
finish; `group.Wait` returns the Killed error, which the task treats as
non-fatal, recording all three outcomes and a winner; the planner then ends
dispatch. The killed turn's usage is zero (#195) and the page shows the kill
only inside error text (#197).

## What a loop operator needs

Distilled from the notes; the first six are about running a loop, the rest
about reading one.

1. The run id and the log's first record, before anything else. The run
   directory appears before `run.jsonl`; a watcher that starts on the
   directory misses the whole run (#192). Wait for the file.
2. Evidence the planner can judge the definition of done on, recorded by
   the workflow. Filenames and flags are not contents; a planner that cannot
   see proof re-dispatches the same task until the cap (shape 3). A command
   exit and its output recorded as values end a loop in seconds (shape 2).
3. A deterministic verdict beside the worker's claim, never in its place:
   `validation command` (the command, exit, output) and `passed` as values on
   the task scope. The planner's prompt already treats such records as
   authoritative.
4. A task cap in the workflow and the shape in the goal. The planner obeys
   the goal's words ("the data file first", "plan this as one task"), not what
   the workflow silently expects (shape 2's first run); the cap is what ends
   a loop the planner will not.
5. House rules where the planner can see them. A rule enforced only by a
   supervisor reaches the worker as a steer and the planner never; the
   planner then has to reason around evidence it cannot explain (shape 3).
6. Ids it can act on from outside. Scope, session and turn keys are
   predictable from the code (`practice.1/task.1/attempts.1/attempt.2`), and
   `KillScope` works from a goroutine holding only the run id; the task, not
   the runtime, decides whether a killed attempt is fatal (shape 4).
7. Who steered or killed whom, and whether it landed, on the page. Today
   that is only in `run.jsonl` (`steer` with `landed`, `killed` with by and
   reason); the page shows a steer only as an injected transcript message and
   a kill only as quoted error text (#197).
8. Values readable as text. The page renders scope values as JSON-escaped
   one-liners, which is unreadable for the command transcript an operator
   reads first (#193).
9. A backlog that keeps what was done. `backlog.md` ends as `"tasks": []`
   when the planner ends; the run's history lives only in the log and the
   page (#194).
10. Errors that are errors. A supervisor look cancelled because the worker
    finished is recorded like a killed turn (#196); the killed attempt's
    tokens are recorded as zero (#195); a `Group` whose loser was killed by
    design shows as an errored subtree under a clean task.
11. The supervisor's interval sized to the worker's step. Eight seconds
    against three-second steps gave 29 supervisor turns for 3 worker turns
    and three quarters of the run's input tokens (shape 3).
