# JavaScript question construction: `choice`, `noul`, `score`, and `Questions`

## Purpose

This leaf covers the JavaScript SDK’s three primitive constructors and the map used to send multiple questions. It is the starting point for building typed Jev supervision rubrics in TypeScript: named alternatives with `choice`, an independent probability with `noul`, and an ordered semantic scale with `score`.

## Primitive construction

### `choice<T>(instructions, criteria)`

- `choice` creates a question that selects among named alternatives and returns `ChoiceQuestion<T>`. Its generic `T` extends `ChoiceCriteria`, allowing the criteria object’s label keys to flow into response typing. [choice](https://docs.typesafe.ai/sdk/javascript/api/functions/choice.md) [choice](https://docs.typesafe.ai/sdk/javascript/api/functions/choice.md)
- `instructions` accepts `EntryType`: text, JSON object, array, or `null`. `criteria` maps labels to descriptions, with `null` permitted for undescribed labels. [choice](https://docs.typesafe.ai/sdk/javascript/api/functions/choice.md)
- The resulting interface is discriminated by `type: "choice"`; its criteria retain generic type `T`, while instructions are optional/nullable `EntryType`. [ChoiceQuestion](https://docs.typesafe.ai/sdk/javascript/api/interfaces/ChoiceQuestion.md) [ChoiceQuestion](https://docs.typesafe.ai/sdk/javascript/api/interfaces/ChoiceQuestion.md)

### `noul(instructions?, criteria?)`

- `noul` creates a yes/no question and returns `NoulQuestion`. Unlike `choice` and `score`, both arguments are optional and instructions default to `null`. [noul](https://docs.typesafe.ai/sdk/javascript/api/functions/noul.md) [noul](https://docs.typesafe.ai/sdk/javascript/api/functions/noul.md)
- Criteria can be `null` or an object with independently optional `true` and `false` `EntryType` descriptions. The constructor therefore permits neither, one, or both outcome descriptions. [noul](https://docs.typesafe.ai/sdk/javascript/api/functions/noul.md)
- The interface repeats the same optionality and uses the discriminator `type: "noul"`. [NoulQuestion](https://docs.typesafe.ai/sdk/javascript/api/interfaces/NoulQuestion.md) [NoulQuestion](https://docs.typesafe.ai/sdk/javascript/api/interfaces/NoulQuestion.md)

### `score<T>(instructions, criteria)`

- `score` creates an ordered-rubric question and returns `ScoreQuestion<T>`; its generic criteria type extends `ScoreCriteria`. [score](https://docs.typesafe.ai/sdk/javascript/api/functions/score.md) [score](https://docs.typesafe.ai/sdk/javascript/api/functions/score.md)
- Instructions accept text, JSON object, array, or `null`. Criteria require at least two descriptions indexed from score zero; individual entries may be `null`. [score](https://docs.typesafe.ai/sdk/javascript/api/functions/score.md)

### `Questions`

- A request’s question collection is a string-keyed map, `[name: string]: Question`; those names identify the corresponding answers. [Questions](https://docs.typesafe.ai/sdk/javascript/api/interfaces/Questions.md)

## Citation bookmarks

- Choice constructor: [choice](https://docs.typesafe.ai/sdk/javascript/api/functions/choice.md)
- Noul constructor: [noul](https://docs.typesafe.ai/sdk/javascript/api/functions/noul.md)
- Score constructor: [score](https://docs.typesafe.ai/sdk/javascript/api/functions/score.md)
- Choice and Noul discriminators: [ChoiceQuestion](https://docs.typesafe.ai/sdk/javascript/api/interfaces/ChoiceQuestion.md) [NoulQuestion](https://docs.typesafe.ai/sdk/javascript/api/interfaces/NoulQuestion.md)
- Named question map: [Questions](https://docs.typesafe.ai/sdk/javascript/api/interfaces/Questions.md)

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
