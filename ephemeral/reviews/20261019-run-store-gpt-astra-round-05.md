# Independent adversarial review: run-store foundation, round 05

Date: 2026-10-19

## Target and authority

Reviewed the complete branch change from `c5c9b142c7de2270863c0811d7958bd27bdfa88a` to HEAD `2e8f4ab5d13490c941efa56c5bbae1bf87b0f918`, including surrounding execution, observation and cleanup code. The worktree was clean at review start.

Authority: the user request, `AGENTS.md`, `skills/gimbal/SKILL.md`, the applicable build/review guidance, current public API documentation, `docs/definition-of-done.md`, the API design/sprint records, `docs/web-app.md`, and `ephemeral/run-store-request.md`, `ephemeral/run-store-design.md`, `ephemeral/run-store-session.md`, and `ephemeral/run-store-astra-review-request.md`. The latter's follow-up authorization makes current HEAD the target; its original revision range is historical. The latest human priorities concerning useful tests, compactness and ordinary failure handling apply throughout.

Performed this review independently without delegation. There was no caller instruction to narrow the defects considered or prescribe a verdict. Only this new review artifact was written to the repository; implementation and existing evidence were left unchanged.

## Assessment

**Material findings remain: one issue.** The last round's missing final recording-error collection is fixed: `retryCleanup` now includes `r.recordingError()` after cleanup work, and the extended regression checks a one-time failure first raised during the retry. The earlier fixes for command error propagation, late-service ownership and observation, session-admission cancellation and initialization ownership remain present, and their selected regressions pass, including under race detection.

The remaining issue is in the separate orphan stop path. It does not require a recovery framework or stronger durability guarantees. It requires applying the already specified safety-stop exception after the existing ownership checks have succeeded.

The added tests generally exercise meaningful contracts. Fresh-process checks reuse the test executable rather than recompiling a fixture per case. Admission and cleanup tests use channels or direct fault injection. The late-service test deliberately exercises a shortened cleanup deadline; its elapsed wait is the behavior under test. The blocked-sync test's 20 ms negative assertion is a scheduling heuristic, but not a material resource-cost problem. The small in-memory store is a substitution fixture, not a second production storage implementation. I found no material need for additional layers, receipts or automatic retries. Some mechanical identity-context additions to unrelated authentication-session tests are unnecessary, but do not affect the material outcome.

## Finding

### 1. Issue: recording failure prevents an explicit orphan stop after ownership has already been established

**Primary location:** `internal/runner/task_cleanup.go:184–207`, especially the early returns at lines 190–191 and 203–207.

**Supporting locations:** `internal/runner/runner.go:251–254`; `internal/runner/task_cleanup.go:118–161,218`; `run_recording.go:197–220`; `remote_execution.go:34–57`.

The ordinary orphan control entry routes a scope/root stop to `cleanupStopped`. That function first acquires the exclusive run-store writer, then the existing runner-liveness lock, reads the saved run and validates the requested scope, and resolves the task route. At that point it has established the ownership conditions required to stop the known remote target. Nevertheless, failure to allocate the cleanup action or commit its intent returns immediately, before the sole `effects.Cleanup` call at line 218.

A full disk or write failure on the workflow host can therefore prevent an operator from stopping already-running work on a healthy remote task host. Repeating the stop while the recording failure persists still dispatches nothing. This is a regression in an existing control path, rather than a demand to reconstruct unknown native resources. The in-process equivalents already handle the distinction: `closeNative` and `remoteCleanup` attempt the known stop even when preparation fails, while retaining the recording error separately.

**Missed requirement:** `ephemeral/run-store-design.md:109` says: “Existing safety-stop/teardown paths must still be able to stop a known owned target after a recording failure: propagate the recording failure and actual stop result separately, never claim that unrecorded cleanup was durably accepted.” The same paragraph explicitly preserves the cessation checks for orphan cleanup; lines 118–161 of the implementation have already performed those checks before the failing branches. Design line 115 requires preserving the existing orphan stop entry and requested target semantics.

**Deterministic reproduction specification:** reuse the existing `orphanFixture` and counting HTTP cleanup handler in `internal/runner/recording_test.go`. Close the original writer, leaving the existing runner lock available and the saved route pointing to the handler. Pass `cleanupStopped` a Store wrapper whose Open delegates normally, whose reads and EnsureNode succeed, and whose Append returns an injected error for the cleanup intent. Invoke `cleanupStopped(state, id, &operation{Action: "scope"}, wrapper)`. Current control flow returns the injected error at line 207 with **zero cleanup requests**. The required result is one stop request to the same root target, with the recording failure still returned and no claim of durable acceptance. A second case can fail EnsureNode to cover line 191. Busy writer, busy runner lock and unknown-scope cases must continue dispatching nothing.

This is a source-established reproduction, not a newly executed fault-injection test: the read-only review did not add or modify test files. The existing orphan tests demonstrate exclusion, single dispatch and unchanged-poll suppression, but do not inject a recording failure before the stop. The passing root cleanup matrix covers retained in-process cleanup, not this separate helper.

