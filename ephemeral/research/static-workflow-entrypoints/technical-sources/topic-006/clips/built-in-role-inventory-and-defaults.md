# Built-In Role Inventory and Defaults

## Complete Role Mapping Across Built-In Workflows

Across the five stock built-in workflows in Gimbal, 15 unique model roles are defined:

| Workflow | Role Name | Constant / Identifier | Default in `cmd/gimbal/defaults.json` | Provider / Harness | Default Effort |
| --- | --- | --- | --- | --- | --- |
| `review` | `code-review` | `gimbal.RoleCodeReview` | `"gpt-6-luna"` | OpenAI / `codex` | implicit high |
| `validate-product` | `product-operation` | `gimbal.WorkflowRole("product-operation")` | `"claude-opus-5-5:high"` | Anthropic / `claude` | high |
| `validate-product` | `product-visual-review` | `gimbal.WorkflowRole("product-visual-review")` | `"gemini-3.8-flash-medium"` | Gemini / `agy` | medium |
| `validate-product` | `product-triage` | `gimbal.WorkflowRole("product-triage")` | `"gpt-6-astra:high"` | OpenAI / `codex` | high |
| `implement` | `sprint-planning` | `gimbal.WorkflowRole("sprint-planning")` | `"gpt-6-astra:high"` | OpenAI / `codex` | high |
| `implement` | `architectural-critique` | `gimbal.WorkflowRole("architectural-critique")` | `"gpt-6-astra:medium"` | OpenAI / `codex` | medium |
| `implement` | `coding` | `roleCoding` (`gimbal.WorkflowRole("coding")`) | `"gpt-6-sol:high"` | OpenAI / `codex` | high |
| `implement` | `qa-orchestration` | `gimbal.WorkflowRole("qa-orchestration")` | `"gpt-6-sol:high"` | OpenAI / `codex` | high |
| `research-document` | `research-planning` | `roleResearchPlanning` | `"gemini-3.8-flash-medium"` | Gemini / `agy` | medium |
| `research-document` | `research-indexing` | `roleResearchIndexing` | `"gemini-3.8-flash-medium"` | Gemini / `agy` | medium |
| `research-document` | `index-curation` | `roleIndexCuration` | `"gemini-3.8-flash-medium"` | Gemini / `agy` | medium |
| `research-document` | `document-authoring` | `roleDocumentAuthoring` | `"gemini-3.1-pro-high"` | Gemini / `agy` | high |
| `research-document` | `editorial-review` | `roleEditorialReview` | `"gemini-3.1-pro-high"` | Gemini / `agy` | high |
| `research-document` | `document-supervision` | `roleDocumentSupervision` | `"gemini-3.8-flash-medium"` | Gemini / `agy` | medium |
| `pyramid-summary` | `document-authoring` | `roleDocumentAuthoring` | `"gemini-3.1-pro-high"` | Gemini / `agy` | high |
| `pyramid-summary` | `editorial-review` | `roleEditorialReview` | `"gemini-3.1-pro-high"` | Gemini / `agy` | high |
| `pyramid-summary` | `document-supervision` | `roleDocumentSupervision` | `"gemini-3.8-flash-medium"` | Gemini / `agy` | medium |
| `pyramid-summary` | `pyramid-planning` | `rolePyramidPlanning` | `"gpt-6-luna"` | OpenAI / `codex` | implicit high |

All 15 roles defined in `cmd/gimbal/defaults.json` are accounted for by the five built-in workflows.
Note that `document-authoring`, `editorial-review`, and `document-supervision` are shared between `research-document` and `pyramid-summary`, each binding to the identical defaults.

## Structure of Defaults in `cmd/gimbal/defaults.json`

The JSON structure is a flat key-value object where keys are role names (matching `gimbal.WorkflowRole` values) and values are model spec strings:
- Bare model name: `"code-review": "gpt-6-luna"`
- Model name with explicit effort suffix: `"coding": "gpt-6-sol:high"`, `"architectural-critique": "gpt-6-astra:medium"`
- Provider-native model name with embedded effort: `"gemini-3.8-flash-medium"`

In `cmd/gimbal/workflows.go:17-26`, `defaults.json` is embedded via `//go:embed defaults.json` and decoded into `map[gimbal.WorkflowRole]string`.
