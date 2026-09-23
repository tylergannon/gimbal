# Form Scalar and Optional Delegation

## Problem Analysis

SKGO form decoding in `internal/formdata/decode.go` and `remote_form.go` was designed around an assumption stated in `remote_form.go:7-11`:
> "a form cannot reuse the generated devalue decoder every other kind uses: a File is not a JSON value and polytype describes none, so the generated closure assigns the submission onto the handler's own argument type with skgo.DecodeForm."

Because of this:
1. In `internal/gen/codecs.go:92-94`, `planCodecs()` explicitly excludes forms from receiving an input codec:
   ```go
   if fn.in != nil && fn.kind != kindForm {
       fn.inCodec = a.codecSet.root(fn.in, fn.goPkg.loadDir, fn.pos)
   }
   ```
2. In `internal/gen/emit.go:594-596`, the generated code routes to `skgo.DecodeForm(call.Arg, &in)`:
   ```go
   var in InType
   if err := skgo.DecodeForm(call.Arg, &in); err != nil { ... }
   ```
3. `skgo.DecodeForm` invokes `formdata.Decode(arg, into)`.
4. In `internal/formdata/decode.go:68-86`, `assign` uses reflection over primitive Go kinds (`reflect.String`, `reflect.Int*`, `reflect.Bool`, `reflect.Slice`, `*devalue.Object`).

### The Optional[T] Binding Gap

In Go, `polytype.Optional[T]` is a struct:
```go
type Optional[T any] struct {
    Present bool
    Value   T
}
```
When `formdata.Decode` encounters an `Optional[T]` field on a form request struct (such as `MinSourcesPerTopic polytype.Optional[int]` in `researchdocument.Params` or model override fields), `dst.Kind()` is `reflect.Struct`.
- When assigning a text string, `assignString` executes its default branch (`default: return typeError(path, "text", dst.Type())`).
- When assigning a numeric value, `assignNumber` executes its default branch (`default: return typeError(path, "a number", dst.Type())`).
- When assigning a boolean, `assign` checks `if dst.Kind() != reflect.Bool` and fails.
`formdata.Decode` is completely unable to assign scalar values into `polytype.Optional[T]`.

## Distinguishing Omitted Fields from Explicit Zero, False, and Empty Values

SvelteKit and SKGO have distinct representations for:
1. **Omitted field**:
   In SvelteKit enhanced forms, if an optional input is not included in the form or is an unchecked checkbox without a fallback, the property key does not exist on the payload POJO.
   In `internal/formdata/decode.go:181-192`, `assignObject` iterates exclusively over `obj.Keys()`:
   ```go
   for _, key := range obj.Keys() {
       index, ok := fields[strings.ToLower(key)]
       ...
       assign(value, dst.FieldByIndex(index), join(path, key))
   }
   ```
   If a property key is absent from `obj.Keys()`, the corresponding struct field in Go is untouched. For `polytype.Optional[T]`, its zero value has `Present: false` and `Value: zero`.
   Polytype's `IsZero()` method returns `!o.Present`, which is `true`. When marshaled or evaluated, Go's `json:",omitzero"` omits it.

2. **Explicitly supplied zero, false, or empty string**:
   When the client explicitly supplies a value:
   - Empty string `""`: The key exists in `obj.Keys()`, and its value is string `""`.
   - Explicit zero `0`: The key exists in `obj.Keys()`, and its value is `float64(0)` (or string `"0"`).
   - Explicit boolean `false`: The key exists in `obj.Keys()`, and its value is boolean `false`.
   Because the key exists in `obj.Keys()`, `assignObject` processes it.

3. **HTML form text empty inputs for numeric fields**:
   When submitting an empty `<input type="number">`, the browser sends `name=""`. SvelteKit's `coerce_form_value` (`form-utils.js:70`) and SKGO's `coerce` (`convert.go:144`) convert empty text to `devalue.Undefined`.
   In `decode.go:40-42`:
   ```go
   if _, undefined := node.(devalue.UndefinedValue); undefined {
       return nil
   }
   ```
   An `undefined` value returns `nil` without assigning or setting `Present: true`.

## Delegation and Modification Strategy

To fix this gap while maintaining compatibility with the existing architecture:
1. **In `internal/formdata/decode.go`**:
   Teach `assign` to recognize `polytype.Optional[T]`:
   When `dst.Kind() == reflect.Struct`:
   Inspect if `dst.Type()` represents `polytype.Optional[T]` (by verifying `dst.Type().PkgPath() == "github.com/tylergannon/polytype"` and `dst.Type().Name() == "Optional"`, or checking for exported `Present bool` and `Value` fields).
   When matched:
   - Check if `node` is `devalue.UndefinedValue` or `devalue.HoleValue` (which indicate absence); if so, return `nil` leaving `Present: false`.
   - Otherwise, set `dst.FieldByName("Present").SetBool(true)`.
   - Recurse into the inner value: `assign(node, dst.FieldByName("Value"), path)`.
2. **Delegating scalar validation to Polytype**:
   Polytype already implements strict scalar boundary checks in `devalue/codegen/codec.go.tmpl` (`dvInteger`, `dvFloat32`, `dvNumber`, `dvString`, `dvBool`).
   For integer targets, `assignNumber` must reject non-integers, infinities, and out-of-range floats matching `dvInteger`.
   For `Optional[T]`, setting `Present: true` and recursing ensures that `Optional[string]`, `Optional[int]`, and `Optional[bool]` accept their respective scalar values (including zero values) identically to Polytype's generated `Decode<Root>` codecs.
