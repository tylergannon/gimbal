# Historical operation sealed-union projection check

**Superseded model:** this verifies the flat 17-variant Graph from commit
`77be2b7`, not the accepted nested model in `graph-model.md`. It provides no
proof of recursive body or supervisor encoding. Retained as historical evidence.

The then-proposed Graph and schema declarations were extracted from
implementation-plan.md into a temporary caller module. No production graph
extractor or web binding implementation was added.

- Toolchain: Go 1.27.1.
- Dependency: github.com/tylergannon/polytype v1.0.0.
- Graph declaration SHA-256: `b74ae9f6a27aff03e270083bf9ddb3299b676ac0828fb9a96b0579c0040f1ac2`.
- Polytype JSON Schema, validation, owner JSON codecs, and TypeScript generation: exit 0.
- Polytype grammar/devalue codec generation: exit 0.
- JSON encode/validate/decode: all 17 concrete operation variants retain their type and fields.
- Devalue stringify/parse: all 17 concrete operation variants retain their type and fields.
- Embedded Site fields are projected into each variant, including id and scope.
- Unknown discriminator: rejected by schema validation and the strict devalue decoder.
- Field from another variant: rejected by schema validation and the strict devalue decoder.
- Generated TypeScript contains all 17 snake-case kind discriminators; output inspected.
- Second generation: no changes to generated schema, JSON codecs, TypeScript, or devalue codecs.

The executable Go probe passed:

```text
=== RUN   TestOperationUnion
=== RUN   TestOperationUnion/unknown_discriminator
=== RUN   TestOperationUnion/wrong_variant_field
--- PASS: TestOperationUnion
    --- PASS: TestOperationUnion/unknown_discriminator
    --- PASS: TestOperationUnion/wrong_variant_field
PASS
```

TypeScript was generated and inspected; this check did not compile or execute
TypeScript or exercise the running skgo application. The full graph's referential
semantics were not tested here; this probe checks the proposed type projection.
