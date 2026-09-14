# Issue 135: remaining native event coverage

Start from origin/main 2ac8940. Read the current issue and comments in
/Users/tyler/.codex/worktrees/8e2d/gimble/ephemeral/issue-135/remaining-issue.json
and the previous live result in ephemeral/issue-135/result.md.

The desired result is exact native model-response events and correct tool/final
rows for Codex fork and resume. PR #202 already fixed delayed and sequential
tool attribution and demonstrated Claude Haiku in the production browser;
preserve that work and do not redo it without a relevant regression.

1. Check the installed Codex version and its generated experimental app-server
   protocol. Determine whether fork/resume can now enable raw events through a
   supported native request. Distinguish another turn on the existing connection,
   socket-only reconnect, and resume after thread unload or daemon restart.
   The previous version exposed the flag only on thread/start; sending unknown
   fields on fork/resume did nothing. Verify capability before promising a fix.
2. Implement the smallest adapter change supported by the actual protocol.
   Preserve native fork/resume semantics and exact native response identifiers.
   Do not infer response completion from token notifications, synthesize IDs,
   reconstruct history, patch upstream Codex, or add exported APIs. If native
   support is still absent, record the precise blocker and complete any supported
   reconnect coverage; do not manufacture an implementation to fill the gap.
3. Reuse ephemeral/research/issue-130/proof for a cheap live production-handler
   browser run with Codex gpt-5.6-luna. Inspect native events and rendered tool
   and final rows for actual reconnect/fork/resume paths, identifying which path
   each run exercised. Use an isolated daemon for restart/unload experiments;
   never disrupt the user's shared daemon. Check usage on these rows as well,
   since the previous fork retained only the first model call's usage.
4. Add focused unit regressions where useful, run appropriate checks (just build,
   just vet, just test for code changes), and report live evidence separately
   from tests. No new validation framework or nontrivial proof machinery.
   Stop when the requested behavior works and validation passes; no cleanup or
   code-improvement loop. Document any remaining upstream limitation honestly.

Sol owns codex adapter changes, focused tests, minimal existing browser fixture
changes, and local result/worklog files in this worktree. Other agents may work
in the repository: do not revert their edits. Commit and push meaningful work
and open a scoped PR if there is a viable change. Report its exact head, checks,
live artifacts, and any unmet acceptance to the parent. The parent arranges a
Claude Opus second opinion and the already-authorized merge. Do not close #135
while exact fork/post-restart events remain unavailable. Do not touch #117.
