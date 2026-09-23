# Cross-Repository Sequencing, Module Pinning, and Delivery Workflow

- **Origin**: `/Users/tyler/.codex/worktrees/d798/gimble`
- **Files**:
  - `skills/gimble-release/SKILL.md`
  - `go.mod`
  - `ephemeral/research/static-workflow-entrypoints/implementation-plan.md`
- **Commit/Baseline**: `main` / `e161721c`
- **Retrieval Date**: 2026-09-23

---

## 1. Pinned Dependency Structure in `go.mod`

From `go.mod` (lines 28, 77–86):

```go
require (
    ...
    github.com/tylergannon/skgo v0.5.0
    ...
)

tool (
    github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen
    github.com/tylergannon/polytype/polytype
    // skgo generates the bindings, and projects the Go types that cross to
    // TypeScript through polytype's library. It runs through `go tool`, so it is
    // built from the module cache and does not have to be a writable checkout.
    github.com/tylergannon/skgo/cmd/skgo
    golang.org/x/tools/cmd/goimports
    golang.org/x/tools/go/analysis/passes/modernize/cmd/modernize
)
```

---

## 2. Cross-Repository Sequencing Rules

From `ephemeral/research/static-workflow-entrypoints/implementation-plan.md` (lines 74–107, 183–203):

```markdown
Implement this in an isolated SKGO worktree, following that repository's
instructions. Gimble currently pins SKGO v0.5.0; inspect the implementation
checkout before changing it rather than assuming the pinned source is latest.

SKGO should generate a typed Go call for each selected remote Form using the
same declaration, endpoint identity, input type, and result type as its server
and browser bindings. Shared transport code in SKGO owns the enhanced-form
envelope and response decoding. Generated Gimble commands must not contain
private copies of the SvelteKit wire protocol.
...
Result: a generated Go client and a real browser form can call one SKGO Form
and obtain equivalent admission results and field errors. Land and publish
the needed SKGO change, then pin that version in Gimble; leave no local module
replacement in the delivered build.
...
Generation succeeds from a clean checkout with the pinned released dependencies,
refreshes stale output, and produces no second-run diff. A changed input/result
contract updates both client bindings; incompatible usages are reported by
generation or compilation. No hand-edited generated output or local module
replacement is required.
```

---

## 3. Merge, Installation, and Clean Build Standards

From `skills/gimble-release/SKILL.md` (lines 39–56, 107–125):

```markdown
## Build the application

```sh
just build
```

The current recipe prepares Go and frontend dependencies, runs generation,
formats generated frontend output, builds Svelte, checks that
`web/build/skgo.manifest.json` exists, packages `web/build.zip`, and builds
`bin/gimble` with the web application embedded. Run `go install ./cmd/gimble`
only after `just build` in the same checkout. Since v0.12.1, versioned
`go install` uses the committed archive without rebuilding the frontend.

...
Before tagging a release, check that `go.mod` has no replacement that blocks
versioned installation. Install from the pushed commit with
`go install github.com/tylergannon/gimble/cmd/gimble@<commit>` and start its web
listener; confirm it serves the page. After tagging, repeat with the exact tag.
A successful install without a served page does not prove the published CLI works.

## Merge and refresh the local installation

Commit and push meaningful work; follow repository review and squash-merge
practice within the user's authorized scope. Retain the task worktree when the
user has asked to keep it. Every release must update `CHANGELOG.md`...
After every feature or fix merges, reinstall both the CLI and the Gimble skills
on this machine:

1. Fast-forward the main checkout and build the merged source with `just build`.
2. Install it with `go install ./cmd/gimble` from that built checkout.
3. Reinstall the published skills...
```
