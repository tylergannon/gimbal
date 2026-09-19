# Complete the live-state UI bundle

The whole definition of done is /Users/tyler/.codex/worktrees/803e/gimble/ephemeral/requirements/ui-live-state/requirements.md, including both issue files it names. Fulfill that complete bundle. Optional finishing work may be deferred per the repository completion rule; missing required behavior or invalid evidence is substantial.

The previous Luna run was stopped at the user's request because it was too slow and missed defects. The user explicitly requests the workflow's default models. Preserve the useful working-tree changes, assess them, and finish the remaining work in a coherent assignment where practical. Do not repeat preliminary repository research or discard the implementation wholesale. Parent owns git delivery.

Read the independent review at /tmp/gimble-ui-live-state-review.md. It identifies two concrete remaining implementation defects: mixed route/observation revision counters suppressing a data update after open, and the map timer restarting on every snapshot. Verify and repair them with focused browser regressions.

Visible connection proof remains missing: idle Live, outage duration across retries, reconnect to Live, recorded display, and an open followed by exactly one changed data event. Existing map/history browser tests cover interrupted states and ticking/frozen folded durations.

The E2E failure in e2e/steps/app.ts is a false positive: an assertion excludes the empty-state sentence from the entire detail pane, but the selected agent transcript quotes that sentence from the test source. Assert against the real empty-state element and retain a positive selected-work assertion; do not remove validation or broaden into issue #270.

Required check evidence remains the exact command in the original requirements: just build && just vet && just test && just fmt-check && just e2e ui-live-state. The current installed implement interface no longer accepts a validation-command flag; the planner/worker must run this command and the validator must inspect its actual result. Prior runs reached E2E and failed; only a later successful result supersedes those failures.

The prior task.2 validator incorrectly marked complete for its selected SSE assignment alone. The fresh validator must assess both issues and all substantial proof requirements, not merely the latest task. Existing passing focused checks need repetition only after relevant changes or for independent evidence still needed. Do not inspect whole multi-megabyte prior transcripts or error-context dumps; they quote each other and add noise. Source, focused tests, this handoff, and the independent review give the needed context.
