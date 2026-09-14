decision: The #157 status comment's "they agree on every decision" held for the absorbed issues but not for four design points (message-row $0 cell, test fixture, page structure, expand state); the coverall (#173) states each choice inline rather than reopening them, since the comment's named merge plus Tyler's "never $0" rule settle the ones that matter.
decision: #152 stays open: no harness reports per-call cost (`claude/events.go` writes `cost: 0` per step and says so; Codex and Antigravity state none). The price proxy in #173 is not a stated per-message cost.
decision: Price data goes in its own issue (#172): it is a build and release process (download, filter, go:embed, daily GHA, patch tag) with a distinct proof, and #173 consumes it and degrades to "no price" without it.
doc_bug: Both coverall drafts carry checkpoint residue (`loadCheckpoint` defaults, "checkpoint alone" proof step, legacy labelling) that #169 removes; the coverall replaces those with #169's own proof step and the saved issue-149 run rendered from its logs.
friction: The Gemini share link renders only in a browser (data fetched after load); curl under four user agents returned no conversation text, WebFetch failed on a header overflow, and the Chrome extension was not connected. Recorded the gap in `ephemeral/research/quota-telemetry/README.md` instead of blocking.
friction: zsh treats a bare `=====` argument as a `=command` expansion and `--include=*.go` as a glob; quote both in Bash calls.
fact: There is no release process on the repo: no tags on origin, no goreleaser, only `docs.yml`; #172 has to define what a patch version is (a `vX.Y.Z` tag on main).
fact: models.dev `api.json` has no reasoning price; #173 prices reasoning as output and says so on the page. Antigravity's `gemini-3.8-flash-low` needs the effort suffix stripped to hit `gemini-3.8-flash`.
decision: Tyler rejected "the log is the only source" and asked for a designed store: file-based (six JSON array files of the seven fact tables under `runs/<id>/`), roll-ups in Go, one path live and finished. Planned with the DF sprint-plan skill (unnumbered, no ledger, no token cache); intent at `docs/sprints/drafts/RUN-STORE-INTENT.md`, three drafts and three critiques beside it, final at `docs/sprints/RUN-STORE.md`.
friction: The Claude CLI's OAuth session is expired and cannot be refreshed here; the Claude planning lanes ran as Agent-tool subagents (model fable) instead of `claude -p`. Say so in the plan.
fact: Verified for the merge: `session.go` writes every step event through `run.sessionEvent` before it emits `turn_ended`, so a turn's native history always precedes its report in log order; session ids are scope-prefixed (`agy.1/agy.1`) so `sessions/<id>.jsonl` nests one directory per scope level and `newEventWriter` MkdirAlls it; all three adapters put the resolved model in `session.step.started` as `data.model.id`; `Registry.finish` deletes the store and `Snapshot` falls to `loadCheckpoint`; `internal/skgo/config.go` carries the `go generate` line that rewrites the skgo bindings; `web/src/lib/observation/index.ts` folds `lifecycle` and `session.usage.updated` in the browser today.
fact: The drafts disagree on three things only: open a finished run by replaying the log (Claude, Gemini) or by loading the files and hydrating transcripts from the session logs (Codex); frames `row`+`totals` (Claude), `state` (Codex), or keep `lifecycle` and add `totals` (Gemini); write under the lock (Codex), outside it (Gemini), or through an in-flight-absorbing flusher (Claude). Everything else agrees with the intent.
decision: Run store plan filed at `docs/sprints/RUN-STORE.md` (merge notes beside the drafts). One interview answer taken on Tyler's behalf: a finished run is opened by replaying its logs through the store and the six files are the store's output; Gimble does not read its own table files this sprint. Flagged as open question 1 in the plan.
fact: Pre-existing breakage on main after #143: `ephemeral/review/{codex,claude}-loop-api/probes/main.go` fake adapters lack `Close`, so `go build ./...` and `go test ./...` fail there. Out of this sprint's scope; the gates run over `go list ./... | grep -v /ephemeral/`. Tyler's call whether to delete the probes.
friction: The proof run's Claude leg needs the Claude CLI logged in, and it is not (OAuth session expired). Phases 1 and 2 proceed; the proof waits for a login or runs on Codex and Antigravity only with the gap stated.
decision: Merged origin/main (ad6f2f6) into the sprint branch before the build so the builder works on current code.

## 2026-09-13 — decision: opening a finished run loads the tables

- decision: Tyler rejected the filed plan's "replay the whole log on open".
  The six files are the data model and Gimble reads them; the session logs
  are read for transcripts only; the lifecycle log is replayed, and the six
  files written, only when one of the six is missing (a run recorded before
  this sprint, or a damaged directory). Replay-on-open re-derives old runs'
  numbers through whatever the fold is today and makes the tables
  write-only for Gimble. The three planning lanes conflated reading the
  session logs (needed on every open) with re-deriving facts (not).
- Amended `docs/sprints/RUN-STORE.md` (L1, use cases, § Opening a finished
  run, DoD open-path tests, risks, open question 1) and the merge notes.
- Builder was held before it wired the registry to replay, then redirected:
  load path first, rebuild path when `missingTable(dir) != ""`, session logs
  through `Event` on both paths, two registry tests (altered number in
  `turn_usage.json` is served; deleted file triggers rebuild).

## 2026-09-13 — Phase 3 proof run

- Builder finished Phases 1 and 2 (`c66bea5`, `bbffd66`); I reran every
  gate myself: build, vet, all Go packages, web tests, `just build`. No file
  outside the plan's footprint.
- Proof at `ephemeral/attest/run-store/` (`result.md`). Every number on the
  page equals the jq sum over `turns.json`+`turn_usage.json` and the
  `turn_ended` records, live and after the run; the binary reopens the run
  from its tables with mtimes unchanged; the issue-149 logs rebuild to the
  registry test's literals.
- gap: the Claude leg ran on Codex (`-second codex`) because the Claude CLI
  is logged out and I may not enter credentials. Haiku's cache-write and
  stated-cost columns are covered by the issue-149 rebuild instead.
- Launched the sol adversarial review (`gpt-5.6-sol`, high) at ~18:14; it
  writes `ephemeral/reviews/202609131715-run-store-round-01.md`.
- Round 1 review (`ephemeral/reviews/202609131715-run-store-round-01.md`):
  two findings, both accepted. (1) On the load path `Event` still replaced
  `model_calls` rows from the session log; fixed with `fromTables` so the
  fold does no accounting over loaded facts, test extended. (2) `./...`
  gates failed on the two pre-existing probe fakes; their `Close` and
  `RunTurn` signatures brought current, nothing else. Commit `f27804a`,
  repository-wide gates green with no exclusions. Round 2 launched on the
  same reviewer session.
- Round 2 (`202609131830-run-store-round-02.md`): two findings, both DoD-bound
  and accepted. (1) `turnSeenLocked` still made turn rows from the session
  log on a loaded store; fixed in `0b40d76`, regression extended. (2) The
  proof's "rose without a reload" was two curl fetches; reran the proof with
  headless Chromium on one open page (`7793796`, result.md § Run 2). The
  Chrome extension was offline; playwright from another checkout was used.
- Round 3 (`202609131845-run-store-round-03.md`): no findings. Merged
  origin/main (`cfe4baf`), gates green, PR #181 opened and squash-merged.
- Per Tyler: reviewer turns are now capped (9 min this round) and the
  builder was not resumed after its last commit; nothing runs open-ended.
- Left for later: #169/#173 rewrites per the store decision; Claude leg of
  the proof needs a CLI login; Gemini share doc capture.
