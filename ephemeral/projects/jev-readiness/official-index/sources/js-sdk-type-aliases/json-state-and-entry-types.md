# JSON, state, instruction, and description types

## Purpose

This leaf captures the JavaScript SDK’s serializable value grammar and the narrower top-level entry shape used for state, instructions, and criterion descriptions.

## Key concepts

- **`JsonValue` is the recursive JSON value union.** It permits strings, numbers, booleans, `null`, arrays of JSON values, and string-keyed objects whose values are JSON values. [JsonValue](https://docs.typesafe.ai/sdk/javascript/api/type-aliases/JsonValue.md)
- **`EntryType` is narrower at the top level.** State, instructions, and criteria may be a string, a JSON object, a JSON array, or `null`; a bare number or boolean is not a documented top-level `EntryType`, even though either may appear inside an object or array as `JsonValue`. [EntryType](https://docs.typesafe.ai/sdk/javascript/api/type-aliases/EntryType.md)
- **Structured entries remain fully JSON-compatible.** Objects use arbitrary string keys and recursively JSON-compatible values; arrays likewise contain `JsonValue`. [EntryType](https://docs.typesafe.ai/sdk/javascript/api/type-aliases/EntryType.md) [JsonValue](https://docs.typesafe.ai/sdk/javascript/api/type-aliases/JsonValue.md)
- **`Description` is exactly `EntryType`.** A criterion description can therefore be text, structured JSON object/array, or `null`; `null` deliberately leaves the label undescribed. [Description](https://docs.typesafe.ai/sdk/javascript/api/type-aliases/Description.md)

## Citation bookmarks

- Full JSON-compatible grammar: [JsonValue](https://docs.typesafe.ai/sdk/javascript/api/type-aliases/JsonValue.md)
- Top-level entry grammar and intended uses: [EntryType](https://docs.typesafe.ai/sdk/javascript/api/type-aliases/EntryType.md)
- Description alias and `null` semantics: [Description](https://docs.typesafe.ai/sdk/javascript/api/type-aliases/Description.md)

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

- **Encode a Gimble supervision snapshot:** use a plain JSON object with explicit fields for run status, current node, recent events, evidence state, and allowed actions; exclude live handles and cyclic runtime objects. Start at [EntryType](https://docs.typesafe.ai/sdk/javascript/api/type-aliases/EntryType.md).
- **Carry structured policy metadata:** put short machine-readable fields inside instruction/criterion objects when plain text loses important distinctions, keeping every nested value within `JsonValue`. Start at [JsonValue](https://docs.typesafe.ai/sdk/javascript/api/type-aliases/JsonValue.md).
- **Validate at the boundary:** serialize representative snapshots during tests and reject unsupported JavaScript values before constructing the API request.

