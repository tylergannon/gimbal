# Independent Antigravity release-recovery review

## Target and outcome

Reviewed the current `agy/agy.go` and `agy/agy_test.go` delta on `codex/issue-574-release-recovery`, based on merged issue-574 commit `aaf55a91`, against the public HarnessAdapter contract, repository instructions, and `docs/definition-of-done.md`. This is a bounded recovery of a release-qualification failure, not a reassessment of the already merged workspace feature.

**Outcome: no findings.** The diagnosis is supported by the actual failure and the standard library's documented pipe lifecycle. The fix is small, idiomatic, and addresses the cause without weakening the observation/result contract or the failing test.

No scope-narrowing instruction was supplied or accepted. Only this review artifact was edited.

## Diagnosis independently assessed

Both supplied release failures identify the same original test and the same error: the stream scanner read a pipe already closed before it could consume the authoritative successful result. The before-fix slow-observer regression reproduces that same closed-pipe error.

The original code obtained stdout through `Cmd.StdoutPipe` and called `Cmd.Wait` concurrently with the scanner. The installed Go documentation explicitly states that Wait closes this pipe after observing process exit and must not be called before all reads have completed. An observer callback can delay consumption while the process exits, allowing Wait to close the unread output. That causal explanation matches both release logs and the new reproduction; it does not depend on treating unsettled tool observations as execution failure.

## Implementation assessment

The adapter now owns an `os.Pipe` reader and assigns the writer directly to `command.Stdout`. Wait therefore no longer owns the scanner's reader. Closing the parent's writer immediately after Start prevents the parent from keeping EOF open itself; deferring reader closure covers both start failure and normal return. Existing interrupt/terminal handling retains the ability to close the reader promptly.

The change leaves authoritative result validation, conversation identity, cancellation, steering, observer-error propagation, structured output, and terminal descendant cleanup semantics in place. It does not manufacture tool success or replay a turn. Preserving the result that the process actually emitted directly supports the definition-of-done rule that degraded observation must not replace native success with failure.

The new regression exercises the real subprocess/pipe boundary using the existing helper executable: an early event pauses its observer while the native helper emits its result and exits. This is meaningful coverage of the changed lifecycle, rather than an assertion that mirrors the new implementation. The existing unsettled-tool regression remains intact. No new abstraction or provider-policy mechanism is introduced.

## Evidence and checks

Inspected the full delta, `runOnce`, `scanStream`, active-output interruption/closure, Unix process-group handling, event projection, adapter contract, existing cancellation/steering/terminal/descendant tests, and the supplied before/after and release-failure logs. Consulted the installed `go doc os/exec.Cmd.StdoutPipe` and `go doc os/exec.Cmd.Wait` as the primary API contract.

Independently executed on the current delta:

- `go test -count=10 ./agy`: **passed**, exit 0. This repeats the whole adapter package, including both the original failing test and the new regression, plus cancellation, steering, observer failure, structured output, and inherited-output cleanup tests.
- `go test -race -count=1 ./agy`: **passed**, exit 0.
- `git diff --check`: **passed**.

The supplied additional logs report 100 repetitions of the two relevant regressions, package/race success, and zero package lint issues. The supplied live-provider log records `AGY_DRAIN_OK` from the caller's Antigravity run with `gemini-3.8-flash-low`; I inspected that log but did not independently repeat the real provider invocation. The independently executed subprocess tests directly exercise the pipe ownership being changed and are the stronger evidence for this specific race correction.

This review accepts the recovery implementation and its causal regression. It does not claim that the release has been published: the actual release qualification and publication still need to succeed on the committed recovery revision.
