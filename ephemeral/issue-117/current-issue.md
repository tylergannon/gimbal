The original Sprint 2 report predates the current normalized session event adapters. Both Claude and Codex now emit streaming text/reasoning deltas (and applicable tool deltas); that portion is implemented.

Remaining work: surface native harness errors/retries, approval requests, and nested subagent transcripts in the run record and observation page for Claude and Codex. Returning a Generate error alone is not an observable transcript event. Nested activity must stay associated with its parent tool without mixing parent/child text, steps, or usage.

Use the current normalized event vocabulary and the SDK/app-server's native callbacks. Preserve noninteractive approval behavior: observing a request must not introduce interactive approvals or weaken permissions. Emit retries when the harness reports them; do not invent retry orchestration.

Acceptance: focused regression checks plus real cheap-harness runs whose persisted native-derived events and production-browser rendering demonstrate the affected paths. Clearly identify any unsupported native capability; fixtures/unit tests alone are not live validation. Reuse the existing validation setup, with no new framework. Normal build/vet/test checks must pass before merge.

This issue is separate from #211, which tracks exact Codex raw response events and accounting on native fork and post-restart/unload resume. #135's completed work shipped in #202 and #207.
