# Fixed release candidate assembly

decision: This is a staged artifact-assembly prerequisite. Keep automatic release dispatch, the publish default and existing source/platform gates unchanged. No tagged/native build, GUI/model session, service lifecycle action, credentials change or publication is authorized for this task. The result is a draft PR.

decision: Reuse the clean task worktree on codex/fixed-release-candidate from a3ee508b. Preserve the older root checkout's uncommitted inventory edits and .gimbal.

decision: Extend the existing release-assets tooling to assemble and verify a fixed publication directory. The manifest records repository/source/tag/run/attempt and hashes of the seven final files; the Actions upload ID/digest belong in job outputs and summary outside the bundle. The manifest is identity metadata, with no acceptance verdict.

decision: Keep the existing two-day Actions artifact lifetime visible as a constraint. Scheduling, evaluator procedures, candidate selection, promotion authority and a separate promotion workflow remain outside this change.

decision: Publisher retries must use the assembly job's saved run attempt, not the retry's current github.run_attempt, so a failed publisher can consume the same immutable artifact without rebuilding it.

friction: actions/download-artifact v4 selects the named subdirectory for ID-based downloads unless merge-multiple is true, even for one ID. Read its owning implementation and set the fixed-candidate consumer explicitly; static workflow checks cover this boundary.
