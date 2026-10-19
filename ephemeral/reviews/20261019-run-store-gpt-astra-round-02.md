# Independent adversarial review: run-store foundation, round 02

Date: 2026-10-19

## Target and authority

Reviewed the complete branch change from `c5c9b142c7de2270863c0811d7958bd27bdfa88a` to HEAD `08e46c4bc8f7ee477bda8792cc97cf65271b0c9f` in `/Users/tyler/.codex/worktrees/run-store-implementation-plan/gimbal`, including surrounding execution, observation, service and controller lifetime code. The working tree was clean at review start.

Authority: the human request in `ephemeral/run-store-astra-review-request.md`, its referenced `ephemeral/run-store-request.md`, `ephemeral/run-store-design.md` and `ephemeral/run-store-session.md`, repository `AGENTS.md`, current public Godoc, `docs/definition-of-done.md`, and the API/design and web-app documentation named by the repository. The latest priorities concerning useful inexpensive tests, compact implementation and ordinary failure handling apply to the entire change. The review request's original revision range is superseded by its follow-up authorization and the current user's explicit instruction to review HEAD.

Performed the review independently, using `/Users/tyler/.agents/skills/adversarial-review/SKILL.md`, without delegation. There was no caller instruction narrowing permissible findings or prescribing a verdict. The read-only constraint supersedes ordinary worklog, commit and implementation workflows. Only this new review artifact was written; implementation and tests were not edited.

## Assessment

**Material findings remain: two issues.** The prior three findings have corresponding fixes and focused regression tests. Their selected tests and race checks pass. The remaining problems are narrow error propagation and ownership mistakes, not reasons to add a recovery system or another receipt layer.

The new tests mostly exercise meaningful contracts: exclusive ownership, stable identity, conflicting facts, output independence, persistence ordering, backend admission, cleanup and cancellation. Fresh-process tests reuse the current test executable rather than compiling a new fixture per case. The new round-01 regression tests coordinate with channels and add no sleep or subprocess setup. The storage sync test has one short timed negative assertion; this is not a material resource-cost problem. The broad test diff also contains mechanical RunID/execution-identity migrations and additional shutdown-error assertions. I found no material test-cost defect warranting a separate finding.

The directory log and engine integration implement a real replaceable recording seam, and the supplied ordinary-run proof is credible within its stated limits. The findings below each have a small correction in an existing path.

## Findings

### 1. Issue: RunCommand reports success when its outcome could not be recorded

**Evidence:** `command.go:114-120`, `command.go:261-265`, `run_recording.go:164-192`, and `run_recording_test.go:204-225`.

`runCommand` now persists command output and the terminal outcome, returning any failure in its fourth return value. The public `RunCommand` wrapper still discards that value:

```go
_, ended, err, _ := runCommand(ctx, s, name, workdir, command, args)
return ended.ExitCode, ended.Stdout, ended.Stderr, err
```

If the command exits successfully but `PutBlob` or the outcome append fails, the workflow receives exit code 0 and a nil error. The run eventually returns its latched recording error, but that happens after the body has already been told that this action succeeded. Ordinary Go code following the call can consume or publish the result before learning that its required durable outcome is missing. A later Gimbal dispatch being blocked does not repair the earlier false success.

**Missed requirement:** `ephemeral/run-store-design.md`, Recommendation and Dispatch integration: commit results before reporting success; specifically, “If outcome commit fails after native completion, return a recording error and retain the executor result.” The acceptance also requires reporting recording failure after a real completed command without another submission.

**Concrete reproduction:** use the existing `faultStore` with `failKind: "outcome"` and the successful `printf x >> submissions` command already used in `TestPersistenceFailurePreventsDispatchAndDoesNotRepeatCompletedWork`. Inspect the error returned by `RunCommand` *inside* the body: it is nil, while the outer `Run` error is nonnil. The marker contains exactly one `x`; no replay is needed to expose the bug. That existing test passed during this review because it only checks the outer Run error and submission count. The per-call observation is established by the return-value flow above; I did not modify the test to add that assertion under the read-only constraint.

**Correction:** return the joined native/capture and recording errors from `RunCommand`, retaining the known exit code and output. Extend the existing outcome-failure case to assert the public call's error inside the body. No new retry behavior is needed.

### 2. Issue: observing a late service completion can abandon the retained store writer

**Evidence:** `scope.go:297-326`, `service.go:251-260,325-337`, `remote_execution.go:125-145,203-222`, `run.go:330-347,504-530,574-576`, and `internal/live/live.go:90-107`.

A service whose shutdown/result collection exceeds the cleanup budget is placed in `pendingServices`. Run correctly retains its store writer while that service is pending. The service's background result goroutine can finish later and close `service.done`.

However, `CleanupPending()` itself then deletes the released service from `pendingServices`. If it was the last retained resource, this returns false without recording the released root cleanup state or closing `r.records`. The live registry subsequently removes the controller. Even a caller still holding the controller cannot finish release through `retryCleanup`: its empty-map early return says there is no pending cleanup and bypasses the writer close.

For an in-process hosted run, the directory store's timer keeps the handle and flock alive, so the run stays busy until the application exits despite having no remaining native work. Its cleanup projection also remains pending. A detached runner may exit and release the OS lock, but that incidental process exit does not provide the missing released cleanup fact. This is a normal late-result/cleanup-budget interleaving; no crash or power failure is required.

**Missed requirement:** `ephemeral/run-store-design.md`, Terminal cleanup is part of recording: the retained controller owns the handle through pending cleanup, the last successful cleanup flushes/closes it, and completion updates the committed root cleanup state and projection. A status query must not discard the only owner before those operations finish.

