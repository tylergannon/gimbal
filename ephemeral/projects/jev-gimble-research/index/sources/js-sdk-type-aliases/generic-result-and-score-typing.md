# Generic result mapping and score-key preservation

## Purpose

This leaf captures how the JavaScript SDK maps each question type to its answer type and how literal Choice keys and Score rubric indices survive into TypeScript results.

## Key concepts

- **`ResultFor<T>` maps questions to responses with a conditional type.** Noul maps to `NoulResponse`; Score infers its criteria type `S` and maps to `ScoreResponse<S>`; Choice infers criteria `E` and maps to `ChoiceResponse<E>`; no recognized question maps to `never`. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/type-aliases/ResultFor.md:5-17`
- **Criteria keys survive answer typing.** The source explicitly states that the answer type preserves criteria keys, making stable Choice labels and Score indices visible to consuming code. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/type-aliases/ResultFor.md:7-17`
- **`ScoreOf<T>` distinguishes fixed tuples from widened criteria.** If `T["length"]` is generic `number`, the score key is `number`; for a fixed tuple, the alias extracts its numeric index keys. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/type-aliases/ScoreOf.md:5-17`
- **`ScoreLegend<T>` is keyed by exactly those inferred scores.** Each score key maps back to the corresponding criterion entry, producing a readonly rubric legend. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/type-aliases/ScoreLegend.md:5-17`
- **The minimum two-entry Score tuple underlies the generic mapping.** Exact score-key typing is useful only when the rubric remains a fixed tuple rather than being widened before it reaches `ScoreQuestion`. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/type-aliases/ScoreCriteria.md:5-11` `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/type-aliases/ScoreOf.md:7-11`

## Citation bookmarks

- Question-to-response conditional type: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/type-aliases/ResultFor.md:5-17`
- Score-key inference rule: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/type-aliases/ScoreOf.md:5-17`
- Legend mapping back to criteria: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/type-aliases/ScoreLegend.md:5-17`
- Score criteria base tuple: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/type-aliases/ScoreCriteria.md:5-11`

## Themes

- The type system connects input policy literals to output handling.
- Narrow literal types provide more useful compile-time exhaustiveness than dynamically widened arrays and maps.
- Response typing protects integration code, but it does not validate model accuracy or runtime payloads received outside the SDK.

## Gotchas

- If a fixed Score rubric is widened to generic `ScoreCriteria`/array form too early, `ScoreOf<T>` deliberately degrades to `number`, losing exact index keys.
- `ScoreOf`’s implementation extracts tuple keys matching the TypeScript numeric-string template; do not assume this gives a runtime range check for arbitrary numbers.
- Conditional generic correctness applies at compile time. Casts, `any`, untyped JSON persistence, or cross-process messages can bypass it.
- A typed Choice label proves membership in the declared criteria type, not that the label is semantically correct for the state.

## Task recipes

- **Preserve exhaustive supervision routes:** define question criteria as narrow literals and let the SDK infer the response type; avoid unnecessary widening or `Record<string, ...>` annotations before constructing the question.
- **Keep score/legend pairs auditable:** use the typed `ScoreLegend<T>` when displaying or storing a score so a numeric result remains connected to the exact rubric entry. Start at `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/type-aliases/ScoreLegend.md:5-17`.
- **Protect persistence boundaries:** validate stored/replayed Jev results before treating them as `ResultFor<T>`, since compile-time conditional types do not verify external JSON.

