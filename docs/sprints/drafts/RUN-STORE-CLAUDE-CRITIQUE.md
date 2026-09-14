# Critique: the two run store drafts

Reviewed against [RUN-STORE-INTENT.md](RUN-STORE-INTENT.md) and the code on
this branch (`internal/observation`, `run.go`, `session.go`,
`event_persistence.go`, `scope.go`, `internal/runlog`,
`web/src/lib/observation`, the skgo v0.4.0 generator in the module cache,
and the saved run under `ephemeral/attest/issue-149`). The intent's six
decisions are taken as fixed. Where the drafts make a claim the code can
answer, the code answers below; the verdicts on the four open questions
follow from those answers. The rule applied throughout: prefer less code.

## What the code says

| Claim | Who | Code | Verdict |
| --- | --- | --- | --- |
| Every step event is written before `turn_ended` | Codex (implied by its replay order) | `session.go`: `wrapped` calls `scope.run.sessionEvent` synchronously; the adapter delivers through it before `RunTurn` returns (`claude/claude.go` `waitTurn` loop, `codex/rpc.go` message channel); the `TurnEnded` record is written after `RunTurn` returns, after the stated-cost `session.usage.updated` and the terminal execution event | True. In a live run no step usage arrives after the turn's report |
| `runlog.Read` tails until `complete` | Codex | `internal/runlog/reader.go`: at EOF it sleeps 10ms and retries until `kind == "complete"` or the ctx ends; it opens only `run.jsonl` | True. It cannot read a session log, and a log with no `complete` never returns |
| Session logs are `sessions/<id>/*.jsonl` | Gemini (and the intent's own gloss) | `event_persistence.go:143` writes `sessions/<session id>.jsonl`; `scope.go:65` makes the id `<scope key>/<name>.<n>`. Saved run: scope `agy.1`, session `agy.1/agy.1`, file `sessions/agy.1/agy.1.jsonl` | The nesting is the id's slashes, not a per-session directory. A root session is `sessions/agy.1.jsonl`; a depth-two scope gives `sessions/sprint.1/task.2/coder.1.jsonl`. A `sessions/*/*.jsonl` glob misses both |
| skgo regeneration removes obsolete generated artifacts | Codex ("through the generator") | skgo `internal/gen/types.go` `generateTypes` writes one `types.ts` per package with a type on the wire; the only removal in the generator is `links.go` pruning the link tree | False. Deleting `usage.remote.go` leaves `[runID]/usage.remote.ts`, `[runID]/types.ts` (`ScopeRef`), and `web/src/lib/skgo/observation/types.ts` (`Usage`, `Tokens`, `Cache`) on disk. `index.ts` imports `Usage` from the last one |
| The browser duplicates Go's sum rule | Gemini ("duplicate sum implementations and drift") | `index.ts`: three frames. `snapshot` replaces; `event` applies the projection, sets `run.usage[session]` on `session.usage.updated`, folds provenance; `lifecycle` folds eight record kinds (not `turn_started`/`turn_ended`). No sum anywhere; the sum is `usage.go` `ScopeUsage` in Go, reached through the `scopeUsage` live query | False. The browser mirrors the fold, not the sum. Gemini's Phase 4 deletes `rollup`, `contains`, "manual scope reduction", none of which exist |
| Step usage carries no model, so in-flight rows use the session's model | Gemini (risk table) | Saved log: `session.step.started.data.model.id` is `claude-haiku-4-5-20251001`; `session.step.ended` has tokens, cost and `assistantMessageID`; both carry `created` in Unix ms | False. The step's model is in the log. Codex's "remember the model at `step.started`, key by message" is right. Under Gemini, `model_call.model` would be wrong for good, since model calls are never replaced |
| Replay can call `runlog.Read[LifecycleRecord]` from `internal/observation` | Gemini (Phase 3) | `LifecycleRecord` and its `UnmarshalJSON` (`jsonschema_gen.go`) are in package `gimble`; `gimble` imports `observation` (`run.go`) | Impossible: import cycle. Observation needs its own decoder of the record JSON (`kind` is snake case per `schema.go` `SealedUnion("kind", polytype.Snake)`, so `planner_decision`, `turn_ended`). Codex plans exactly this |
| `TurnStarted`/`TurnEnded` reach the store today | Both assume not | `run.go` `observeLifecycle` has no case for them; they reach `Store.Lifecycle` with only `Record` set and fold nothing | Correct assumption |
| Files written outside the lock are safe at this scale | Gemini | `Store.mu` is the only serialization of mutations; `Group` runs sessions concurrently; two mutations that snapshot a table, unlock, then rename can rename out of order | Not safe without a second serializer. See open question 3 |
| The saved run can be replayed in place | Gemini (DoD 5, `replay_test.go`) | `git ls-files` tracks the six files under `ephemeral/attest/issue-149/logs`, including `observation.json` | Writing six table files there dirties the tree on every test run. Codex's temporary copy is right |
| Existing tests pin the checkpoint | Codex (enumerates them) | `store_test.go:263` `TestCheckpointIsWrittenOnlyAtClose`; `usage_test.go` reads back through the checkpoint path; `web/observation_live_test.go:104` drives the `scopeUsage` remote by its manifest id | Codex lists all of these. Gemini lists none of `web/observation_live_test.go`, `gimble_test.go`, or the checkpoint test; deleting the remote breaks `go test ./...` until the live test is retargeted |

One more fact both drafts need and neither states: `turn_ended.duration` is
nanoseconds in the log (`4042303625` in the saved run) and `time` is
RFC 3339; both convert to ms. `value_set.value` is JSON source text inside a
string, as `index.ts` `parseValue` already handles.

## Codex draft

**Architecture.** Sound and grounded. The three choices that matter are
right for the code as it is: one decoder of the exact record JSON inside
`observation` serving both live and replay (required, since observation
cannot import `gimble`); writes once per accepted record under the store
lock with nothing on the delta path; and a `state` frame that replaces the
`lifecycle` frame so `foldLifecycle`, `LifecycleRecord`, `parseValue`,
`scopeAt` and the `usageUpdated` line leave `index.ts`. The sibling-prefix
rule (`task.1` excludes `task.10`), wholesale replacement including the
empty report, step model from `step.started`, failed steps qualifying only
with both cost and tokens (`session.go:stepUsage`), deterministic row order,
and a write-error path into `recordFailure` are all correct and all
checkable.

**Completeness.** The most complete of the two on the edges: test
inventory, the live test retarget, the finite read problem, the generated
files, the temp copy of the saved run, and `Close` ordering. Two errors of
mechanism. First, "remove obsolete generated usage-query artifacts through
the generator" cannot happen; the three stale files are deleted by hand and
`index.ts` declares `Usage` locally (the snapshot already crosses as opaque
JSON through `hooks.RunSnapshot`, so nothing else needs the generated type).
Second, "serialize lifecycle append plus fold so concurrent producers
publish in log order" solves a problem that does not exist: records from
different goroutines touch different keys and commute; records for one
scope or turn come from one goroutine. Drop it; it widens the writer's lock
into the store's.

**Phasing.** Four phases, each leaving something runnable: facts and
write-through, saved runs, page, proof. The order is right; Phase 1 alone
already produces the six files on a live run.

**Risk coverage.** Names the real one: replaying provisional step usage
after the terminal report. Its fix (group native records by turn and feed
them between `turn_started` and `turn_ended`) is more code than needed; see
below. The "close/open race" risk is handled by keeping the store
registered until the last record is written, which is fine.

**Feasibility.** Feasible as written, with two items to cut (the runlog
refactor and the interleaved replay) and one to correct (generator claim).
Its finite reader that "returns an explicit incomplete-log error" would make
a crashed run unopenable; a crashed run should render what its log says.

**Definition of done.** Mirrors the intent's criteria and adds two useful
sharpenings: files are written while the run is live (not at close), and
the arithmetic is checked from `turns.json` and `turn_usage.json` by hand.
Item 7's exclusions match the intent's out-of-scope list.

**Keep from Codex:** the local record decoder; the `state` frame replacing
`lifecycle`; synchronous writes under the lock, none on deltas; step model
and times from the stamped step events; the write-error path; deterministic
row order; six files initialized at open so an empty run still satisfies
criterion 1; the temp copy of the saved run; retargeting the live test at
the SSE endpoint.

**Weaknesses:** the generator claim; the unnecessary runlog refactor; the
unnecessary interleaving; the incomplete-log error; the `state` frame's
size on every step end (see open question 2); the loader plus a
projection-only hydration path, which is the more-code answer to open
question 1.

## Gemini draft

**Architecture.** The shape is the intent's, restated at length. Where it
departs from restatement it is wrong in ways the code shows: replay through
`runlog.Read[LifecycleRecord]` from `observation` cannot compile; the
session log glob misses root and depth-two sessions; writes outside the
lock reorder under `Group`; the step-model claim is false; the "browser has
duplicate sums" premise is false. The replay order (all of `run.jsonl`, then
all session logs) reintroduces provisional step usage after every turn's
report was applied, so replayed `turn_usage` double counts. Codex names this
exact risk; Gemini's plan has the bug and no risk row for it.

