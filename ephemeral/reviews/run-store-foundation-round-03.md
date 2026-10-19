# Adversarial review: run store foundation, round 03

Date: 2026-10-18. Reviewer: Claude (adversarial-review skill, performed directly, read-only apart from this file).

## Review target

- Worktree `/Users/tyler/.codex/worktrees/run-store-implementation-plan/gimbal`, branch `codex/run-store-foundation` at e28e7352, clean tree, in sync with `origin/codex/run-store-foundation`. Eight commits over `main` (c5c9b142); the one commit since round 02 is e28e7352 `fix: pass complete release qualification lint checks`. Branch total: 155 files changed, +5322/-743.
- Authoritative sources, unchanged from rounds 01 and 02: `ephemeral/run-store-request.md`, `ephemeral/run-store-design.md`, `ephemeral/run-store-session.md`, `AGENTS.md`, `docs/definition-of-done.md`, `justfile` (`vet`, `lint-go`, `test`, `check-web`, `fmt-check`), `.github/workflows/release.yml`, plus the round-01 and round-02 reviews.
- Proof under review: `/Users/tyler/.codex/visualizations/2026/10/19/01a1522a-1e4c-7942-bdce-772e9ed181e2/recording-foundation/review-evidence.md` and the artifacts beside it.
- Caller constraints (read-only, one output file) are valid operating constraints; no narrowing of scope or verdict was present.

## Evidence inspected

- Read the complete production and test diff of e28e7352. Production changes are exactly four lines: three deferred best-effort `Close`/`Remove` calls made explicit (`internal/runner/runner.go:266`, `internal/runner/task_cleanup.go:136,163`, `runstore/directory/directory.go:721`) and the De Morgan rewrite of the parent-kind predicate at `directory.go:438`, which I checked is logically identical (`!(A && B)` to `!A || !B`). `backend.Connection.Close` (`backend/backend.go:174`) is safe on a zero value, so the deferred close at `task_cleanup.go:163` before the error check is fine. No workflow API, wire shape, store format or application behavior changed.
- Re-read all three authoritative design/session documents in full against the test inventory in `runstore/directory/directory_test.go`, `run_recording_test.go`, `run_store_substitution_test.go`, `internal/runner/recording_test.go` and `internal/taskdaemon/recording_test.go`.
- Ran at e28e7352 (logs under `/private/tmp/claude-501/-Users-tyler--codex-worktrees-run-store-implementation-plan-gimbal/76388f25-cca3-460a-8146-390d8bf5ed00/scratchpad/`):
  - `golangci-lint run --max-same-issues=0 --max-issues-per-linter=0 ./...` with the locally installed 2.15.0: 0 issues, exit 0 (`lint.log`).
  - `golangci-lint run ./...` with v2.14.0, the version `.github/workflows/release.yml:165` pins: 0 issues, exit 0 (`lint214.log`).
  - `go vet ./...`: clean (`vet.log`).
  - `go test -count=1 ./...`: 77 packages ok, exit 0 (`fulltests.log`).
  - `just fmt-check`: all 168 files correctly formatted.
  - Frontend with the pinned runner `web/node_modules/.bin/vp`: `run check` 0 errors / 0 warnings over 5485 files (`webcheck.log`); `test --run` 44 files, 201 tests passed (`webtest.log`). Round 02 had not run these; the evidence's frontend claims are now confirmed.
- Independent live-run readback: I wrote a disposable program in the scratchpad (not in the tree) that uses only `runstore/directory.Store.Open`, `Read`, `Header`, and `OpenBlob` and pointed it at the proof's project runs directory. For the three newest runs it found the same record and kind counts as the proof's `after-qualified.jsonl` (observe-host 01M5986T6VGPFSXA2DYX3AD08C: 113 records, 11 nodes, 47 agent events, 7 outcomes; verify-discount 01M5987CQZJBD9FZH56M2CG1JY: 19 records; cancelled observe-host 01M5987K07HCAJR172AR2ATC04: 17 records), every fact referenced a node created earlier in the stream, and no `executor-spool` directory remains under any run. The remote command's intent records its HTTP route and `command_<action ULID>` operation, its outcome references stdout as a 21-byte content-addressed blob, and `OpenBlob` refuses a reference without the full digest (`runstore: corrupt data`), then verifies size and SHA-256 on read (`directory.go:762-775`). `run.json` for all three reports `cleanup_status: released`. No proof container remains in `docker ps -a`.
- Checked the proof binaries: `go version -m runtime/gimbal-linux` reports `vcs.revision=e28e7352…`, `vcs.modified=false`.

