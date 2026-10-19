# Independent adversarial review: run-store foundation, round 03

Date: 2026-10-19

## Target and authority

Reviewed the complete branch delta from `c5c9b142c7de2270863c0811d7958bd27bdfa88a` to HEAD `7e0ca74b2c379f329167a93f40faa14223120e89`, including relevant surrounding application code. The working tree was clean at review start and remained unchanged before writing this artifact.

Authority: the current user request; `ephemeral/run-store-astra-review-request.md` and its referenced `run-store-request.md`, `run-store-design.md`, and `run-store-session.md`; repository `AGENTS.md`; the current public Godoc; `docs/definition-of-done.md`; and the relevant API design, sprint and web-app documentation. The latest human priorities concerning useful inexpensive tests, compact implementation and ordinary failure handling apply to the entire change. The review-request document's original commit range is superseded by the explicit instruction to review current HEAD.

Performed independently under `/Users/tyler/.agents/skills/adversarial-review/SKILL.md`, without delegation. There was no caller instruction narrowing permissible defects or prescribing a verdict. The read-only instruction supersedes worklog, commit and implementation workflows. No implementation or test files were edited; this is the only new review artifact.

## Assessment

**Material findings remain: one issue.** Both round-02 findings are fixed, and their regression tests pass, including under the race detector. The remaining issue is another consequence of the already-tested late-service interleaving: authoritative recording survives, but the normal command projection has already been closed. I reproduced the discrepancy while running the existing test without modifying it.

The storage seam is substantive: node identities, intents, bindings, outcomes, normalized events and owned blobs pass through the replaceable run handle. The directory implementation and current proof support exclusive ownership and independent readback. I found no additional material application bypass or justification for adding a recovery framework.

The added tests generally protect meaningful contracts. Fresh-process tests reuse the existing test executable rather than repeatedly compiling fixtures. Cancellation/admission and recording-failure regressions use channels or direct fault injection. The late-service regression deliberately exercises a shortened cleanup timeout of 50 ms per boundary; that wait tests the behavior in question. The blocked-sync test's 20 ms negative assertion is a small scheduling heuristic, not a material resource-cost problem. The broad test migration includes some unnecessary identity-context additions to unrelated authentication-session tests, but these are not a material blocker. No new automatic replay, migration framework or resource-reconstruction subsystem is needed to address the finding below.

## Finding

### 1. Issue: a late service result is durably recorded but permanently omitted from the ordinary command view

**Evidence:** `run.go:330–347`; `remote_execution.go:126–144` (and the equivalent completion paths in `messageService` and `ownedService.wait`); `event_persistence.go:160–174,177–189`; `internal/observation/store.go:180–192,276–280`; `run.go:565–575`; `run_cleanup_recording_test.go:143–214`.

When a service's final result remains blocked beyond the cleanup budget, Run correctly retains the service and authoritative store writer. However, `run.finish` unconditionally closes the observation store and `run.jsonl` writer before returning, even while `pendingServices` is nonempty.

When that result subsequently arrives, the service goroutine commits its outcome and `command_ended` lifecycle fact through the retained run store. Its projection step then writes through the already-closed run-log file, producing an observation error. The reducer also discards the lifecycle record because `Store.Lifecycle` returns immediately when `s.closed` is true. Explicit cleanup later records root release and closes the authoritative writer, but does not project the missing command result. Its return path does not surface the late observation error either.

Consequently the ordinary command row retains `ended: 0`, empty output references and its initial fields after cleanup is confirmed released. The complete outcome is available through Store readback, but a UI reload cannot recover it from the existing projections: the final event is absent from `run.jsonl` as well as the command table. This is an ordinary slow-result/cleanup-budget interleaving, without a crash, storage failure or power-loss scenario.

**Missed requirement:** `ephemeral/run-store-design.md`, “Directory implementation and existing observations,” requires new-run recording through the store followed by the existing reducers/live streams. Its terminal-cleanup section distinguishes body return from release of the recording owner. `ephemeral/run-store-session.md`, implementation step 3, requires preserving retained cleanup and routing late terminal facts through the owned handle; its observation acceptance requires commands and transcripts to continue appearing normally. Retaining the authoritative handle fixes ownership but does not finish the accompanying projection lifetime. This finding does not request crash reconstruction or execution replay.

**Executed reproduction:** ran the unchanged existing test:

```sh
go test . -run '^TestLateServiceResultRetainsWriterUntilExplicitCleanup$' -count=1
```

A separate read-only loop inspected that test's temporary segment, `commands.json`, `run.json`, and `run.jsonl` while the fixture was alive. It decoded complete checksummed segment frames and captured the state after the service completion and root release. The test passed in 0.706 s. For run `01M5AQ7TBKQR4MFY3KQV3W3SFR`, the observation was:

- authoritative lifecycle stream contained one `command_ended` fact;
- root projection had `cleanup_status: released`;
- command `server.1` still had `ended: 0`, `duration: 0`, `interrupted: false`, and empty stdout/stderr file references;
- `run.jsonl` contained no `command_ended` event.

The fixture deliberately holds the service result until Run has returned, then releases it, waits for `service.done`, queries the live controller and performs explicit cleanup. Its final assertions check root cleanup and writer release, but not the command's final projection. The successful test therefore does not contradict this observed failure. No test source or fixture was changed for this reproduction, and the temporary files were removed by the test's normal cleanup.