**Completeness.** Thin where it counts. No decoder for record JSON in
observation. No session log reader (there is none in the repo today). No
write-error path. No row ordering, so `encoding/json` over Go maps writes
arrays in random order and the files churn on every rewrite. No mention of
`web/observation_live_test.go`, `gimble_test.go`, or the checkpoint test.
Deletes `usage.remote.ts` by hand but not the two `types.ts` files. "Subsequent
requests can be served directly from memory" implies a cache of replayed
stores that is never specified; the simple answer is replay per request
(the SSR load and the SSE connect both call `Registry.Snapshot`, so two
replays of about 150KB per page view, which is fine).

**Phasing.** Five phases, sensible order. Phase 1's `tables_test.go` with
"jq-parseable array formatting" tests is proof machinery in miniature; a
single decode of the written file is enough. Phase 5 names a
`ephemeral/attest/run-store-proof/main.go` "temporary proof script"; a
program to run the proof workflow is needed either way, so the objection is
only to keeping it.

**Risk coverage.** The table has five rows and misses the replay ordering
bug, the concurrent-open race its own always-write design creates (two
requests replaying the same run rename the same `.json.tmp`), and the
generated-file leftovers. The "step model" row is backwards, as above.

**Feasibility.** Not feasible as written: the import cycle stops Phase 3
before it starts, and the glob and ordering bugs would ship silently. All
three are cheap to fix, but the draft does not know they are there.

