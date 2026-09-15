## Request

Support recursive Go type definitions throughout polytype's JSON Schema, Go JSON, TypeScript, and devalue generation.

Gimble issue [#201](https://github.com/tylergannon/gimble/issues/201) needs a nested workflow description: an ordered body contains operations, some operations contain further bodies, and a supervisor can have supervisors of its own. Operation/subgraph variants are sealed interfaces. These are finite tree values described by recursive Go types.

With Gimble's pinned polytype v1.0.0, the builder rejects recursion with `circular dependency found for type ...` (`internal/builder/gen_schema.go`, interface path around line 842 and named-type path around line 1015). This request is based on that pinned version; it does not claim a reproduced failure against a newer release.

## Small examples to support

```go
type Node struct {
    Name     string `json:"name"`
    Children []Node `json:"children"`
}

type Element interface { element() }

type Leaf struct { Text string `json:"text"` }
func (Leaf) element() {}

type Branch struct { Children []Element `json:"children"` }
func (Branch) element() {}
```

Register Element with the existing SealedUnion API. The same need arises with pointer/optional recursion and mutually recursive named types.

## Desired behavior

- The type grammar can represent named recursive references without infinitely expanding types.
- JSON Schema uses references/definitions for these shapes.
- Generated Go JSON and devalue codecs encode/decode finite nested values, including sealed-union discriminators at every level.
- Generated TypeScript preserves the recursive named types and discriminated unions.
- Validation and diagnostics remain meaningful for invalid nested values.
- Verify nested round trips, recursive unions, mutual recursion, and deterministic regeneration.

This concerns recursive type definitions. It does not require serializing cyclic in-memory pointer graphs or adding shared-object identity to JSON.

## Current workaround

Gimble will keep its recursive Go Graph model and handwrite the Graph JSON codec for now. We do not want to flatten the domain model into reference tables solely to satisfy code generation. The interim codec does not itself resolve the TypeScript/devalue generation boundary; native recursive support would let all projections use the same Go model.
