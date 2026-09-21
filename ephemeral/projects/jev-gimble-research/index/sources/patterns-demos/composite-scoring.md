# Composite scoring

## Purpose

Composite scoring decomposes a fuzzy overall judgment into independent Jev Score questions, normalizes their outputs, and combines them using weights explicitly owned by application code.

## Key concepts

- **Decompose before aggregating.** The pattern evaluates independent dimensions separately and combines them only after the model returns. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/patterns/composite-scoring.md:5-7` `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/patterns/composite-scoring.md:238-242`
- **One state can serve several role-specific composites.** The resume example scores Python depth, team leadership, system design, and generalism once, then applies one weight vector for senior IC ranking and another for engineering-manager ranking. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/patterns/composite-scoring.md:244-265`
- **Rubrics are descriptive and ordered.** Each dimension uses concrete levels from absence through increasingly strong evidence, rather than asking the model for an opaque overall number. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/patterns/composite-scoring.md:267-324`
- **Normalization and weights are deterministic code.** The example divides four-level-index scores by four to reach 0–1 and calculates weighted sums in Python. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/patterns/composite-scoring.md:326-341`
- **Inspectability enables policy correction.** Because each subscore and each weight is visible, unexpected rankings can be diagnosed and weights adjusted rather than rewriting an opaque prompt. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/patterns/composite-scoring.md:328-343`

## Citation bookmarks

- Pattern premise: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/patterns/composite-scoring.md:238-242`
- Parallel dimensions and role-specific weight diagram: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/patterns/composite-scoring.md:244-265`
- Four complete rubrics: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/patterns/composite-scoring.md:267-324`
- Normalization and two weight vectors: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/patterns/composite-scoring.md:326-341`
- Inspectability rationale: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/patterns/composite-scoring.md:343-343`

## Themes

- Model output is evidence by dimension; code defines the objective.
- The same atomic evaluation can support multiple policies without another model call.
- Review and tuning become localized to rubric wording and weight constants.

## Gotchas

- Weighted sums can hide one catastrophic subscore. For supervision safety, pair the composite with hard code gates for dimensions that must never be averaged away.
- Normalization depends on rubric cardinality and score semantics; copying `/4` is only correct for a five-level rubric indexed 0–4.
- The page discusses inspecting scores and tuning weights but does not describe training/evaluation data, confidence treatment for individual scores, or how to avoid post-hoc overfitting.

## Task recipes

- **Build an observable run-health score:** ask separately about progress, repetition, instruction adherence, evidence quality, and risk; keep all dimension values visible; use Go-owned weights only for prioritization. Start at `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/patterns/composite-scoring.md:238-265`.
- **Support several supervisor policies:** reuse the same atomic scores with one weight vector for “needs coaching soon” and another for “needs immediate human attention.” The two-role analogue is `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/patterns/composite-scoring.md:326-341`.
- **Debug a bad intervention:** inspect the dimension scores first; if those match labels, tune code weights; if not, revise the affected rubric and reevaluate. The source rationale is `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/patterns/composite-scoring.md:343-343`.

