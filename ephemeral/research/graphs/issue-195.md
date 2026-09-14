# A killed turn records zero usage: attempt.2's three seconds of gpt-5.6-luna cost nothing on the page

URL: https://github.com/tylergannon/gimble/issues/195
State: open
Milestone: None
Updated: 2026-09-14T01:18:51Z

Found in the loop practice runs (#178): shape 4 run `01M2EQK497YCFAC0JV2K7T66WV.group-in-task`.

Three Codex attempts (gpt-5.6-luna) started at once; an operator goroutine killed attempt 2 by id three seconds in (`runtime.KillScope(run, "practice.1/task.1/attempts.1/attempt.2", "operator", ...)`). The kill worked: the `killed` record is at seq 20, the turn ended at seq 21 with `interrupted: true`, the scope ended with the `Killed` cause, and the loop went on.

**Expected:** the killed turn's usage shows what the model consumed before the kill (it had received a prompt and run at least one tool call), so the run's totals say what the run cost.

**What happened:** the snapshot's `turn_usage` for `practice.1/task.1/attempts.1/attempt.2/writer.1/turn.1` is `{"gpt-5.6-luna": {"input": 0, "cache_read": 0, "cache_write": 0, "output": 0, "reasoning": 0, "stated_cost": 0}}`, its `model_calls` entry carries one message with every field 0, and the session total for `attempt.2/writer.1` is 0 across the board. The two attempts that finished report 14609 and 5028 input tokens. The run header's totals therefore leave out one of three attempts.

Probably the Codex app-server reports token counts at the end of a turn and an interrupted turn never gets that report; if so the adapter should read the last `tokenCount` it saw when it interrupts, or the page should mark the turn's usage as unknown rather than 0. Worth deciding which before a bake-off workflow kills losers by design and every loser costs "nothing".
