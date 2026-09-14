# Codex: verify whether cacheWriteInputTokens sits inside inputTokens

URL: https://github.com/tylergannon/gimble/issues/151
State: closed
Updated: 2026-09-13T17:35:34Z

Follow-up from the token accounting port (#149, branch claude/issue-148-plan-ae018a).

`codex/events.go` maps `input = inputTokens - cachedInputTokens`. OpenCode's `packages/ai/src/protocols/open-responses.ts` subtracts both cached and cache-write tokens. Every live sample in the probe (`ephemeral/research/issue-149/FINDINGS.md`) had `cacheWriteInputTokens: 0`, so whether cache write is a subset of `inputTokens` on Codex app-server is unverified, and the adapter deliberately does not subtract it.

If a model that reports nonzero cache write appears, take one live sample and settle it: if cache write is inside `inputTokens`, subtract it too.
