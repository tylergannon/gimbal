# Independent adversarial review: run-store foundation, round 04

Date: 2026-10-19

## Target and authority

Reviewed the complete branch change from `c5c9b142c7de2270863c0811d7958bd27bdfa88a` to HEAD `a9cc64eb03d2895fb5befe1f11506d3f748ec496`, including the surrounding execution, observation and cleanup code. The working tree was clean at the start of this review.

Authority: the current user request; `ephemeral/run-store-astra-review-request.md`; its referenced `run-store-request.md`, `run-store-design.md` and `run-store-session.md`; repository `AGENTS.md`; the public Godoc; `docs/definition-of-done.md`; and the relevant API design, sprint and web-app documentation. The latest human priorities—useful inexpensive tests, compact code and ordinary failure handling—apply to the full branch. The original commit range in the request document is superseded by the instruction to review current HEAD.

Performed independently under `/Users/tyler/.agents/skills/adversarial-review/SKILL.md`, without delegation. No caller instruction narrowed the permissible findings or prescribed a verdict. The read-only boundary supersedes worklog, commit and implementation workflows. This review artifact is the only project file created or changed.

## Assessment

**Material findings remain: one issue.** The round-03 observation-lifetime finding is fixed: observation writers now remain open with retained cleanup, and the regression checks the completed command in both the live registry and a fresh reader while preserving the run's terminal verdict. The current saved runtime evidence corroborates that behavior.

The recording seam is substantive. Engine identity, intent, executor bindings, outcomes, normalized events and owned blobs pass through the run handle; exclusive writer ownership and separate-process readback have meaningful tests. The HTTP/messaging operation changes prevent an older dispatcher from silently accepting unidentified productive work. I found no additional material bypass requiring a new recovery mechanism.

The added tests generally exercise useful contracts and failure modes. Fresh-process tests reuse the existing test executable; they do not repeatedly compile fixture binaries. Admission races and storage failures use channels or direct injection. The late-service test's two shortened 50 ms cleanup boundaries exercise the actual timeout behavior. The blocked-sync test uses a small negative timing assertion, but there is no material test-cost objection here. The broad fixture migration contains unnecessary identity contexts in some unrelated tests/examples, but that does not justify another material finding.

The design is explicit about its limits: this is recording and reopen, not execution replay, automatic crash repair, durable human waits or local orphan-resource reconstruction. No new receipt framework, recovery loop or robustness layer is needed for the finding below.

## Finding

### 1. Issue: a recording failure first raised during retained cleanup can be silently lost

**Primary location:** `run.go:532–579`, particularly the final return at line 579.

**Supporting evidence:** `remote_execution.go:126–144`; `run_recording.go:165–180,197–220`; `runstore/directory/directory.go:708–738`; `run_cleanup_recording_test.go:40–50,77–98`.

`retryCleanup` checks the latched recording error indirectly when it prepares its own intent. It does not include `r.recordingError()` when returning. Cleanup work performed after that initial check can latch a new recording failure without returning it through the local `recordErr` variable.

The retained-service path gives a concrete interleaving:

1. A remote service's final result remains delayed past the initial cleanup budget. `Run` returns with the service retained and no recording error, as in the existing late-service regression.
2. An explicit cleanup retry successfully records its own intent, then waits in `service.stop()` for the final result.
3. The result arrives successfully. The service goroutine sets `service.terminalErr` to nil before recording its output. Its `commandOutcome` then fails to ingest a completed-output blob. That error is latched on the run, but the goroutine discards the returned error and closes `service.done` after attempting the lifecycle projection.
4. Blob-ingestion failure need not poison the record writer: DirectoryStore's `PutBlob` returns errors from temporary-file creation, copying, syncing or renaming without assigning `r.poison`. Consequently subsequent small record appends and `Close` can succeed. A transient failure creating the blob is sufficient; this does not require corrupted records or a power-loss scenario.
5. `retryCleanup` sees the service released, removes it from pending ownership, commits its own successful outcome and root release, closes the writers, and returns nil. The newly latched recording error is absent from `result`, `recordErr` and `observationError`.

The native resource really is released, so releasing ownership is correct. The incorrect behavior is reporting an entirely successful cleanup while the service's authoritative output/outcome was not saved. `Run` has already returned, its earlier Complete event predates the failure, and the controller is now eligible for removal; there is no later normal caller that will receive the latched error. The existing session/scope close helpers have the same reporting dependency: they intentionally return native cessation separately and latch recording errors, which the retry owner must collect after invoking them.

**Missed requirement:** `ephemeral/run-store-design.md:103` explicitly requires: “Later store/close errors are reported to the cleanup caller rather than swallowed.” Line 109 requires propagating recording failure separately from actual stop success. This finding concerns that error-reporting contract, not a demand to retry storage or retain a resource that has already stopped.

**Reproduction specification and verification limit:** this finding is established by the control flow above; I did not modify a test or inject a new failure during this read-only review. A deterministic regression can reuse `TestLateServiceResultRetainsWriterUntilExplicitCleanup`: have a Store wrapper signal after the retry-cleanup intent has committed, then release the held service result and fail its first `PutBlob` once while allowing subsequent Appends/Close. Assert that the cleanup call returns that injected error, resources are still released, the writer can reopen, the original run verdict is unchanged, and no second native submission occurs. On the current code the cleanup call returns nil. The existing channel fixture supplies the ordering; no additional compilation or long sleep is needed.

