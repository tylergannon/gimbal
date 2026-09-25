# Scalar and Optional Binding in SKGO Form Decoding

## Goal-Relevant Synthesis

Form functions in SKGO are the only remote kind whose input argument bypasses Polytype's generated devalue codecs ([source](sources/skgo-form-decoding-source.txt):104-133). While queries and commands receive devalue payloads decoded by Polytype's generated strict decoders ([source](sources/skgo-form-decoding-source.txt):146-149), forms post `application/x-sveltekit-formdata` which SKGO decodes using reflection in `skgo.DecodeForm` and `internal/formdata/decode.go` ([source](sources/skgo-form-decoding-source.txt):84-97, 173-228).

Because `formdata.Decode` only recognizes primitive Go kinds (`reflect.String`, `reflect.Int*`, `reflect.Bool`, `reflect.Slice`, `*devalue.Object`), it fails when assigning scalars to `polytype.Optional[T]`, which is a Go struct ([source](sources/polytype-optional-and-codecs-source.txt):14-22). To support Gimbal's workflow inputs, `formdata.Decode` must recognize `polytype.Optional[T]`, unpack its presence, and delegate scalar decoding.

Omitted form fields and explicitly supplied zero, false, or empty string values are distinguished cleanly by key presence in the parsed `*devalue.Object` ([source](sources/skgo-form-decoding-source.txt):280-314). Omitted fields never trigger assignment, preserving `Present: false`. Explicitly supplied zero values trigger assignment, populating `Value` and setting `Present: true`, which prevents `json:",omitzero"` from omitting them ([source](sources/polytype-optional-and-codecs-source.txt):20-22). Detailed mechanics are detailed in [form-scalar-optional-delegation.md](clips/form-scalar-optional-delegation.md) and [nested-structs-parameter-shapes.md](clips/nested-structs-parameter-shapes.md).

## Answers to Assigned Questions

### 1. Where does SKGO form decoding bypass Polytype codecs, and how must it be modified to delegate scalar values to Polytype?
- **Where bypass occurs**:
  - `internal/gen/codecs.go:92-94`: `planCodecs()` explicitly excludes `kindForm` from receiving an `inCodec` ([source](sources/skgo-form-decoding-source.txt):120-123).
  - `internal/gen/emit.go:594-596`: `writeArgument()` emits `skgo.DecodeForm(call.Arg, &in)` instead of `Decode<Root>(call.Arg)` ([source](sources/skgo-form-decoding-source.txt):141-145).
  - `remote.go:429-434`: `DecodeForm` delegates directly to `formdata.Decode(arg, into)` ([source](sources/skgo-form-decoding-source.txt):92-97).
  - `internal/formdata/decode.go:68-86`: `assign()` switches on primitive types and assigns directly via reflection, with zero invocation of Polytype codecs or types ([source](sources/skgo-form-decoding-source.txt):210-228).
- **Modification required**:
  - In `internal/formdata/decode.go`, update `assign()` to inspect if destination `dst.Kind() == reflect.Struct` matches `polytype.Optional[T]`.
  - When `Optional[T]` is detected: ignore `Undefined` or `Hole` nodes (leaving `Present: false`); otherwise mark `dst.FieldByName("Present").SetBool(true)` and recursively call `assign()` on `dst.FieldByName("Value")`.
  - For scalar validations (bounds, integers, booleans), enforce range checking matching Polytype's `dvInteger`, `dvFloat32`, and `dvBool` ([source](sources/polytype-optional-and-codecs-source.txt):174-213).

### 2. How can form decoding distinguish omitted form fields from explicitly supplied zero, false, or empty string values when populating Optional[T] fields tagged with json:",omitzero"?
- **Key absence vs presence**: `formdata.Decode` assigns struct fields by looping over `obj.Keys()` ([source](sources/skgo-form-decoding-source.txt):283-294). If a field was omitted in the submission, the key is absent from `obj.Keys()`; `assign` is never invoked, leaving `Optional[T].Present = false`.
- **Explicit zero/false/empty**: When the client supplies `0`, `false`, or `""`, the key is in `obj.Keys()`. `assign` executes, sets `Present = true`, and sets `Value` to the scalar zero value.
- **Preserving omitzero**: `Optional[T].IsZero()` reports `!o.Present` ([source](sources/polytype-optional-and-codecs-source.txt):20-22). When `Present == true`, `IsZero()` returns `false`, ensuring that explicitly supplied zero/false/empty values are retained during JSON serialization and not omitted.
- **Empty text inputs for number controls**: Unset HTML number inputs send `""`, which SvelteKit's `coerce_form_value` ([source](sources/sveltekit-form-utils-reference.txt):68-73) and SKGO's `coerce` convert to `devalue.Undefined`. `decode.go:40` skips `devalue.Undefined`, leaving `Present == false` ([source](sources/skgo-form-decoding-source.txt):187-189).

### 3. How does Polytype handle nested struct field paths in standard form data submissions, and what conventions match Gimbal's workflow parameter shapes?
- **Nested paths in SvelteKit/SKGO**: Controls named with dotted paths (e.g. `params.goal`) are split by `split_path` and converted into nested `*devalue.Object` trees by `setNested` ([source](sources/sveltekit-form-utils-reference.txt):16-18, 83-113).
- **Struct decoding**: `assignObject` maps nested `*devalue.Object` values to matching struct fields by recursively calling `assign` on the nested struct field ([source](sources/skgo-form-decoding-source.txt):289-292). In Polytype's generated codecs, nested structs are decoded in place via `decodeObjectInto` ([source](sources/polytype-optional-and-codecs-source.txt):96-118).
- **Gimbal parameter conventions**: Across all five built-ins (`review`, `implement`, `validate-product`, `research-document`, `pyramid-summary`), all workflow parameters are flat scalar types (`string`, `int`, `polytype.Optional[int]`). Either flat request struct fields (`project`, `work_dir`, `conversation`, `goal`, `role_<name>`) or a single nested struct `params` (`params.goal`, `params.token_budget`) perfectly align with Polytype and SKGO conventions without requiring map or custom codec machinery.

## Evidence Boundary / Unresolved

- **Supported facts**:
  - SKGO `formdata.Decode` does not support `polytype.Optional[T]` today; passing an `Optional[T]` field causes a runtime type error during form decode.
  - Polytype requires `json:",omitzero"` on all `Optional[T]` fields (`internal/builder/typegrammar.go:726`).
  - SvelteKit converts empty numeric inputs to `undefined`, which is distinct on the wire from an explicit numeric `0`.
- **Inference**:
  - Updating `formdata.Decode` via reflection is cleaner and less invasive than generating full devalue decoders for forms, because forms can theoretically contain `File` inputs (though Gimbal workflows do not).
- **Unresolved questions**:
  - Should workflow request structs keep parameters in an embedded `Params` struct (`params.goal`) or flatten all parameters to top-level fields alongside `project` and `work_dir`? Both work with SvelteKit's `split_path`, but flattening simplifies CLI flag mapping.

## Downloaded Primary Sources

- [SKGO form decoding and codec bypass source excerpts](sources/skgo-form-decoding-source.txt) — from `/Users/tyler/.codex/worktrees/d798/skgo` (`fed929b`).
- [Polytype Optional and codec source excerpts](sources/polytype-optional-and-codecs-source.txt) — from `github.com/tylergannon/polytype@v1.0.3` and `v1.1.0`.
- [SvelteKit form-utils wire format and coercion reference](sources/sveltekit-form-utils-reference.txt) — from `/Users/tyler/src/skgo/ephemeral/inspiration/reference/kit@3.0.0-next.27/src/runtime/form-utils.js`.