**Concrete causal reproduction:** run one remote service through an in-process Run with a directory store and retained live controller. Let the task cleanup call confirm cessation, but hold the service's final result or output read behind a channel. Keep it blocked through both bounded service waits in root teardown so Run returns with only `pendingServices` populated and the store retained. Release the final result and wait for the service result goroutine to close `done`. Query `CleanupPending()` once (or call the live registry's `InProgress`). It deletes the final entry and reports false; Store.Open still returns busy, and a subsequent explicit cleanup retry exits at `run.go:529-530` without closing the writer. This follows directly from the cited ownership transitions; no new scenario was executed or test file added during this read-only review.

**Correction:** retain the service bookkeeping until the existing cleanup owner has recorded release and closed the writer, or make that ownership transition explicitly finish those operations before removing the controller. The smallest solution can keep the entry for explicit cleanup retry; this does not require automatic resource recovery or a new background retry policy. A channel-controlled regression should cover a service that finishes after Run returns and a status query before retry.

## Prior findings

- **Recording failure confused with native cleanup failure:** fixed for the reviewed session and remote-scope paths. `closeNative` and `remoteCleanup` base pending ownership on actual stop results while latching recording failures. The new success/retry and intent/outcome cases pass.
- **Cancellation during session admission:** fixed for the reviewed creation and fork paths. Post-intent admission checks prevent dispatch after cancellation; `adoptNative` coordinates publication with teardown and closes a late handle. The intent/native interleaving tests pass under the race detector.
- **Failed projection-marker initialization leaks the writer:** fixed by the initialization ownership defer. The marker-failure test verifies no body execution, removal of the live reservation and successful exclusive reopen; it passes under the race detector.

These conclusions concern the specific reported cases. They do not establish that every resource lifetime path is correct; finding 2 is a separate service path.

## Evidence inspected and verification

Inspected the storage contract and directory implementation; engine node/action/event/blob recording; scope, session, command, service, interview and supervisor integration; backend identity validation and HTTP/messaging operation changes; executor/task-daemon identity propagation; standalone producers; host/runner admission and store resolution; observation initialization and legacy/new-format separation; retained and orphan cleanup; generated transport changes and examples. Audited the full changed-file set, including the mechanical fixture migrations and nonmechanical test edits, and read the current proof index and relevant previous reviews.

Current supplied proof:
`/Users/tyler/.codex/visualizations/2026/10/19/01a1522a-1e4c-7942-bdce-772e9ed181e2/recording-foundation/astra-fixes-evidence.md`.

I independently inspected the saved native-transcript and remote-reopened screenshots. They show both native answers and the remote command/output. The supplied native run used Codex **luna**. I did not launch a new native turn, remote container or UI cancellation scenario.

Read-only inspection of the three current on-disk run stores verified frame checksums, contiguous sequences, node/parent/reference ordering, every referenced blob's size and SHA256, the reported counts, `cleanup_status: released`, and absence of each run-local command executor spool:

| Run | Records | Nodes | Blobs |
| --- | ---: | ---: | ---: |
| `01M5AP4JNRPK6RB61Y81RXWV9P` | 109 | 11 | 3 |
| `01M5AP61EE8TYWK1SN7SYG87V1` | 19 | 3 | 2 |
| `01M5AP6S08VQKZYZSNPTCQSN88` | 17 | 2 | 1 |

The before/after Astra-fix summary files are byte-identical (SHA256 `904257a61b31434e4efbfeb150dcfb01290fea868aead7c51e1841a37f7d0460`). The saved Linux binary reports revision `08e46c4bc8f7ee477bda8792cc97cf65271b0c9f` and `vcs.modified=false`. I read the disposable Store inspection utility's source and the supplied check-log endings. The initial full-suite failure and subsequent successful root/cmd/web rerun are disclosed accurately in the proof index. I did not treat those log excerpts as a new full-suite execution.

Ran this uncached focused check at HEAD:

```sh
go test ./runstore/... ./internal/runner ./internal/taskdaemon ./internal/environments . -run 'Test(Ownership|Slots|Interrupted|Incomplete|Buffered|LostCommit|RunRequires|IdentityTree|FailedInitialization|PersistenceFailure|OldDaemon|UnidentifiedBackend|RetainedCleanup|NewFormatObservation|SavedFacts|CommandOutputSurvives|EngineUses|NewCleanup|Sweeper|Orphan|.*Recording|.*Identity|.*Rejected|CancellationDuringSessionAdmission|ProjectionMarkerFailure)' -count=1
```

All selected packages passed: directory 0.406s, runner 0.800s, taskdaemon 0.531s, environments 1.163s, root 4.081s. `runstore` has no package tests.

Also ran:

```sh
go test -race ./runstore/directory . -run 'Test(BufferedEventsDoNotWaitForSyncAndTerminalDoes|SlotsAndAtomicConflicts|RecordingFailureDoesNotRetainStoppedResources|CancellationDuringSessionAdmissionReleasesEveryNativeSession|ProjectionMarkerFailureReleasesStoreWriter|RetainedCleanupOwnsStoreAndAppendsAfterImmutableOutcome)$' -count=1
```

Both passed: directory 1.276s, root 3.313s. These are focused checks, not a new full-suite/build/frontend qualification, and neither includes the missing assertion/interleaving described in the findings.

The current proof still explicitly leaves buffer-capacity/segment-roll boundaries, some startup lock windows, mixed settled-run restart, context-only override rejection and parallel completion inversion without separate demonstrations. I have not promoted those acknowledged coverage limits into additional correctness findings or expanded this slice into replay, SQL storage or orphan resource reconstruction.

## Outcome

material findings remain
