# Server-Side Model Resolution and Override Schema

## Request Struct Definition for Role Overrides

Currently, the CLI sends an untyped `Models map[gimbal.WorkflowRole]string` on the wire (`internal/generate/command.go:209`).
Under the accepted architecture (`implementation-plan.md:115-126`):
1. **Explicit role fields on generated request structs**:
   Instead of transmitting a dynamic map, the request struct generated for each workflow declares concrete fields for each role defined in that workflow's graph.
   For example, for `review`:
   ```go
   type StartReviewRequest struct {
       Project        string                   `json:"project"`
       WorkDir        string                   `json:"work_dir"`
       Conversation   polytype.Optional[string]`json:"conversation,omitzero"`
       RoleCodeReview polytype.Optional[string]`json:"role_code_review,omitzero"`
       Params         ReviewParams             `json:"params"`
   }
   ```
   Or for `implement`:
   ```go
   type StartImplementRequest struct {
       Project                   string                   `json:"project"`
       WorkDir                   string                   `json:"work_dir"`
       Conversation              polytype.Optional[string]`json:"conversation,omitzero"`
       RoleSprintPlanning        polytype.Optional[string]`json:"role_sprint_planning,omitzero"`
       RoleArchitecturalCritique polytype.Optional[string]`json:"role_architectural_critique,omitzero"`
       RoleCoding                polytype.Optional[string]`json:"role_coding,omitzero"`
       RoleQAOrchestration       polytype.Optional[string]`json:"role_qa_orchestration,omitzero"`
       Params                    Params                   `json:"params"`
   }
   ```
2. **Distinguishing default inheritance from explicit overrides**:
   - When a caller (CLI or browser form) does not specify an override for a role, the field is omitted from the request payload.
   - On the server, `req.RoleCoding.Present` is `false`. The server falls back to `defaults[gimbal.WorkflowRole("coding")]` loaded from `defaults.json`.
   - When an override is explicitly specified (even if equal to the default string, or setting a specific effort/model), `req.RoleCoding.Present` is `true`. The server uses `req.RoleCoding.Value` as the override.
   - If an empty string or invalid override is explicitly supplied, `req.RoleCoding.Present` is `true` with `Value: ""`. The server-side validation catches this and rejects the request before run creation.

## Server-Side Resolution and Validation Pipeline

Before any workflow run is created or registered, the start remote handler must resolve and validate all models:

1. **Resolution pipeline**:
   For each role required by the concrete workflow:
   - Determine model spec string: if `req.Role<X>.Present`, use `req.Role<X>.Value`; else use `defaults[role]`.
   - Call `internal/binding.Roles(specs)`:
     - Splits `spec` on `:` into `name` and optional `effort`.
     - Calls `modelalias.Resolve(selection)`:
       - Checks for non-blank name and valid effort (`low`, `medium`, `high`, `xhigh`, `max`).
       - Resolves alias to native model ID and provider (`openai`, `anthropic`, `gemini`, `opencode`).
       - Resolves harness (`codex`, `claude`, `agy`, `opencode`).
     - Calls `binding.Adapter(resolved.Harness)` to instantiate the adapter (`codex.New()`, `claude.New()`, `agy.New()`, `opencode.New()`).
     - Constructs `gimbal.ModelBinding{Adapter: adapter, Model: resolved.Model, Effort: resolved.Effort}`.

2. **Admission-time failure handling**:
   - If any role override is unrecognized, has an invalid effort, or conflicts with native fixed efforts (e.g. `flash` with incompatible effort), `binding.Roles` returns a formatted error naming the role.
   - If any provider harness cannot be initialized, an error is returned.
   - The start remote returns an HTTP 400 Bad Request (or SvelteKit `skgo.Invalid` issue) with the failure diagnostic.
   - Because this validation occurs before `p.Run(...)` or `started <- id`, **no run is ever created or published** for an invalid model configuration.