**Correction:** once the store/liveness checks and target validation have succeeded, preserve the recording error while allowing the existing explicit safety stop to reach that target once. Report recording and native-stop results separately; do not manufacture committed cleanup facts. Retain the current refusal to dispatch when ownership, cessation or target identity cannot be established. This needs no automatic storage repair, replay, owner takeover or new retry policy.

## Evidence inspected

Inspected the public storage contract and directory implementation; identity allocation and action/event/blob recording; scope, group, session, command, service, interview, artifact and supervisor integration; backend identity validation and executor/task-daemon propagation; HTTP and messaging transport changes and plugin registration; standalone conversation/environment producers; host/runner admission and project-store resolution; observation initialization, projections and historical readers; retained and orphan cleanup; generated transport/analyzer changes; examples; new tests and the broader fixture migration. Earlier review conclusions were context, not substitutes for inspecting current code.

Current proof index:
`/Users/tyler/.codex/visualizations/2026/10/19/01a1522a-1e4c-7942-bdce-772e9ed181e2/recording-foundation/astra-fixes-evidence.md`.

Read that index, its earlier `review-evidence.md`, the supplied public-Store inspection utility, and the latest check-log endings. The index accurately distinguishes current HEAD's regression qualification from the live application cycle at `a9cc64eb03d2895fb5befe1f11506d3f748ec496`. The saved Linux binary independently reports that preceding revision and `vcs.modified=false`; I did not relabel it as HEAD proof.

A read-only decoder independently checked the four current proof stores' frame lengths/checksums, contiguous sequences, node/reference ordering, all referenced blobs' sizes and SHA256 hashes, cleanup projections and absence of run-local executor spools:

| Run | Records | Nodes | Blobs | Blob bytes | Projection |
| --- | ---: | ---: | ---: | ---: | --- |
| `01M5AQHGZGE9E41A57W6RPS55Y` | 110 | 11 | 3 | 13 | completed/released |
| `01M5AQK6VGKXMFKZ6Y43XAQS7R` | 19 | 3 | 2 | 21 | completed/released |
| `01M5AQKS7ZVKQAACJE2Z7DKBZ1` | 17 | 2 | 1 | 0 | cancelled/released |
| `01M5AQH274ZBHMJEGCJ8W98W3M` | 24 | 4 | 2 | 7 | failed/released |

The before/after summaries are byte-identical, SHA256 `56f8e726d585e468c3e6b68a6530cc83a82360775733295468d412c2b0057f0b`. The late-service command projection retains its ended time, exit 0, interrupted state, `stopped` output and both output references.

Independently viewed `native-transcript-astra-settled.png`, `late-service-reopened.png` and `remote-reopened-astra-settled.png`. They show the two native answers with remembered Darwin context, late service output, and remote test command/output. The supplied native run used Codex **luna**. I did not start another native turn, remote container, app restart or UI cancellation, and did not acquire a writer on the supplied stores. Those live operations remain supplied evidence, corroborated by their saved artifacts.

## Checks executed

```sh
go test ./runstore/... ./internal/runner ./internal/taskdaemon ./internal/environments . -run 'Test(Ownership|Slots|Interrupted|Incomplete|Buffered|LostCommit|RunRequires|IdentityTree|FailedInitialization|PersistenceFailure|OldDaemon|UnidentifiedBackend|RetainedCleanup|NewFormatObservation|SavedFacts|CommandOutputSurvives|EngineUses|NewCleanup|Sweeper|Orphan|.*Recording|.*Identity|.*Rejected|CancellationDuringSessionAdmission|ProjectionMarkerFailure|LateService)' -count=1
```

Passed: directory 0.393 s, runner 0.847 s, taskdaemon 0.529 s, environments 1.332 s, root 4.613 s. The public runstore package has no tests itself.

```sh
go test -race ./runstore/directory . -run 'Test(BufferedEventsDoNotWaitForSyncAndTerminalDoes|SlotsAndAtomicConflicts|RecordingFailureDoesNotRetainStoppedResources|CancellationDuringSessionAdmissionReleasesEveryNativeSession|ProjectionMarkerFailureReleasesStoreWriter|RetainedCleanupOwnsStoreAndAppendsAfterImmutableOutcome|LateServiceResultRetainsWriterUntilExplicitCleanup|PersistenceFailurePreventsDispatchAndDoesNotRepeatCompletedWork)$' -count=1
```

Passed: directory 1.281 s, root 4.265 s. `git diff --check c5c9b142 HEAD` also passed.

These are focused checks, not a new full-suite, build, lint or frontend qualification. The supplied proof's acknowledged coverage edges—buffer capacity and segment rolling, some startup lock windows, mixed settled-run restart, context-only override rejection and completion inversion—remain limitations rather than newly demonstrated failures. The reported finding concerns an explicit existing requirement and a concrete early-return path, not expansion into those untested areas or into execution recovery.

## Outcome

material findings remain
