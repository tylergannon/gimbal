# Official pattern catalog

## Purpose

This leaf indexes the official patterns landing page as a source artifact. Its central architectural claim is that complex AI-powered behavior should be composed from discrete atomic decisions, with four documented patterns covering batching, uncertainty, aggregation, and handler selection.

## Key concepts

- **Atomic decisions compose into system behavior.** TypeSafe is positioned inside a larger application; the model supplies narrow decisions, while the application composes them. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/patterns.md:5-11`
- **Speculative fan-out** batches even conditionally relevant questions into one call, then lets code decide relevance; claimed benefits are cost and speed. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/patterns.md:13-18`
- **Confidence-gated routing** treats uncertainty as a second decision axis for safety and reliability. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/patterns.md:15-19`
- **Composite scoring** combines separately evaluated dimensions into an application-owned aggregate, targeting cost, reliability, and speed. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/patterns.md:15-20`
- **Intent routing** classifies requests so application code can select the appropriate handler, targeting cost and speed. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/patterns.md:15-20`

## Citation bookmarks

- Architectural premise: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/patterns.md:5-11`
- Four-pattern table: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/patterns.md:13-20`
- Request for community use cases: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/patterns.md:22-24`

## Themes

- Model judgment remains granular; control flow remains explicit in application code.
- The four patterns combine naturally: fan out coaching signals, score several dimensions, gate by confidence, then route to code, a specialist agent, or a human.
- Gimble’s likely leverage is a fast decision plane beside its observable Go workflow, not an opaque replacement for workflow logic.

## Gotchas

- The landing page assumes the reader already understands primitives and confidence; it is not a complete behavioral contract. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/patterns.md:9-11`
- The benefits column is qualitative and contains no benchmark data.
- The catalog advertises only four patterns and solicits additional use cases, so it should not be treated as an exhaustive architecture taxonomy. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/patterns.md:22-24`

## Task recipes

- **Map a supervision loop to the catalog:** batch candidate diagnoses (fan-out), derive inspectable run-health dimensions (composite score), make intervention risk depend on confidence (confidence gate), and select no-op/code/agent/human (intent route). Begin with the catalog at `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/patterns.md:13-20`, then open the corresponding detailed source leaf.
- **Reject an overbroad integration:** if the desired output is unbounded text or novel code, keep that step with a generative agent; reserve Jev for one of the catalog’s discrete-decision roles.

