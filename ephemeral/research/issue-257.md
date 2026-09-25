# Issue 257: Generate workflow role reference from roles.go

## Finished state

Running `go generate ./...` extracts every exported `WorkflowRole` constant from root `roles.go` into a generated role reference in the documentation site. Each entry shows the Go constant, serialized role value, and its Go doc description. `roles.go` remains the sole source of truth.

This follows PR #246, which introduces the typed catalog and keeps each description between 10 and 20 words.

## Done when

- The generated reference includes every exported `WorkflowRole` constant in declaration order.
- Each entry contains the constant name, its string value, and its description from `roles.go`.
- Generation fails clearly when a role is missing a description or its description is outside the 10–20-word range.
- Adding, renaming, or removing a role updates the generated documentation deterministically.
- Running `go generate ./...` twice leaves the worktree clean.
- Generated role documentation is labeled as generated and is never maintained by hand.

## Bounded design

`roles.go` remains the only authored role catalog. Generation parses that file's Go syntax and documentation, selects exported constants whose declared type is `WorkflowRole`, and preserves source declaration order. For each role, it resolves the constant string value and normalizes the Go doc into the human-facing description. Generation fails with a precise source-positioned error if a role has no description or its description is outside the required 10–20-word range; malformed entries must not be silently omitted.

The output is a generated documentation-site page or data artifact using the site's existing composition and styling. It is clearly labeled generated and presents exactly the Go constant name, serialized role value, and description. The generator owns the complete output; there is no hand-maintained list or second source of truth. Focused tests cover ordering, extraction, values, missing/short/long descriptions, add/rename/remove behavior, and deterministic output. Proof is focused tests, two consecutive `go generate ./...` runs with a clean second run, the repository's normal docs/application build, and inspection of the rendered reference as a caller.

Source: https://github.com/tylergannon/gimbal/issues/257
