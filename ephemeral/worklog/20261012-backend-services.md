decision: Implement only the agreed orchestration/secrets migration and runnable non-trivial workflows. User explicitly requires independent subagent execution and old-interface retirement verification. No complicated proof framework. Stop for approval before extra correctness machinery beyond a few hundred lines.
decision: Keep all changes local and staged; prior no-deployment instruction remains.
correction: Independent review found shared-store release must follow aggregate cleanup, and environment release must follow native session close. Reused existing cleanup paths; no new recovery machinery.
correction: A child turn on an ancestor-owned session still needs child-scope cleanup delivered to that daemon. Empty remaining environment ownership permits parent cleanup acknowledgement without dispatch to an unused base queue.
friction: Local psqlrc changed query stdout; the built-in service workflow uses psql -X for predictable program consumption.
decision: User subsequently authorized rebase, commit, push, and PR with live web screenshots; deployment and merge remain outside this task.
friction: Commit hook found unchecked deferred cleanup returns missed by earlier vet; mark best-effort temporary-file and handle cleanup explicitly while retaining authoritative publication errors.

correction: The PR #476 owner review supersedes the earlier symbol-retirement conclusion: the old Docker/environment CLI and endpoint-label use remained. #480 and #481 are blockers; #478 is a follow-up, and #479 starts with an approval-gated design proposal.
decision: Parallel fixes separate environment retirement, declaration grammar/backend write shape, and source-derived graph visibility. Preserve the approved small lint solution for producer-local yields instead of introducing one-shot context state.
friction: Broad browser verification exposed an existing main layout regression: a truthy project selected ProjectWorkspace before the Runs loading branch, leaving About visible during delayed navigation. Restored RunsLoading inside the project workspace and scoped the test's detail selector after main introduced additional sidebars; kept all behavioral assertions.
