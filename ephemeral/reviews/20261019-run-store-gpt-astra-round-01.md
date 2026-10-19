# Independent adversarial review: run-store foundation

Date: 2026-10-19

## Target and authority

Reviewed the run-store foundation from `c5c9b142c7de2270863c0811d7958bd27bdfa88a` to `6c78985455920ec5b80fc54d50d142aed3dcdc1f` in `/Users/tyler/.codex/worktrees/run-store-implementation-plan/gimbal`. The implementation revision is `e28e735265b44d9bdf6c026b88c0741a788d6393`.

Authority: `AGENTS.md`, `docs/definition-of-done.md`, `ephemeral/run-store-request.md`, `ephemeral/run-store-design.md`, `ephemeral/run-store-session.md`, and the latest human priorities reproduced in `ephemeral/run-store-astra-review-request.md`. The latest priorities concerning compactness, useful tests, and ordinary failure handling take precedence over defending the earlier design's complexity. No caller narrowing or requested verdict was present. This review was performed independently, without delegation. Only this review artifact was written.

## Assessment

Material findings remain. The storage seam is real, the important new tests generally protect observable contracts, and I found no need to add a recovery subsystem. The most important problems are ordinary lifecycle ownership mistakes: recording failure is confused with failed resource cleanup, cancellation can race session admission, and failed initialization can abandon the writer. All three call for small ownership fixes and focused regression tests, not more receipts or automatic recovery.

The new tests use the existing test executable for fresh-process checks rather than recompiling fixtures for each case. The directory concurrency test uses channels to hold a sync; its 20 ms negative assertion is a small scheduling heuristic, not a costly setup loop. The focused tests completed quickly below. The changes also mechanically inject execution-identity contexts into unrelated authentication-session tests (`plugins/auth-files/main_test.go`); those additions have no apparent behavioral purpose, but are not a material blocker.

## Findings

### 1. Issue: a recording failure makes successfully stopped resources permanently pending

**Evidence:** `run_recording.go:212-218`, `remote_execution.go:50-61`, `scope.go:289-300`, `run.go:538-548,569-572`, and `internal/runner/runner.go:158-160`.

`closeNative` returns `errors.Join(nativeCloseError, recordError)`. The scope and retained-cleanup caller use that joined error to decide whether the native session still needs cleanup. `remoteCleanup` makes the same mistake before updating `pendingScopes`. Once any recording failure has been latched, `prepareAction` returns it on every subsequent call (`run_recording.go:96-98`), including after a later native close succeeds. Consequently a successful close never removes the session/scope from the pending map, the run-store handle never closes, and a detached runner remains in its `CleanupPending()` loop indefinitely. Repeated operator stops can keep dispatching close/cleanup against an already released resource without ever settling ownership.

This is relevant to an ordinary write failure, including a full disk; it does not require power loss or exotic recovery. It contradicts the design's instruction to “propagate the recording failure and actual stop result separately,” and the instruction that the existing cleanup lifecycle, rather than the store, determines process lifetime.

**Deterministic reproduction:** use the existing fake adapter and fault-store pattern. Complete one session turn, then inject a recording failure before scope teardown (or fail only the close-outcome append). Have `adapter.Close` return nil. After Run returns, the controller reports `CleanupPending()==true`. Invoke its existing cleanup retry with the adapter again returning nil: it still reports pending, and a second Store.Open still returns busy. No native close failure is necessary. This is a source-established causal reproduction; I did not add or execute a new test file under the read-only constraint.

**Small correction:** carry native cessation and recording errors separately through teardown. Use actual native cessation to remove pending ownership and release the handle; still return the recording error and do not claim durable acceptance. Apply the same rule to session and bulk scope cleanup.

### 2. Issue: cancellation during the session intent commit can create a session after its scope has finished cleanup

**Evidence:** `session.go:298-319`, `run_recording.go:109-114`, `scope.go:283-303`, and `internal/agentexecutor/agent.go:492-503`.

The new session-intent commit uses `context.WithoutCancel(ctx)`. After it completes, `Session.turn` calls `CreateSession` without rechecking cancellation or whether the session was closed while the commit was in progress. Meanwhile scope teardown marks the session closed, snapshots its still-empty native ID, and skips native Close. The local executor's CreateSession does not reject an already cancelled context: it creates the session using the supplied identity. The late native ID is then installed into the already-closed session. Scope end is guarded by `sync.Once`, so its later invocation does not revisit that resource.

