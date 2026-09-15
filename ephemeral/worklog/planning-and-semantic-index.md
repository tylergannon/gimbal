# Planning and semantic index completion

correction: The user correctly called out that the first builtin delivery implemented only draft/critique/synthesis, not the full Sprint Plan behavior, and omitted Semantic Index as a builtin. A working narrow fixture did not establish complete skill conversion.
decision: User clarified Semantic Index means a reusable builtin workflow. The existing flat graph-research index is a separate artifact, not fulfillment.
decision: Convert behavior into ordinary Gimble scopes and sessions; do not execute or reproduce Diffusion CLI/ledger machinery verbatim. Track each source-skill capability against shipped behavior before claiming completion.

correction: Helpers existing in source did not mean the production path called them: semantic discovery was initially disconnected and tests passed explicit paths. Added an implicit project-pointer test through the actual planning sequence.
correction: A fake model returning multiple questions ignored a prompt that demanded Question empty. Verify prompt intent as well as scripted output handling; the follow-up now permits another necessary question and retains all answers.
correction: Merge notes must contain actual synthesis decisions; a prewritten instruction file or optional blank fields do not fulfill that artifact.
correction: `go test` with no test files does not establish build/update/preservation behavior. Semantic-index completion now requires actual fake-harness tests plus native retrieval against independent expectations.
correction: Native orientation initially drifted into an ordered implementation plan, anchoring subsequent independent drafts. Narrowed its prompt to observed facts, policy, citations, and uncertainty; the retry produced those categories without a proposed design.
proof: The native 12-source index build preserved all source SHA256 values. Blind retrieval walked actual routes/leaves/source files and matched three independently fixed expectations; generated eval target lists were not counted as retrieval success.
friction: Native Sprint Plan's first Gemini draft failed with external 503 capacity exhaustion. Preserve that failed run as incomplete evidence and retry; do not report it as a completed interview/synthesis.
proof: Native update after one intentional edit and one deletion used exactly one reader, removed the deleted leaf, preserved ten unchanged leaves, and passed source-preservation checks. Stale and clean audits made no index writes.
friction: Second live Plan reached a real Gemini response but failed the agy adapter's unsettled-tool check. Investigate event semantics using that exact transcript rather than disabling lifecycle validation or counting the partial plan as done.
decision: Do not infer that a pending native tool settled merely because a later step completed. Require raw provider terminal evidence or preserve the failure; an attractive recovery heuristic can falsify runtime claims.
finding: A focused raw agy probe showed missing-file tools emit ACTIVE then ERROR with tool_info.error, followed by ordinary successful tools. The adapter handled DONE only. Recognizing the provider's explicit ERROR terminal state fixes the actual cause without inferring success or accepting unresolved ACTIVE tools.
correction: Final synthesis must provide its own decisions after interview; retaining a nonempty pre-interview placeholder when the final response omits decisions can falsely publish preliminary notes. Removed that fallback and added a regression that refuses to publish plan/merge notes in that case.
