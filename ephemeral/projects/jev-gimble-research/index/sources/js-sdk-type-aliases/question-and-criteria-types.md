# Question union and criteria contracts

## Purpose

This leaf describes the compile-time question union and the criteria shapes for Choice and Score, including minimum rubric size, label descriptions, and `null` semantics.

## Key concepts

- **`Question` is a discriminated union.** A question is one of `NoulQuestion`, `ScoreQuestion`, or `ChoiceQuestion`, identified by its `type` field. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/type-aliases/Question.md:5-14`
- **Choice criteria map arbitrary string labels to entries.** Each key is a candidate label and each value is an `EntryType`; a `null` entry leaves that label undescribed. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/type-aliases/ChoiceCriteria.md:5-17`
- **Score criteria are ordered and nontrivial by construction.** The readonly tuple requires at least two `EntryType` elements, and their positions are scores starting at zero. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/type-aliases/ScoreCriteria.md:5-11`
- **Every rubric entry can use the full entry grammar.** A score level or Choice label description can be text, JSON object/array, or `null`, because both resolve through `EntryType`. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/type-aliases/ChoiceCriteria.md:11-17` `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/type-aliases/ScoreCriteria.md:7-11`

## Citation bookmarks

- Complete question union: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/type-aliases/Question.md:5-14`
- Choice label-to-entry index signature: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/type-aliases/ChoiceCriteria.md:5-17`
- Score tuple minimum, ordering, and `null`: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/type-aliases/ScoreCriteria.md:5-11`
- Shared entry grammar: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/type-aliases/EntryType.md:5-17`

## Themes

- The primitive kind is explicit and machine-discriminable.
- Choice is keyed categorical policy; Score is ordered positional policy; Noul has no criteria alias here.
- Rubrics can be structured, but code retains the authoritative labels and score order.

## Gotchas

- `ChoiceCriteria`’s index signature does not encode a minimum option count, non-empty labels, stable ordering, or mutual exclusivity; runtime/API validation and evaluation must establish those properties.
- Score meaning depends entirely on array order. Reordering descriptions changes the semantics of every numeric score without changing the alias.
- `null` is allowed for undescribed labels/levels, but widespread use removes semantic guidance and can make model behavior harder to review.
- Question dispatch depends on the `type` discriminator; dynamically constructed objects that widen or mistype it can lose useful TypeScript narrowing.

## Task recipes

- **Define a coaching route:** use Choice keys as stable code-owned destinations such as `none`, `deterministic_check`, `coach_agent`, and `human_review`; give each a mutually exclusive description. Start at `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/type-aliases/ChoiceCriteria.md:5-17`.
- **Define intervention urgency:** use an ordered Score tuple with at least two concretely described levels, from safe/no intervention through immediate escalation; test that every downstream comparison matches the zero-based order. Start at `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/type-aliases/ScoreCriteria.md:5-11`.
- **Keep exhaustive handlers:** switch on each question/result discriminator and each fixed Choice label, with a safe fallback for externally sourced or widened data.