The passing `TestRecordingFailureDoesNotRetainStoppedResources` does not cover this interleaving: it enables persistent failure before the workflow body returns. The error is therefore already latched when retryCleanup prepares its outer intent, and/or also fails the outer outcome. That explains why the present tests pass despite this missing final error collection.

**Correction:** collect the run's latched recording error after the retry's owned cleanup work, alongside its returned storage/close and observation errors. Keep native cessation authoritative for pending ownership and root release. Extend an existing regression to cover a failure first arising inside the retry; do not add automatic retries or another ownership subsystem.

## Evidence inspected

Inspected the storage contract and directory implementation; node/action/event/blob recording; scope, group, session, command, service, interview, artifact and supervisor integration; execution identity validation and native executor/task-daemon propagation; HTTP and messaging envelopes and plugin registrations; standalone conversation/environment producers; host/runner admission and store selection; observation initialization, projections and historical readers; retained and orphan cleanup; generated transport/analyzer changes; examples; added tests and the fixture migration. Prior review conclusions were treated as context, not authority.

Current proof index:
`/Users/tyler/.codex/visualizations/2026/10/19/01a1522a-1e4c-7942-bdce-772e9ed181e2/recording-foundation/astra-fixes-evidence.md`.

Read the supplied public-Store inspection utility and current check-log endings. The saved Linux production binary reports the reviewed HEAD and `vcs.modified=false`. Independently viewed `late-service-reopened.png`, `native-transcript-astra-settled.png`, and `remote-reopened-astra-settled.png`. They show the late service's `stopped` output, both native answers with remembered Darwin context, and the remote test command/output. The supplied native run used Codex **luna**.

A read-only decoder independently checked the four current proof stores' frame lengths/checksums, contiguous sequences, node/reference ordering, every referenced blob's size and SHA256, released cleanup projections, and absence of run-local executor spools:

| Run | Records | Nodes | Blobs | Blob bytes | Projection |
| --- | ---: | ---: | ---: | ---: | --- |
| `01M5AQHGZGE9E41A57W6RPS55Y` | 110 | 11 | 3 | 13 | completed/released |
| `01M5AQK6VGKXMFKZ6Y43XAQS7R` | 19 | 3 | 2 | 21 | completed/released |
| `01M5AQKS7ZVKQAACJE2Z7DKBZ1` | 17 | 2 | 1 | 0 | cancelled/released |
| `01M5AQH274ZBHMJEGCJ8W98W3M` | 24 | 4 | 2 | 7 | failed/released |

The late-service command table has a nonzero ended time, exit 0, interrupted true, stdout `stopped`, and both output references. The before/after summaries are byte-identical with SHA256 `56f8e726d585e468c3e6b68a6530cc83a82360775733295468d412c2b0057f0b`.

I did not launch another native turn, remote container, application restart or UI cancellation. I did not open the supplied stores for writing. Those live operations remain supplied evidence, corroborated by the saved artifacts rather than repeated here.

## Checks executed

```sh
go test ./runstore/... ./internal/runner ./internal/taskdaemon ./internal/environments . -run 'Test(Ownership|Slots|Interrupted|Incomplete|Buffered|LostCommit|RunRequires|IdentityTree|FailedInitialization|PersistenceFailure|OldDaemon|UnidentifiedBackend|RetainedCleanup|NewFormatObservation|SavedFacts|CommandOutputSurvives|EngineUses|NewCleanup|Sweeper|Orphan|.*Recording|.*Identity|.*Rejected|CancellationDuringSessionAdmission|ProjectionMarkerFailure|LateService)' -count=1
```

Passed: directory 0.446 s, runner 1.017 s, taskdaemon 0.569 s, environments 1.384 s, root 4.573 s. The public runstore package has no tests itself.

```sh
go test -race ./runstore/directory . -run 'Test(BufferedEventsDoNotWaitForSyncAndTerminalDoes|SlotsAndAtomicConflicts|RecordingFailureDoesNotRetainStoppedResources|CancellationDuringSessionAdmissionReleasesEveryNativeSession|ProjectionMarkerFailureReleasesStoreWriter|RetainedCleanupOwnsStoreAndAppendsAfterImmutableOutcome|LateServiceResultRetainsWriterUntilExplicitCleanup|PersistenceFailurePreventsDispatchAndDoesNotRepeatCompletedWork)$' -count=1
```

Passed: directory 1.301 s, root 4.084 s. `git diff --check c5c9b142 HEAD` also passed.

These are focused checks, not a new full-suite/build/frontend qualification. The supplied proof distinguishes earlier full-suite qualification from the current affected-suite rerun. Its acknowledged remaining coverage edges—buffer capacity and segment rolling, some startup lock windows, mixed settled-run restart, context-only override rejection and completion inversion—remain unproven here. They are not additional findings absent a concrete failure.

## Outcome

material findings remain