**Definition of done.** Adequate and close to the intent's criteria. Item 6
("the browser contains no prefix-matching or sum functions") is already true
today. Item 5 replays the tracked saved run in place, which dirties the tree.

**Keep from Gemini:** replay the whole log on open as the one open path
(the less-code answer to open question 1, with the guard below); a small
`totals` frame on usage change, which is the intent's own wording in
decision 3; the `Store` field sketch (turn usage as `turn -> model -> row`).

**Weaknesses:** everything under Feasibility; the false premises in the
Overview; the missing test inventory; writes outside the lock.

## Where they disagree

### 1. Finished-run transcripts: replay on open (Gemini) vs load tables and hydrate (Codex)

Less code: replay on open. The decisive fact is decision 5: transcripts
live only in the session logs, so opening a finished run reads every
session log regardless of which draft is chosen. In the saved run the
session logs are 152KB and `run.jsonl` is 29 lines. Loading the tables
saves re-folding those 29 lines and costs a loader (six decodes plus map
fills), a projection-only variant of `Store.Event` (apply projection and
provenance, skip accounting), and a second open path that must agree with
the replay path. Replay on open has one path, needs no loader, and makes
decision 4 automatic: every open rebuilds, so a fold change reaches old runs
the next time they are opened, which is what "derived and rebuildable"
means in practice.

Safer: a draw once each is fixed, so less code wins. Replay on open needs
three small things Gemini does not plan:

- The record decoder in observation (Codex's; both need it).
- One guard: a step's usage is ignored for a turn that already has an
  `ended`. Live order guarantees no step usage follows the report
  (`session.go`, above), so the guard changes nothing live and makes
  replay order-insensitive for accounting. Model call rows are still
  appended; they are facts, not sums. This replaces Codex's per-turn
  interleaving entirely.
- `os.CreateTemp` in the same directory for the temp file, so two
  concurrent opens of the same finished run cannot collide on one `.tmp`
  name. Row order is deterministic, so both write identical bytes.

Session log paths need no walk and no glob: after `run.jsonl` replays, the
session table holds every id, and each log is `sessions/<id>.jsonl`.

Note the wording tension: decision 3 says "opening a finished run loads the
files into the same maps", while open question 1 offers "no loader" as a
candidate. This critique reads the open question as governing and the
decision's sentence as describing the outcome (same maps, live and
finished). If Tyler reads decision 3 literally, Codex's loader is the
answer and the guard still replaces the interleaving.