**Correction:** let the existing owner project retained service completion before closing the relevant run-log/reducer lifetime, or explicitly deliver that completion through the existing owned cleanup path. Preserve the already-committed run terminal verdict. Extend this same regression to check the completed command row and final output references in both the live registry and a fresh ordinary projection reader. This needs an ownership/finalization correction, not a new recovery system or another receipt layer.

## Prior findings

- **RunCommand hid outcome recording failure:** fixed. `RunCommand` now joins the command and recording errors while retaining the known exit code. The existing injected-outcome test asserts the error inside the workflow and verifies exactly one native submission.
- **Status queries abandoned the retained service writer:** fixed. `CleanupPending` no longer removes services. The explicit cleanup owner records root release and closes the writer before the controller disappears. The regression verifies exclusive reopen and released root projection. Finding 1 concerns the separate command projection, not a recurrence of the writer leak.
- The earlier session/scope cleanup error separation, cancellation during session creation/fork, and projection-marker initialization ownership fixes remain covered by passing focused tests and race checks.

## Evidence inspected

Inspected the changed storage interface and complete directory implementation; node/action/event/blob recording; scope, group, session, command, service, interview, artifact and supervisor integration; execution identity validation; HTTP/messaging wire changes and plugin registrations; native executor/task-daemon plumbing; standalone producers; host/runner admission and store selection; observation initialization and historical/new-format separation; retained and orphan cleanup; generated transport and analyzer changes; examples; added tests and the mechanical fixture migration. Also inspected prior review reports and worklog context without treating their conclusions as authoritative.

Current proof index:
`/Users/tyler/.codex/visualizations/2026/10/19/01a1522a-1e4c-7942-bdce-772e9ed181e2/recording-foundation/astra-fixes-evidence.md`.

Independently inspected `native-transcript-astra-final.png` and `remote-reopened-astra-final.png`. They show both native answers, including remembered Darwin context, and the remote test command/output. The supplied native run used Codex **luna**. Read the disposable public-Store inspection utility's source and the current check-log endings. The saved Linux production binary reports the reviewed HEAD and `vcs.modified=false`.

Read-only inspection of the three current proof stores independently verified complete frame checksums, contiguous record sequences, parent/reference ordering, every referenced blob's size and SHA256, released cleanup projections and absence of each run-local executor spool:

| Run | Records | Nodes | Blobs | Blob bytes |
| --- | ---: | ---: | ---: | ---: |
| `01M5APV9RY5CM1JB0M6XVBQFV9` | 104 | 11 | 3 | 13 |
| `01M5APVB6H172G9DGR23ZSWX48` | 19 | 3 | 2 | 21 |
| `01M5APVCB0S6AVMAVVRY8X34T0` | 17 | 2 | 1 | 0 |

The final before/after summaries are byte-identical, SHA256 `60cde4cdddd9cdfbacd17de19cc9bd124b81ffe143478852a5bbe28437e0c058`. These checks corroborate the supplied normal-run readback evidence. I did not launch another native turn, remote container, app restart or UI cancellation, and did not alter the proof stores with Store.Open.

## Checks executed in this review

```sh
go test ./runstore/... ./internal/runner ./internal/taskdaemon ./internal/environments . -run 'Test(Ownership|Slots|Interrupted|Incomplete|Buffered|LostCommit|RunRequires|IdentityTree|FailedInitialization|PersistenceFailure|OldDaemon|UnidentifiedBackend|RetainedCleanup|NewFormatObservation|SavedFacts|CommandOutputSurvives|EngineUses|NewCleanup|Sweeper|Orphan|.*Recording|.*Identity|.*Rejected|CancellationDuringSessionAdmission|ProjectionMarkerFailure|LateService)' -count=1
```

Passed: directory 0.534 s, runner 0.690 s, taskdaemon 0.417 s, environments 1.118 s, root 4.367 s. The public runstore package has no tests of its own.

```sh
go test -race ./runstore/directory . -run 'Test(BufferedEventsDoNotWaitForSyncAndTerminalDoes|SlotsAndAtomicConflicts|RecordingFailureDoesNotRetainStoppedResources|CancellationDuringSessionAdmissionReleasesEveryNativeSession|ProjectionMarkerFailureReleasesStoreWriter|RetainedCleanupOwnsStoreAndAppendsAfterImmutableOutcome|LateServiceResultRetainsWriterUntilExplicitCleanup|PersistenceFailurePreventsDispatchAndDoesNotRepeatCompletedWork)$' -count=1
```

Passed: directory 1.291 s, root 3.889 s. Also ran the single late-service test with the read-only observation described in finding 1. `git diff --check c5c9b142 HEAD` passed.

These are focused checks, not a new full-suite, build, lint or frontend qualification. The supplied proof accurately separates its earlier full-suite results from the current affected-suite rerun. Its acknowledged remaining coverage edges—capacity/segment-roll boundaries, some startup lock windows, mixed settled-run restart, context-only override rejection and completion inversion—remain limits rather than independently demonstrated results here. I found no concrete additional failure warranting a separate material finding for those edges.

## Outcome

material findings remain
