# Fixed candidate completion

decision: Reused PR #664 and its existing branch in the fresh abb9 worktree. Live GitHub inspection on November 1 confirms draft head 17deb5b and main a3ee508b. The root checkout and all other worktrees are preserved.

decision: The earlier task's no-tag/no-native-build authorization was deliberately narrower. This task requires explicit Tyler approval of its written definition and proposed real build-only dispatch before implementation or consequential experiments.

friction: A branch-only dispatch cannot exercise the native packaging/candidate path: macos-release requires the dispatch ref itself to be a version tag. Plan an approved unpublished tag rather than presenting a skipped native job as upload proof.

decision: Repository instructions prohibit committed run-output/proof programs; report observed proof in chat/PR and keep only plans, worklog intelligence and independent review notes in ephemeral.

friction: Sonnet specification review session 036f4069-9601-4efe-bd99-196a8dddbf5f could not read the separate design worktree under default CLI directory permissions. Resume with an explicit read directory; do not treat inaccessible authority as reviewed.

decision: First independent review identified temporary stable-version tags as an input to automatic version preparation. Approval must include version-preparation coordination and deletion of this task's new tag; no existing tag may be moved or deleted.

decision: Build-only proof cannot establish actual publisher-job or retry behavior. Preserve the source/check-level claim and explicitly report that operational boundary rather than authorizing publication to complete the foundation.

decision: Sonnet spec consensus reached only-nitpicks in round 03, retained session 036f4069-9601-4efe-bd99-196a8dddbf5f. The task hands back the existing PR without merge; merge can activate automatic release depending on Conventional Commit selection and pending changes. The review's categorical claim that any release-scoped merge necessarily publishes is stronger than prepare-release source supports: nextVersion can return an empty tag.

decision: Unchanged-revision retry and repaired-source dispatch are distinct. New source requires approval of a new exact revision/tag; never repoint/recreate a test tag as a retry.

decision: Tyler delegated the retention choice ("whatever IDK"). Keep the existing two-day lifetime and complete verification before expiry; no new retention policy in this foundation.

correction: Tyler says to execute if the owner recommends approval; ask questions only for decisions actually requiring him. This approves the recommended bounded task, necessary repairs, and coordinated build-only tag/dispatch/cleanup. Do not request repeated approval for routine revision/tag selection within that scope. Merge/publication remain excluded.

decision: Terra found no necessary source changes. The previously unexercised boundary was actual Actions artifact transport, not another implementation. Keep the existing implementation and PR rather than inventing repairs.

decision: Independent Sonnet proof review reached only-nitpicks in fixed-candidate-completion-proof-round-01.md. Its earlier CLI shell permission denials were resolved for the authorized validation invocation; this final round actually executed independent local checks and GitHub API reads.

decision: Owned temporary tag was removed locally and remotely after verification, and A received explicit window release. Native GUI acceptance and actual publisher-job/retry behavior remain outside the proved foundation; do not reuse this build-only result as their qualification.

decision: After the foundation handback Tyler requested "continue". Integrate the ready PR as ci: with no publication: fresh main equals latest v0.20.2 and nextVersion ignores ci: when no other unreleased changes exist. Coordinate a short merge/version-preparation window through A and verify the actual main automation outcome before release of that window.

friction: Root checkout still contains the older uncommitted run-inventory edits and .gimbal. Preserve them; do not stash, discard or attempt a conflicting fast-forward merely to satisfy cleanup protocol.
