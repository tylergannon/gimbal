# Release diagnosis

correction: User requires a written diagnosis and Claude adversarial review after earlier chat answers conflated resolved publication failures and current observer failures. Check each active workflow and actual distribution before asserting current state.

correction: Release performance failures also skip dependent hourly hook measurement; local commit-hook execution is a separate capability. Python was already embedded in the October 12 appcast script, before October 13 standalone release scripts.

friction: Whole-file comparison of the bundled and standalone signed CLI differs after app packaging re-signs it. Compare code-directory identity and version/commit instead of treating signing timestamps as code divergence.

decision: Diagnosis only. Raw downloads/API responses/logs stay in /tmp; tracked artifacts are analytical notes and independent review. No release repair, installation, or production-catalog modification is authorized by this review request.

correction: User adds removal of Python to the remediation plan and delegates language choice. Select existing Go for structured automation and its tests, retain existing shell orchestration, include inline Python and all maintained release/CI/hook entry points. Preserve the fast hook path and prove the actual runner conditions.

decision: Claude Sonnet 5.5 independently reproduced the monitor defect and separately reverified downloaded package integrity/signatures/feed/casks, including updater signature through OpenSSL instead of the original CryptoKit probe. Reviewer wording about consecutive failures and issue update/close capability was corrected from source evidence.

decision: Claude review converged with only nitpicks after restoring observer-first ordering, specifying TypeScript/Node hook startup, expanding the maintained Python inventory, and preserving explicit test gates for Node and nested skill modules. Future migration remains unimplemented.

friction: Reviewer searches missed the existing mise Node 24.13.0 executable and incorrectly denied the observed probe. Explicit path execution reproduced v24.13.0 and warning-free TypeScript; negative discovery is not stronger than direct executable evidence.

decision: Tyler authorized execution of the reviewed recovery and no-Python plan. Restore and observe the independent release monitor before the remaining ports; overall completion includes all maintained skill helpers and inline/test Python dependencies.
correction: Live inventory also finds an inline Python HTTP server in local_service_fulfillment_test.go. Replace that fixture with the existing Go test executable so the full native release gate can run without Python.
friction: Historical GitHub data contains two cancelled jobs with completion one second before start, not only the initially diagnosed record. Recover by exposing each unavailable interval and continuing the scan, rather than excluding one known run ID.

friction: Direct Node tests inherit NODE_TEST_CONTEXT; nested frontend smoke tests then emit IPC instead of the TAP summary. Remove that inherited test context from hook child environments so direct CLI and fixture execution observe the same protocol.
friction: JavaScript string replacement interprets dollar-quote in a literal shell expression as a replacement token; a Go test edit was malformed and immediately rejected by formatting/compilation. Use a replacement callback or structured patch for shell literals containing dollars.
decision: The automation gate discovers each self-contained skill module explicitly, runs its tests/vet/lint, and poisons Python command lookup during executable checks. Borrowed-helper refreshes cannot silently reinstate tracked maintained Python files.
friction: Benchmark mode passed but actual no-argument lefthook failed on macOS Bash 3.2: nounset treats an empty argument array as unset. Independent validation reproduced the same defect. Cover the real no-argument Bash entry point, not only direct Node/coordinator or benchmark invocations.
friction: Complete language ports exceeded the 10000-added-line staged-content limit by 270 lines. Split coherent release/hook and installed-skill changes into separate commits; do not bypass the guard or trim required behavior to meet it.
correction: Claude's broader case-insensitive maintained-source scan found service-check's inline Python server and consumer, outside the standalone-helper inventory. Port this actual shipped workflow too, update its runtime prerequisite, regenerate its invocation metadata, and include inline command detection in the automation gate.
