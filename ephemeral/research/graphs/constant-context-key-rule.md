# Hard lint rule: constant context keys

Current decision from Tyler, 2026-09-14. Supersedes earlier recommendations to permit dynamic Set keys in lint-clean workflows. Workflows should be explicit enough that reading their source reveals what they do. The linter enforces this authoring convention; Go compilation and runtime behavior remain unchanged.

## Rule

Every call to `gimble.Set` or `gimble.SetJSON` must use a compile-time constant string key. String literals, named Go constants, and constant expressions pass. Formatted strings, runtime concatenation, variables, and function-returned keys fail. Values remain dynamic.

Proposed stable diagnostic:

`[GIMBLE102-SIMPLE-WORKFLOWS/CONSTANT-CONTEXT-KEY]: Context keys must be compile-time constants so a workflow's recorded fields are explicit in its source. Use a named constant or string literal; keep changing data in the value.`

Resolve the actual Gimble symbols with Go type information. Inspect the key expression's compile-time value using `go/types`; do not use pointer analysis, execute code, or treat a variable initialized with a literal as a Go constant. Apply to both Set and SetJSON. Do not skip a call merely because its key is nonconstant: that is exactly the diagnostic.

## Real negative acceptance case

The current builtin sprint must fail lint at `internal/workflows/sprint/sprints.go:257`:

```go
gimble.Set(ctx, fmt.Sprintf("repository check %d", i+1), commandText(check, code, output))
```

It loops over the fixed commands `go vet ./...` and `go test ./...` and invents numbered field names. No workflow requirement necessitates those dynamic keys. The straightforward later rewrite executes the checks explicitly and records them immediately under constant keys such as `"repository vet"` and `"repository tests"`. This preserves earlier evidence if the second check aborts; an aggregate-after-the-loop rewrite is not required.

Do not rewrite or exempt the builtin to conceal this acceptance failure. Demonstrate the current source failing with the named diagnostic. A subsequent constant-key workflow change can make it pass. The existing task-specific command already records dynamic command content under the constant key `"task command"` and should pass this rule.

## Fixtures

- Fail the current sprint expression and an equivalent SetJSON call with a runtime key.
- Fail a key read from input, a formatted key, a variable initialized with a literal, and a function-returned key.
- Pass a literal, named constant, and constant concatenation.
- Pass dynamic values, command arguments, task content, and model selection when the context key is constant.
- Prove the negative example still compiles/runs without the analyzer; the linter exits nonzero with the diagnostic at the key expression/callsite.
- Keep the separate duplicate-key rule: two writes to one constant key on the same scope are still misuse. Changing a dynamic key to one repeated constant does not make the original loop valid.

Tracked in Beta linter #162; informs graph extraction #201. This is an accepted requirement, not a proposed candidate.