The design explicitly requires rechecking cancellation/ended state after storage I/O before publishing handles or dispatching. This is an ordinary Cancel action while storage is slow. It can leave a newly allocated session unclosed while cleanup is reported released. This finding concerns session creation; the local command path does have a context check before Submit (`command.go:282-286`).

**Deterministic reproduction:** wrap the run store to pause only the session-create intent Append on a channel, allowing unrelated appends to proceed. Start a first Generate, wait for the paused intent, cancel its scope and let scope teardown mark the session closed while `native` is empty, then release the intent Append. A counting adapter whose CreateSession allocates an ID and whose Close records calls observes one creation after cancellation and zero closes for that ID. The production executor adapter likewise creates its session record with a cancelled context. This interleaving follows directly from the cited code; it was not executed as a new test in this read-only review.

**Small correction:** recheck session/scope cancellation after the commit and coordinate native-session publication with teardown. If admission finishes after closure has won, release that returned native resource through the existing cleanup path rather than publishing it as usable. Do not add automatic session replacement or replay.

### 3. Issue: failed projection-marker initialization leaks the successfully created store writer

**Evidence:** `run.go:217-230`; the directory store starts its handle-owned flush goroutine before returning from Create (`runstore/directory/directory.go:173-174`, `startTimer` at 324 onward).

After Store.Create succeeds, failure of `MkdirAll` closes `records`, but failure of the immediately following `os.WriteFile(recording-format)` does not. `observation.Open` returns nil and removes its reservation; `beginRun` then returns without closing the run handle. No controller receives ownership. The directory store's timer retains the handle, file descriptors and flock, so garbage collection cannot repair this. A rejected run can remain busy until the application exits.

The requested initialization contract says failure is not an accepted run and must not leave live ownership behind. Leaving the directory reserved is intentional; leaving its writer alive is not.

**Deterministic reproduction:** inject a Store wrapper whose Create delegates to the directory store, creates a directory at `<project>/runs/<id>/recording-format`, and returns the valid run handle. Run then fails its marker write with an is-a-directory error, executes no body, and removes the observation registration. Calling the underlying Store.Open for that ID still returns busy because the engine never closed the returned handle. Disk exhaustion at the marker write reaches the same branch. This is a source-established reproduction, not a newly executed test.

**Small correction:** give the newly created handle one cleanup owner until initialization transfers it to the run; close it on every initialization failure, preserving the original error. A single deferred failure cleanup can replace the inconsistent per-branch closes.

## Evidence inspected and checks

- Read the authoritative request, design and acceptance documents, repository instructions, the review skill and organization protocol.
- Inspected the changed storage contract and complete directory implementation; engine recording, scope/session/command/service/interview/artifact integration; backend identities and HTTP/messaging admission; executor/task-daemon propagation; observation registration/format detection; retained/orphan cleanup and host/runner store resolution; relevant surrounding cancellation and teardown code; changed generators and launch examples; the new recording, substitution and ownership tests and related fixture migrations.
- Read the supplied runtime evidence index at `/Users/tyler/.codex/visualizations/2026/10/19/01a1522a-1e4c-7942-bdce-772e9ed181e2/recording-foundation/review-evidence.md` and the prior round-03 report. Those describe real native, remote, UI-cancellation and spool-independent readback proof at the named implementation revision. I did not independently rerun those live scenarios or claim that their happy-path evidence covers the findings above.
- Ran this uncached focused command in the target worktree:

```sh
go test ./runstore/... ./internal/runner ./internal/taskdaemon ./internal/environments . -run 'Test(Ownership|Slots|Interrupted|Incomplete|Buffered|LostCommit|RunRequires|IdentityTree|FailedInitialization|PersistenceFailure|OldDaemon|UnidentifiedBackend|RetainedCleanup|NewFormatObservation|SavedFacts|CommandOutputSurvives|EngineUses|NewCleanup|Sweeper|Orphan|.*Recording|.*Identity|.*Rejected)' -count=1
```

All selected packages passed: directory 0.392 s, runner 0.645 s, taskdaemon 0.477 s, environments 1.287 s, root package 2.724 s. `runstore` has no package tests. This was focused verification, not a new full-suite, race-detector or live-UI qualification.

The existing tests demonstrate duplicate-start exclusion, same-slot identity, conflict rejection, fresh-process ownership/readback, torn-tail/corruption behavior, nonblocking buffered append during sync, lost commit acknowledgement, outcome failure without command replay, retained cleanup after a native close failure, and observation reconstruction. They do not exercise the three failure/interleaving cases above. The supplied evidence also explicitly leaves capacity/segment-roll and some startup race checks unproved; I have not converted those coverage limitations into speculative correctness findings.

## Outcome

material findings remain
