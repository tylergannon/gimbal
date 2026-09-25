# Workflow Role Model Schema and Server-Side Resolution

## Goal-Relevant Synthesis

In the current codebase, the CLI transmits an untyped map `Models map[gimbal.WorkflowRole]string` over the wire ([source](sources/gimbal-command-generation-and-submission.txt):23-26). The server resolves this map using `internal/binding.Roles(request.Models)` ([source](sources/gimbal-command-generation-and-submission.txt):78-82). Under the accepted static entrypoint architecture, start requests must replace the dynamic map with statically generated fields for the actual roles required by each workflow ([source](sources/gimbal-command-generation-and-submission.txt):96-103).

Across the five stock built-in workflows (`review`, `implement`, `validate-product`, `research-document`, `pyramid-summary`), exactly 15 roles are defined ([source](sources/gimbal-defaults-and-workflow-roles.txt):29-118). Their default model specifications are defined in `cmd/gimbal/defaults.json` ([source](sources/gimbal-defaults-and-workflow-roles.txt):7-27).

Role overrides on generated request structs use `polytype.Optional[string]` tagged with `json:",omitzero"`. When omitted on the wire (`Present == false`), the server falls back to the default from `cmd/gimbal/defaults.json`. When explicitly supplied (`Present == true`), the server validates and binds the override. All model resolution, alias expansion, and provider harness validation execute during admission *before* `p.Run` is invoked, guaranteeing that rejected models create no run. Detailed analyses are in [built-in-role-inventory-and-defaults.md](clips/built-in-role-inventory-and-defaults.md) and [server-side-model-resolution.md](clips/server-side-model-resolution.md).

## Answers to Assigned Questions

### 1. What model roles are declared across the five built-in workflows, and how are their default values structured in cmd/gimbal/defaults.json?
- **Declared roles across the five built-ins**:
  - `review`: `code-review` (`gpt-6-luna`)
  - `validate-product`: `product-operation` (`claude-opus-5-5:high`), `product-visual-review` (`gemini-3.8-flash-medium`), `product-triage` (`gpt-6-astra:high`)
  - `implement`: `sprint-planning` (`gpt-6-astra:high`), `architectural-critique` (`gpt-6-astra:medium`), `coding` (`gpt-6-sol:high`), `qa-orchestration` (`gpt-6-sol:high`)
  - `research-document`: `research-planning` (`gemini-3.8-flash-medium`), `research-indexing` (`gemini-3.8-flash-medium`), `index-curation` (`gemini-3.8-flash-medium`), `document-authoring` (`gemini-3.1-pro-high`), `editorial-review` (`gemini-3.1-pro-high`), `document-supervision` (`gemini-3.8-flash-medium`)
  - `pyramid-summary`: `document-authoring` (`gemini-3.1-pro-high`), `editorial-review` (`gemini-3.1-pro-high`), `document-supervision` (`gemini-3.8-flash-medium`), `pyramid-planning` (`gpt-6-luna`)
  - (Total: exactly 15 unique roles, matching `cmd/gimbal/defaults.json` lines 10-26).
- **Structure of defaults in `cmd/gimbal/defaults.json`**:
  - A single flat JSON object mapping string role keys to model spec strings ([source](sources/gimbal-defaults-and-workflow-roles.txt):7-27).
  - Format of spec strings: either bare model name (`"gpt-6-luna"`), model with explicit effort (`"claude-opus-5-5:high"`, `"gpt-6-astra:medium"`), or provider-native name with embedded effort (`"gemini-3.8-flash-medium"`).

### 2. How should model role override fields be defined on generated request structs to distinguish default inheritance from explicit overrides?
- **Struct definition**:
  Each generated workflow start request struct declares a dedicated field for each role required by its workflow, using `polytype.Optional[string]` tagged with `json:",omitzero"`:
  ```go
  RoleCoding polytype.Optional[string] `json:"role_coding,omitzero"`
  ```
- **Distinguishing inheritance from override**:
  - **Inheritance (default)**: When omitted from the request (in CLI or browser form), `RoleCoding.Present` is `false`. The server takes this as an explicit directive to load the default model spec from `cmd/gimbal/defaults.json`.
  - **Explicit override**: When the field is provided on the wire, `RoleCoding.Present` is `true`. The server uses `RoleCoding.Value` as the override spec.
  - **Preserving zero values**: If the user submits an empty string or invalid model, `RoleCoding.Present` is `true` with `Value: ""`. The server attempts to resolve it and returns a validation error, rather than silently falling back to the default.

### 3. How does the server resolve and validate model overrides against provider configurations in the startup environment prior to run creation?
- **Resolution mechanism**:
  - The server handler collects effective role specs into a `map[gimbal.WorkflowRole]string` combining defaults and explicit overrides.
  - It invokes `internal/binding.Roles(specs)` ([source](sources/gimbal-model-resolution-and-binding.txt):58-89).
  - `binding.Roles` calls `internal/modelalias.Resolve(selection)` for each spec ([source](sources/gimbal-model-resolution-and-binding.txt):130-203):
    - Parses name and effort.
    - Resolves alias to native model ID and provider.
    - Selects the execution harness (`agy`, `claude`, `codex`, `opencode`).
  - `binding.Roles` calls `binding.Adapter(resolved.Harness)` to instantiate the provider adapter (`agy.New()`, `claude.New()`, `codex.New()`, `opencode.New()`).
- **Validation prior to run creation**:
  - In `web/control.go:114-118`, `binding.Roles` runs before `started <- id` or `p.Run(...)` ([source](sources/gimbal-command-generation-and-submission.txt):78-82).
  - If any model name is unblank, unsupported, has an invalid effort, or conflicts with fixed effort, `modelalias.Resolve` returns an error.
  - If an unknown harness is encountered, `binding.Adapter` returns an error.
  - The handler halts immediately, returning an HTTP 400 error (or `skgo.Invalid` form issue).
  - **No run ID is allocated, no store is registered, and no goroutine is launched** for an invalid model override.

## Evidence Boundary / Unresolved

- **Supported facts**:
  - Exactly 15 roles are declared across the 5 built-in workflows, matching `cmd/gimbal/defaults.json`.
  - `internal/binding.Roles` validates model aliases and instantiates harness adapters, failing fast on invalid model specifications.
  - `cmd/gimbal/workflows.go` embeds `defaults.json` and supplies it to `Command()`.
- **Inference**:
  - The start remote handler should embed `defaults.json` or take it as a dependency in the server package, ensuring the server remains the single authority on role defaults.
- **Unresolved questions**:
  - Should role fields on start forms be nested under a `models` struct (e.g. `models.coding`) or flat top-level fields (e.g. `role_coding`)? Flat top-level fields avoid unnecessary struct nesting, while a nested struct keeps model overrides isolated from workflow data parameters.

## Downloaded Primary Sources

- [Gimbal defaults and workflow roles source excerpts](sources/gimbal-defaults-and-workflow-roles.txt) — from `cmd/gimbal/defaults.json` and `internal/workflows/*`.
- [Gimbal model resolution and binding source excerpts](sources/gimbal-model-resolution-and-binding.txt) — from `internal/binding/binding.go` and `internal/modelalias/modelalias.go`.
- [Gimbal command generation and submission source excerpts](sources/gimbal-command-generation-and-submission.txt) — from `internal/generate/command.go`, `web/control.go`, and `implementation-plan.md`.
