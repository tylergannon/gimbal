# Run store proof: result

Date: 2026-09-13 18:16 local. Branch `claude/token-usage-coverall-12e8d5` at
`bbffd66` (Phase 1 `c66bea5`, Phase 2 `bbffd66`, plan revision `01807f0`).
Program: `ephemeral/attest/run-store/main.go`, run as
`go run ./ephemeral/attest/run-store -second codex -hold 12m`.

## Models

| Session | Configured | Resolved (`sessions.json` / `turn_usage.json`) |
| --- | --- | --- |
| `shared.1` (root, then `research.1`) | codex `gpt-5.6-luna` | `gpt-5.6-luna` |
| `compare.1/attempt.1/writer.1` | codex `gpt-5.6-luna` | `gpt-5.6-luna` |
| `compare.1/attempt.2/writer.1` | agy `gemini-3.8-flash-low` | `gemini-3.8-flash-low` |

Gap: the plan's first `compare` attempt is Claude Haiku. The Claude CLI on
this machine is logged out (`claude auth status`: `loggedIn: false`) and
the orchestrator may not enter credentials, so that attempt ran on Codex
(`-second codex`). The Claude columns (cache write, stated cost) are
covered instead by step 4, the saved issue-149 run, whose Haiku session
has both nonzero.

Run id `20260913-181639.run-store-proof`, port 8098, 20.0 s, no error.
Output: `run-01.txt`.

## 1. Live, then finished, without a reload

Captured with `curl` against the SSR page (the same numbers the browser
shows before its EventSource connects; `page-midrun.txt`,
`page-after.txt`).

Mid-run at 18:16:53, while `compare.1` was going and `attempt.1` had not
yet reported (its card read 0 across):

| Line | in | out | reasoning | cache read | cache write | $ |
| --- | --- | --- | --- | --- | --- | --- |
| Header (root) | 26875 | 246 | 54 | 31232 | 0 | 0 |
| `shared.1` card | 12027 | 45 | 16 | 31232 | 0 | 0 |
| `attempt.2/writer.1` card (turn still open, step usage) | 14848 | 201 | 38 | 0 | 0 | 0 |
| `attempt.1/writer.1` card | 0 | 0 | 0 | 0 | 0 | 0 |

`midrun-turns.txt` shows both `compare` turns with `ended: 0` and
`midrun-turn_usage.txt` already holds the Gemini step usage: live
accounting before the report.

After the run (18:17:00):

| Line | in | out | reasoning | cache read | cache write | $ |
| --- | --- | --- | --- | --- | --- | --- |
| Header (root) | 38999 | 499 | 85 | 62464 | 0 | 0 |
| `shared.1` card | 12027 | 45 | 16 | 31232 | 0 | 0 |
| `attempt.2/writer.1` card | 14848 | 201 | 38 | 0 | 0 | 0 |
| `attempt.1/writer.1` card | 12124 | 253 | 31 | 31232 | 0 | 0 |

The header rose by exactly `attempt.1`'s two turns (11609+515 in,
127+126 out, 18+13 reasoning, 9984+21248 cache read).

`ls .gimble/runs/20260913-181639.run-store-proof/`: `model_calls.json
run.json run.jsonl scopes.json sessions sessions.json turn_usage.json
turns.json`. No `observation.json`.

## 2. jq per scope against the page

`jq-per-scope.txt`, one call per scope over `turns.json` and
`turn_usage.json` (the plan's filter), beside the page and the
`turn_ended` records (`turn-ended-records.txt`, seq 5, 8, 17, 20, 22):

| Scope | Model | in | cache read | cache write | out | reasoning | $ | Page line |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `""` | gpt-5.6-luna | 24151 | 62464 | 0 | 298 | 47 | 0 | header: 38999 / 62464 / 0 / 499 / 85 / 0 = sum of both rows |
| `""` | gemini-3.8-flash-low | 14848 | 0 | 0 | 201 | 38 | 0 | (same) |
| `research.1` | gpt-5.6-luna | 413 | 21248 | 0 | 17 | 0 | 0 | seq 8 |
| `compare.1` | gpt-5.6-luna | 12124 | 31232 | 0 | 253 | 31 | 0 | seq 20 + 22 |
| `compare.1` | gemini-3.8-flash-low | 14848 | 0 | 0 | 201 | 38 | 0 | seq 17 |
| `compare.1/attempt.1` | gpt-5.6-luna | 12124 | 31232 | 0 | 253 | 31 | 0 | `attempt.1/writer.1` card |
| `compare.1/attempt.2` | gemini-3.8-flash-low | 14848 | 0 | 0 | 201 | 38 | 0 | `attempt.2/writer.1` card |

Placement: `turns.json` has `shared.1/turn.1` at scope `""` and
`shared.1/turn.2` at scope `research.1`; `research.1` holds `turn.2` and
not `turn.1` (the jq row above is seq 8 alone). The `shared.1` card sums
both turns (12027 = 11614 + 413) because session totals are keyed by
session, as the card is today.

Stated cost is 0 for every row: neither Codex nor Gemini reports a cost.

## 3. Served again by the binary

Stopped the program, ran `go run ../../../cmd -port 8097` in the same
directory, fetched `/runs/20260913-181639.run-store-proof`
(`page-reserved.txt`): header and every card identical to the table
after the run above; five transcripts rendered. The six files' mtimes are
unchanged after the open (`mtimes-before-reserve.txt`, `diff` empty): the
run was read from its tables and nothing was rewritten.

## 4. The saved issue-149 run, rebuilt from its logs

Copied `internal/observation/testdata/issue-149/{run.jsonl,sessions/}`
(not `observation.json`) to `.gimble/runs/20260912-205306.issue-149/` and
fetched its page from the same server (`page-issue-149.txt`). It
rendered with six transcripts; the six table files appeared beside the
logs. Header: 45236 in, 343 out, 220 reasoning, 96549 cache read, 8269
cache write, $0.0249421. `jq-issue-149-root.txt` per model:

| Model | in | cache read | cache write | out | reasoning | $ |
| --- | --- | --- | --- | --- | --- | --- |
| claude-haiku-4-5-20251001 | 20 | 66341 | 8269 | 130 | 220 | 0.0249421 |
| gpt-5.6-luna | 15210 | 30208 | 0 | 112 | 0 | 0 |
| gemini-3.8-flash-low | 30006 | 0 | 0 | 101 | 0 | 0 |

These equal the Phase 1 registry test's literals and sum to the header.

## Files

`main.go`, `run-01.txt`, `serve-01.txt`, `page-midrun.{html,txt}`,
`page-after.{html,txt}`, `page-reserved.{html,txt}`,
`page-issue-149.{html,txt}`, `midrun-turns.txt`, `midrun-turn_usage.txt`,
`jq-per-scope.txt`, `jq-issue-149-root.txt`, `turn-ended-records.txt`,
`mtimes-before-reserve.txt`, and `.gimble/` (the two run directories).
