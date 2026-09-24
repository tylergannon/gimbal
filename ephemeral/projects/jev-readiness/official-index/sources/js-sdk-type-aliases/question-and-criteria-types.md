# Question union and criteria contracts

## Purpose

This leaf describes the compile-time question union and the criteria shapes for Choice and Score, including minimum rubric size, label descriptions, and `null` semantics.

## Key concepts

- **`Question` is a discriminated union.** A question is one of `NoulQuestion`, `ScoreQuestion`, or `ChoiceQuestion`, identified by its `type` field. [Question](https://docs.typesafe.ai/sdk/javascript/api/type-aliases/Question.md)
- **Choice criteria map arbitrary string labels to entries.** Each key is a candidate label and each value is an `EntryType`; a `null` entry leaves that label undescribed. [ChoiceCriteria](https://docs.typesafe.ai/sdk/javascript/api/type-aliases/ChoiceCriteria.md)
- **Score criteria are ordered and nontrivial by construction.** The readonly tuple requires at least two `EntryType` elements, and their positions are scores starting at zero. [ScoreCriteria](https://docs.typesafe.ai/sdk/javascript/api/type-aliases/ScoreCriteria.md)
- **Every rubric entry can use the full entry grammar.** A score level or Choice label description can be text, JSON object/array, or `null`, because both resolve through `EntryType`. [ChoiceCriteria](https://docs.typesafe.ai/sdk/javascript/api/type-aliases/ChoiceCriteria.md) [ScoreCriteria](https://docs.typesafe.ai/sdk/javascript/api/type-aliases/ScoreCriteria.md)

## Citation bookmarks

- Complete question union: [Question](https://docs.typesafe.ai/sdk/javascript/api/type-aliases/Question.md)
- Choice label-to-entry index signature: [ChoiceCriteria](https://docs.typesafe.ai/sdk/javascript/api/type-aliases/ChoiceCriteria.md)
- Score tuple minimum, ordering, and `null`: [ScoreCriteria](https://docs.typesafe.ai/sdk/javascript/api/type-aliases/ScoreCriteria.md)
- Shared entry grammar: [EntryType](https://docs.typesafe.ai/sdk/javascript/api/type-aliases/EntryType.md)

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

- **Define a coaching route:** use Choice keys as stable code-owned destinations such as `none`, `deterministic_check`, `coach_agent`, and `human_review`; give each a mutually exclusive description. Start at [ChoiceCriteria](https://docs.typesafe.ai/sdk/javascript/api/type-aliases/ChoiceCriteria.md).
- **Define intervention urgency:** use an ordered Score tuple with at least two concretely described levels, from safe/no intervention through immediate escalation; test that every downstream comparison matches the zero-based order. Start at [ScoreCriteria](https://docs.typesafe.ai/sdk/javascript/api/type-aliases/ScoreCriteria.md).
- **Keep exhaustive handlers:** switch on each question/result discriminator and each fixed Choice label, with a safe fallback for externally sourced or widened data.

