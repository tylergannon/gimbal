# Run store: merge notes

How `docs/sprints/RUN-STORE.md` was assembled from the three drafts and
three critiques, and which interview questions were answered on Tyler's
behalf because he was away. Tyler's instruction for this planning run:
no numbering, no ledger, no token cache; no widening of scope; no proof
machinery.

## Process notes

- The Claude lanes (draft and critique) ran as Agent-tool subagents on
  claude-fable-5-1, not through `claude -p`: the CLI's OAuth session is
  expired and cannot be refreshed from here. The Codex lane ran
  `codex exec --model gpt-6-astra`; the Gemini lane ran `agy -p`.
- Every code-level claim that decides a design point was checked against
  the checkout by the orchestrator before merging (see the worklog entry
  of 2026-09-13 and the Claude critique's "What the code says" table).

## Where the drafts agreed

Everything the intent fixed: seven tables, six files, rows in Go maps in
`internal/observation`, totals in Go over `turn_usage` by prefix
containment, a `totals` frame, the ended-turn accounting rule, the
placement correction, `observation.json` and `checkpoint.go` deleted, no
tree, no prices, no database, no message table, no change to adapters,
the log format, `internal/runlog`, or the reducer ports, no proof
machinery. All three answered open question 4 the same way: the tables
live in `internal/observation`.

## Where they disagreed, and what was taken

| Point | Claude | Codex | Gemini | Taken | Why |
| --- | --- | --- | --- | --- | --- |
| Opening a finished run | Replay the log on open | Load the six files, hydrate transcripts from session logs, rebuild only when files are missing | Replay the log on open | Codex (reversed by Tyler) | First taken as replay on open (see interview answer 1). Tyler rejected that: the tables are the data model and Gimble must read them; replay re-derives old runs' numbers through the current fold. Session logs are still read on every open, for transcripts only. |
| Replay ordering | `run.jsonl` first, then session logs, with a guard: step usage is ignored on a turn that has ended | Group native records per turn and feed them between `turn_started` and `turn_ended` | `run.jsonl` then session logs, no guard (double counts) | Claude's guard | One condition replaces an interleaver. Live order already puts every step before its report (`session.go`), so the guard changes nothing live. |
| Write cadence | Dirty set plus an in-flight-absorbing flusher | Synchronous under the store lock, once per accepted record, nothing on deltas | Synchronous outside the lock | Codex | The lock is the ordering; outside it two `Group` producers can rename out of order. The flusher is more code and its pseudocode clears dirty bits before the write succeeds (Codex critique). Files are small; the hold is sub-millisecond. |
| Frames | `snapshot`, `row`, `totals`, `event` | `snapshot`, `state` (whole fact maps), `event` | `snapshot`, `lifecycle`, `totals`, `event` | Claude's `row` and `totals` | The browser fold is deleted either way; `row` is the smaller payload on the hot path (the Claude critique sized `state` at every prompt and result per step end against the 4MB subscriber bound). Table names in the frame equal the snapshot's keys (Codex critique). |
| Lifecycle decoding | In the store from the record JSON | Same | `runlog.Read[LifecycleRecord]` from observation | In the store | `gimble` imports `observation`; the reverse is an import cycle. |
| Log reading | A line reader in `replay.go`; `runlog` untouched | A finite reader added to `runlog`, incomplete log is an error | `runlog.Read` (tails forever on a log without `complete`) | Claude | `runlog` keeps its tailing contract; a crashed run renders what it logged. |
| Finished stores in memory | A `finished` map beside `runs` | Replay per request; serialize per run | Unspecified cache | One registry map, kept after close | Fewer lines than a second map; a run that finished in this process is never replayed; opens are serialized under the registry lock. |
| Session log discovery | Walk `sessions/` at any depth | Read `sessions/<id>.jsonl` by recorded id | `sessions/*/*.jsonl` glob | By recorded id after `run.jsonl` replays | No walk, no glob; the glob misses root and depth-two sessions. |
| Decisions in the scope row | Raw event only | (not detailed) | `{seq, body}` | `{seq, body}` | `seq` is a column of the intent's `decision` table and lives on the outer record. |
| Generated files after deleting the remote | Delete by hand if the generator leaves them | "Through the generator" | Not listed | Delete by hand | skgo prunes only the link tree. |
| `web/observation_live_test.go` | Delete | Retarget at the SSE endpoint | Not listed | Retarget | It is the one HTTP-level test of the observation stream; the retarget is small. |
| Open rewrites files | Always | Only when missing or unreadable | Always | Only when any of the six is missing | One condition; viewing a tracked run directory must not dirty the tree. |

## Corrections applied to the base draft

Taken from the critiques, all within scope:

- Initialize all six files as `[]` at `Open`, so a run with no steps still
  has `model_calls.json` (Codex critique).
- `Store.Lifecycle` returns the write error; `run.go` passes it to
  `recordFailure` as `observeAgent` already does (Codex critique).
- Phases follow compilation: the Go store and every Go file that names a
  removed type (`usage.remote.go`, the two web Go tests) change in one
  phase; the browser is the next; no temporary compatibility types.
- Saved issue-149 run: four scopes including the root, not three; tests and
  the proof replay a copy, never the tracked directory.
- `RunViewer.svelte` registers the new frame names on the EventSource;
  the decision line reads `body.task`.
- Session totals are keyed by the turn's placement session (`turn.session`),
  as the card does today.
- The turn-boundary write fallback in the Claude draft's risks is dropped:
  it would change the fixed cadence.

## Rejected criticisms

- Codex draft's "serialize lifecycle append plus fold": records from
  different goroutines touch different keys; nothing to order.
- Gemini draft's `tables_test.go` "jq-parseable formatting" tests: one
  decode of the written file is the test.
- Any suggestion of a reconcile script, fixture harness, performance
  report, or rename counting as an acceptance item.

## Interview answers taken on Tyler's behalf

1. **Decision 3 says "loads the files into the same maps"; open question 1
   offered replay with no loader.** Answered on Tyler's behalf: replay on
   open, files written as output. **Reversed by Tyler on 2026-09-13**: the
   answer abdicated the data model; Gimble reads the six files, reads the
   session logs for transcripts only, and replays the lifecycle log only
   to rebuild a directory with a file missing. The lanes had conflated the
   transcript read (needed) with re-deriving facts (not). The final
   document's § Opening a finished run and open question 1 carry the
   corrected decision.
2. **Where the proof program lives.** `ephemeral/attest/run-store/`, set
   up like `ephemeral/attest/issue-149/`; a program to run the workflow is
   the proof run, not proof machinery.
3. **Whether `web/observation_live_test.go` survives.** Retargeted at the
   SSE endpoint rather than deleted.

## Execution

Per Tyler: an Opus subagent builds; `agent -provider openai -model
gpt-5.6-sol` runs `/adversarial-review`; the orchestrator plans, watches
both, and rejects any criticism that widens or deepens scope or asks for
proof machinery.
