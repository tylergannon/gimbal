# JSON, state, instruction, and description types

## Purpose

This leaf captures the JavaScript SDK’s serializable value grammar and the narrower top-level entry shape used for state, instructions, and criterion descriptions.

## Key concepts

- **`JsonValue` is the recursive JSON value union.** It permits strings, numbers, booleans, `null`, arrays of JSON values, and string-keyed objects whose values are JSON values. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/type-aliases/JsonValue.md:5-19`
- **`EntryType` is narrower at the top level.** State, instructions, and criteria may be a string, a JSON object, a JSON array, or `null`; a bare number or boolean is not a documented top-level `EntryType`, even though either may appear inside an object or array as `JsonValue`. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/type-aliases/EntryType.md:5-17`
- **Structured entries remain fully JSON-compatible.** Objects use arbitrary string keys and recursively JSON-compatible values; arrays likewise contain `JsonValue`. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/type-aliases/EntryType.md:7-15` `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/type-aliases/JsonValue.md:7-17`
- **`Description` is exactly `EntryType`.** A criterion description can therefore be text, structured JSON object/array, or `null`; `null` deliberately leaves the label undescribed. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/type-aliases/Description.md:5-11`

## Citation bookmarks

- Full JSON-compatible grammar: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/type-aliases/JsonValue.md:5-19`
- Top-level entry grammar and intended uses: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/type-aliases/EntryType.md:5-17`
- Description alias and `null` semantics: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/type-aliases/Description.md:5-11`

## Themes

- The SDK supports machine-readable structured context without accepting arbitrary JavaScript objects.
- The same entry grammar spans state, question instructions, and criterion descriptions.
- `null` is a meaningful contract value for “undescribed,” not merely an absent property.

## Gotchas

- Top-level `number` and `boolean` are `JsonValue` but not `EntryType`; wrap scalar state in a string, object, or array when necessary.
- `undefined`, functions, symbols, bigint, class instances, `Date`, `Map`, `Set`, cyclic objects, and binary values are outside the documented JSON grammar.
- The aliases express shape, not size/depth limits or canonical serialization. Deep or large run snapshots may still fail an undocumented API limit or create avoidable token cost.
- A structured value can be type-correct while semantically ambiguous. Stable field names and bounded values remain an application responsibility.

## Task recipes

- **Encode a Gimble supervision snapshot:** use a plain JSON object with explicit fields for run status, current node, recent events, evidence state, and allowed actions; exclude live handles and cyclic runtime objects. Start at `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/type-aliases/EntryType.md:7-17`.
- **Carry structured policy metadata:** put short machine-readable fields inside instruction/criterion objects when plain text loses important distinctions, keeping every nested value within `JsonValue`. Start at `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/type-aliases/JsonValue.md:7-17`.
- **Validate at the boundary:** serialize representative snapshots during tests and reject unsupported JavaScript values before constructing the API request.

