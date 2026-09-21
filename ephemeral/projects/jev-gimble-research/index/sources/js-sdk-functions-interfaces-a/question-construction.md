# JavaScript question construction: `choice`, `noul`, `score`, and `Questions`

## Purpose

This leaf covers the JavaScript SDK’s three primitive constructors and the map used to send multiple questions. It is the starting point for building typed Jev supervision rubrics in TypeScript: named alternatives with `choice`, an independent probability with `noul`, and an ordered semantic scale with `score`.

## Primitive construction

### `choice<T>(instructions, criteria)`

- `choice` creates a question that selects among named alternatives and returns `ChoiceQuestion<T>`. Its generic `T` extends `ChoiceCriteria`, allowing the criteria object’s label keys to flow into response typing. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/functions/choice.md:5-18` `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/functions/choice.md:33-35`
- `instructions` accepts `EntryType`: text, JSON object, array, or `null`. `criteria` maps labels to descriptions, with `null` permitted for undescribed labels. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/functions/choice.md:19-31`
- The resulting interface is discriminated by `type: "choice"`; its criteria retain generic type `T`, while instructions are optional/nullable `EntryType`. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/ChoiceQuestion.md:5-13` `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/ChoiceQuestion.md:19-47`

### `noul(instructions?, criteria?)`

- `noul` creates a yes/no question and returns `NoulQuestion`. Unlike `choice` and `score`, both arguments are optional and instructions default to `null`. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/functions/noul.md:5-19` `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/functions/noul.md:56-58`
- Criteria can be `null` or an object with independently optional `true` and `false` `EntryType` descriptions. The constructor therefore permits neither, one, or both outcome descriptions. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/functions/noul.md:21-50`
- The interface repeats the same optionality and uses the discriminator `type: "noul"`. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/NoulQuestion.md:5-24` `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/NoulQuestion.md:59-77`

### `score<T>(instructions, criteria)`

- `score` creates an ordered-rubric question and returns `ScoreQuestion<T>`; its generic criteria type extends `ScoreCriteria`. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/functions/score.md:5-18` `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/functions/score.md:33-35`
- Instructions accept text, JSON object, array, or `null`. Criteria require at least two descriptions indexed from score zero; individual entries may be `null`. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/functions/score.md:19-31`

### `Questions`

- A request’s question collection is a string-keyed map, `[name: string]: Question`; those names identify the corresponding answers. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/Questions.md:5-13`

## Citation bookmarks

- Choice constructor: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/functions/choice.md:5-35`
- Noul constructor: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/functions/noul.md:5-58`
- Score constructor: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/functions/score.md:5-35`
- Choice and Noul discriminators: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/ChoiceQuestion.md:41-47` `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/NoulQuestion.md:69-77`
- Named question map: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/Questions.md:5-13`

## Themes

- **Pick the primitive by semantic shape:** `noul` for an independent binary proposition, `choice` for mutually exclusive named alternatives, `score` for ordered levels.
- **Question names are protocol keys:** use stable, machine-oriented names because they are the join between authored rubric and returned answer.
- **Criteria are the contract:** human-readable descriptions, not label names alone, define operational meaning.
- **Structured instructions are supported:** question instructions are not restricted to strings, enabling relevant structured context within an `EntryType`.

## Gotchas and gaps

- Optional Noul instructions make an empty or underspecified question type-correct. For supervision, treat missing instructions as an authoring defect even though the SDK permits it.
- `null` descriptions are legal across primitives, but sparse criteria weaken semantic clarity and later auditability.
- A `Choice` encodes mutual exclusion. Multiple simultaneous coaching needs should be separate Nouls rather than one forced winner.
- A `Score` requires order and at least two levels; do not use it for unordered routes merely to obtain a number.
- `Questions` is broadly string-indexed and the assigned docs do not show a generic relationship between the whole question map and answer-map keys. Preserve keys with local `as const`/helper typing if compile-time cross-map guarantees matter.
- The assigned sources contain signatures only—no request example, option-count limit, runtime validation behavior, or malformed-criteria error documentation.

## Task recipes

1. **Author a supervision rubric:** create independent `noul` questions for observable conditions; add a `choice` only for mutually exclusive intervention routes; use `score` when the middle/ordering has real semantics.
2. **Keep names stable:** define question keys as constants, persist them with stored answer vectors, and version changes to wording or criteria separately from keys.
3. **Validate before calling:** locally reject blank instructions, fewer than two Choice labels, fewer than two Score levels, duplicate/unstable keys, and unlabeled criteria where auditability is required.
4. **Separate policy:** constructors define what Jev measures. Keep thresholds, precedence, retries, and workflow actions in ordinary code.
