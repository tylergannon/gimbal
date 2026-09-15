# Native agy ERROR tool probe

The prompt asked Gemini Flash to read a nonexistent `missing.md`, then the included `valid.md`. The native stream reports `view_file` step 2 as `ACTIVE` followed by `ERROR`, with `tool_info.error.type=TOOL_ERROR` and a `no such file or directory` message (`raw.jsonl` lines 4-5). A later `view_file` step 4 reports `ACTIVE` then `DONE` (`raw.jsonl` lines 7-8), and the final result is `SUCCESS`. Stderr is empty.

Gimble's agy projector previously treated only `DONE` as terminal. It emitted a tool call for the failed read but ignored the explicit `ERROR` update, leaving that step open and rejecting the later successful result as an unsettled tool. The adapter now records `ERROR` with its native detail as `session.tool.failed` and ends the step. A tool still `ACTIVE` at result remains a protocol error. The projector regression uses the same ACTIVE/ERROR, ACTIVE/DONE, SUCCESS sequence.

The raw stream, stderr, and CLI log stay local and are ignored by Git. `command.txt` records the exact command; `valid.md` is the source fixture.

The separate `agy-probe-gimble` binary exercised the same prompt through real `gimble run-prompt` using `gemini-3.8-flash-low`. It exited 0 with the expected summary in `gimble-output.txt`. The run record `01M2JRZX1XATRGMDFHMSSBH47R.run-prompt` contains `session.tool.failed` for missing `view_file` item 2, then `session.step.ended` for that item, later `session.tool.success` for valid-file item 4, and a clean `run_ended` plus `complete` event. `gimble-stderr.txt` names the model and reports the turn and run ended `ok`. These records are local and ignored; the output and stderr are included for review.