### 2. Frames: `state` with whole fact maps (Codex) vs `lifecycle` plus `totals` (Gemini)

Codex, on the intent's own test: `index.ts` must shrink. Keeping `lifecycle`
keeps `foldLifecycle` (about forty lines, eight kinds) and adds a `totals`
case; net growth. Replacing `lifecycle` with a `state` frame deletes the
fold, the `LifecycleRecord` type and two helpers, and the browser's
`run.usage` fold goes with them.

One cost Codex does not size: a `state` frame carries every turn's `prompt`
and `result`. On a sprint-sized run (hundreds of turns, multi-kilobyte
prompts) that is megabytes per frame, and Codex publishes it on every step
end. The subscriber bound is 512 frames or 4MB (`subscribe.go`); a browser
three frames behind overflows and reconnects to a fresh snapshot, then
overflows again. The fix within the intent is to publish `state` only when a
lifecycle record is accepted (at most two per turn plus scope, session,
value and decision records) and a small `totals` frame (scope and session
totals only) when a step changes `turn_usage`. `turn_ended` changes
`turn_usage` too and is a lifecycle record, so it publishes `state`, which
carries totals. Two replace-whole frames, two assignments in `index.ts`, no
browser fold. This is Codex's shape with Gemini's frame name for the hot
path.

### 3. Writes: under the lock (Codex) vs outside it (Gemini)

Codex. The lock is the ordering. Outside it, two concurrent mutations (a
`Group` of two, or a step ending in one session while a scope ends in
another) can rename in the wrong order and leave a file older than memory;
preventing that needs a per-table version or a single writer goroutine,
which is the second code path open question 3 warns against. Gemini's own
numbers (files under 100KB, rename under 0.5ms) say the lock hold is
negligible. `Subscribe` and `Snapshot` also take the lock and would wait at
most that long. The comment in `checkpoint.go` that "no disk work happens
while the reduction lock is held" describes the checkpoint design decision
2 replaces; it goes with the file.

### 4. A finite reader in `runlog` (Codex) vs nothing (Gemini)

Neither. Do not touch `runlog`: `Read` keeps its tailing contract and its
test, and it is the wrong shape for a session log anyway. A finite reader
is a `bufio.Scanner` over one file, one `json.Unmarshal` per line, in
`observation/replay.go`, about fifteen lines, used for `run.jsonl` and every
session log. Codex's "share its line-reading logic" is a refactor of code
whose partial-line handling exists only for tailing. Codex's "explicit
incomplete-log error" for a log without `complete` should also go: a
crashed run renders whatever it logged, with `run.status` as the last
record left it; the absent `complete` is the recording verdict and not a
reason to refuse the page.

### 5. Replay ordering (both)

Both drafts need the ended-turn guard from question 1. With it, Gemini's
order (run log, then session logs) is correct and Codex's grouping is
unnecessary.

## Recommendation

| Question | Answer | Why |
| --- | --- | --- |
| Transcripts | Replay on open, one path, with the ended-turn guard and `CreateTemp` | Session logs are read on every open regardless; the loader and the projection-only path are pure overhead |
| Snapshot and frames | `snapshot`, `state` on lifecycle records, `totals` on step usage, `event` unchanged | `index.ts` loses the fold; the hot path stays small |
| Write cadence | Synchronous, under the lock, once per accepted record, nothing on deltas | Ordering for free; the hold is sub-millisecond |
| Tables in Go | `internal/observation`, as both say | One consumer, one store |
| Log reading | A fifteen-line scanner in `replay.go`; `runlog` untouched | Less code, no contract change |
| Lifecycle decoding | In `observation`, from the record JSON, for live and replay | Import direction forces it |
| Generated files | Delete the three stale files by hand, declare `Usage` in `index.ts`, regenerate | The generator prunes only the link tree |
| Saved run | Temporary copy in tests and in the proof | The directory is tracked |

Taken together: Codex's architecture with Gemini's open path. Codex is the
draft to build from, with the runlog refactor, the interleaving, the
serialize-the-append item, and the generator claim removed, and with
`state` restricted to lifecycle records beside a small `totals` frame.
Gemini's draft should not be built from; its four code-level errors (import
cycle, glob, replay order, out-of-lock writes) are each cheap to fix but
the draft is unaware of all of them.