## Round-02 findings, status

1. `just lint-go` fails: resolved. Zero issues with both the local 2.15.0 and the release-pinned 2.14.0, including the unbounded-reporting flags. The three pre-existing `main` issues and the additional ones unbounded reporting exposed are also cleared.
2. Scope-ended node creation latched as a recording failure: unchanged (`run_recording.go:51,59`). Still a nitpick, see below.
3. Acceptance items without a repeatable check: unchanged, see finding 2 below, which adds one item I had missed.
4. Session creation bound as a `turn_` operation: unchanged (`run_recording.go:83-95`). Still a nitpick.

## Findings

### 1. Nitpick: the evidence text lags the proof artifacts

- `review-evidence.md` states "Live proof revision: 00e901f3" and argues equivalence for e28e7352 by describing the lint diff. But the artifact directory holds a complete later cycle at e28e7352: `before-qualified.jsonl`/`after-qualified.jsonl` (byte-identical before and after spool removal), `native-transcript-qualified.png`, `cancelled-run-qualified.png`, the Linux task binary rebuilt from e28e7352 with `vcs.modified=false`, and three runs created at 04:55Z (IDs above). My own readback agrees with that summary.
- Impact: none on correctness; the stronger proof exists and is simply not described. The evidence should name the e28e7352 cycle and its run IDs so the report and the artifacts agree.

### 2. Nitpick: acceptance items still without a repeatable check

Carried from round 02 with one addition. None contradicts the code I read; the live proof covers the happy paths but not these edges:

- Buffered backpressure at `pendingLimit` (4 MiB, `directory.go:28,568`): no test fills the pending buffer and shows a context-cancelable wait rather than a drop or failure. The session asks to "stream a long event sequence and verify bounded buffering and segment count"; `TestBufferedEventsDoNotWaitForSyncAndTerminalDoes` proves sync independence, not the bound.
- Segment roll at 16 MiB and the "truncate only the last segment" rule: no test crosses a segment boundary; tests reference only `00000000.segment`.
- Pausing `Store.Create` between writer-lock creation and flock, and runner startup between `runner.lock` creation and flock, racing a sweeper and `Snapshot`.
- Host restart with zero `Store.Open` over a mix of successful and failed-body released runs (one settled run is covered).
- `Project.Run` rejecting a context-only store override (`internal/host/host.go:302`).
- Parallel completion inversion compared against readback.

### 3. Nitpick: the lint commit edits tests in unrelated packages and adds assertions

- To make the full gate green, e28e7352 touches `cmd/gimbal/release_plugin_archive_test.go`, `internal/pi/{agent,session}`, `internal/workflows/researcheval/wait_test.go`, `internal/environments/connect_watch_test.go`, `backend/messagefiles/store_test.go`, `plugins/messaging-{filesystem,river}` and `scope_lifecycle_test.go`. Most changes only read a previously discarded channel value into a message, but several add new assertions: worker shutdown must return nil or `context.Canceled`, `researcheval` now fails if a timed-out child returns success, `pi/session` now fails if a prompt after abort returns an error.
- They all pass, and clearing the release gate was the round-02 ask, so this is within reason. It is worth knowing that these packages now assert slightly more than they did on `main`; if one of them flakes later, the cause is this commit, not the run store.

## What I did not do

- I did not rerun the live application (the Codex `luna` turns, the Docker remote executor, the UI cancel). I verified the resulting stores independently from a fresh process, checked the binaries' recorded revision, and confirmed the spool and container removal the evidence claims.

## Outcome

no material findings remain. The round-02 lint gate is closed under both lint versions, every repository check I could run passes at e28e7352, and the live stores at that revision read back correctly through the public contract. Findings 1 to 3 are nitpicks: a stale sentence in the evidence, unproved-but-consistent edge scenarios, and scope creep inside the lint commit.
