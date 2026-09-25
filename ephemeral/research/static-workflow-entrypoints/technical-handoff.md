# Technical Handoff: Static Workflow Entrypoints

## 1. Package Architecture and Static Generation Pipeline

### Facts and Primary Sources
- The `Instance`, `Runtime`, and `AdmitProject` functions currently reside in `web/runtime.go` ([web/runtime.go:34-70, 222-283](technical-sources/topic-001/sources/web-runtime.go.txt)).
- Context helpers for projects are located in `web/src/project.go` ([web/src/project.go:5-35](technical-sources/topic-001/sources/web-src-project.go.txt)).
- The `workflow_gen.go` file currently imports `web` to call `web.Submit` ([internal/generate/command.go:111, 207](technical-sources/topic-002/sources/internal-generate-command.go.txt)).

### Recommended Implementation
- **Relocation of Ownership:** Move the `Instance`, `Runtime`, active run tracking (`activeRuns`), and `AdmitProject` logic from `web/runtime.go` to a new `internal/host` package. This breaks the cyclic dependency between workflows and the web layer.
- **Context Keys:** Relocate `ProjectChoice` and `ProjectDir` context helpers from `web/src/project.go` to `internal/host` so `web/src` is no longer a dependency for host ownership.
- **Generator Isolation:** Update `internal/generate/command.go` so workflow packages emit only the `Graph` definition. Remove `Hosted()`, `Command()`, and `web` imports from workflow packages.
- **Static Built-in Inventory:** Create a single source of truth for the five stock built-ins (`review`, `implement`, `research-document`, `pyramid-summary`, `validate-product`) in `internal/builtin/workflows.go`. Generators iterate over this slice to emit route handlers in `web/src/routes/` and CLI commands in `cmd/gimbal`.
- **Example Decoupling:** Decouple `interview` and `implementinterview` from the stock generator. Update `cmd/examples/` main programs to invoke `gimbal.Run(...)` directly as standalone runners.

### Remaining Uncertainties
- Whether `internal/host` should provide a synchronous `StartRun(ctx, name, models, body)` returning `(runID, err)` or rely on callers invoking `go p.Run(...)` with a channel, as currently done in `submit()` ([web/control.go:119-144](technical-sources/topic-008/sources/source-002-control-submission-async-run.md)).

## 2. SKGO Form Wire Protocol and Go Client Contract

### Facts and Primary Sources
- SKGO forms use a custom binary envelope (`application/x-sveltekit-formdata`) with a 7-byte binary prologue (version 0, LE u32 header length, LE u16 offset length) and a devalue `[data, meta]` header layout ([sveltekit-client-form-protocol.txt](technical-sources/topic-003/sources/sveltekit-client-form-protocol.txt)).
- Validation-only requests send `{"validate_only": true}` in the `meta` object. The SKGO handler detects `meta.ValidateOnly` and returns an empty issue list immediately without running the workflow ([skgo/remote_form.go:176-179](technical-sources/topic-003/sources/skgo-remote-form.txt)).
- SvelteKit uses HTTP 200 OK for both successful results and errors. Result schema: `{"type": "result", "data": "<devalue_string>"}` where data contains `{"_": {"submission": true, "result": <typed_value>}}`. Issue schema: `{"type": "result", "data": "<devalue_string>"}` where data contains `{"_": {"submission": true, "issues": [...]}}`, omitting `q`, `l`, and `r`. Server error schema: `{"type": "error", "error": {"status": <status_code>, "message": "<error_message>"}}` ([skgo-remote-form.txt](technical-sources/topic-003/sources/skgo-remote-form.txt)).
- SKGO does not generate Go client functions for remote forms yet; `fed929b` deleted `internal/devalue` entirely in favor of using `github.com/tylergannon/polytype/devalue.Uneval` directly, but didn't add clients ([skgo-git-diff-v0.5.0-fed929b.txt](technical-sources/topic-004/sources/skgo-git-diff-v0.5.0-fed929b.txt)).
- Gimbal connects to the control socket via UDS using `http.Transport` with `DialContext` ([web/submit.go:129-141](technical-sources/topic-004/sources/gimbal-web-submit-and-control.txt)).

### Recommended Implementation
- **Client Generation & Framing:** Update SKGO's `internal/gen` to emit typed Go clients mirroring the form declarations (e.g., `func (c *Client) StartReview(ctx, in) (Result, error)`). The client must construct the exact 7-byte binary prologue and devalue header used by SvelteKit.
- **Transport Configuration:** The generated CLI commands will use this client, configured with a UDS `http.Transport`, connecting to `http://gimbal/_app/remote/...`.
- **Error Types:** The Go client must parse SvelteKit 200 OK responses to return `*skgo.Invalid` for field issues and `*skgo.HTTPError` for server failure envelopes.
- **Cancellation & Retries:** The client honors `ctx.Err()` to unblock the CLI on `Ctrl+C` but must **not** retry mutations automatically, as doing so could spawn duplicate runs. It does not send cancellation RPCs to the server. The server will continue the run.
- **SKGO Release:** A new SKGO release (e.g., `v0.7.0`) must be tagged and pinned in `go.mod` after client generation lands, leaving no local `replace` directives.

### Remaining Uncertainties
- The exact coordinate/version of the SKGO release tag to pin in `go.mod` once development is complete.

## 3. Polytype Codecs and Form Data Binding

### Facts and Primary Sources
- SKGO's `formdata.Decode` bypasses Polytype codecs and uses reflection, failing on `polytype.Optional[T]` ([internal/formdata/decode.go:84-97](technical-sources/topic-005/sources/skgo-form-decoding-source.txt)).
- Omitted form fields are absent from parsed keys, while explicit empty inputs (`0`, `false`, `""`) are present ([polytype.Optional](technical-sources/topic-005/sources/polytype-optional-and-codecs-source.txt)).
- There are exactly 15 roles across the 5 workflows, defined in `cmd/gimbal/defaults.json` ([cmd/gimbal/defaults.json](technical-sources/topic-006/sources/gimbal-defaults-and-workflow-roles.txt)).

### Recommended Implementation
- **Form Decoding Patch Proposal:** Rather than generating full Polytype devalue decoders for forms, update `internal/formdata/decode.go` in SKGO to detect `polytype.Optional[T]` via `reflect.Struct`. If the field is omitted, leave `Present = false`. If present and not `devalue.Undefined`, set `Present = true` and recursively assign the scalar `Value`.
- **Role Overrides:** Generated request structs must declare explicit fields for their workflow's roles as `Role<Name> polytype.Optional[string] json:"role_<name>,omitzero"`.
- **Server Resolution:** If an override is omitted, load the default from `defaults.json`. If present, validate via `internal/binding.Roles` ([internal/binding/binding.go:58-89](technical-sources/topic-006/sources/gimbal-model-resolution-and-binding.txt)). Fail immediately with HTTP 400 or a field issue on invalid models before a run ID or goroutine is created.

### Remaining Uncertainties
- Whether role override fields should be nested under a `models` struct in the form data or kept as flat `role_<name>` top-level fields (flat fields simplify CLI flags).

## 4. Instance-Scoped Admission and Run Lifetime

### Facts and Primary Sources
- The `projectRequest` middleware requires `p != nil` (via Referer or URL path) for all `/_app/remote/` requests, returning 404 otherwise ([web/runtime.go:366-411](technical-sources/topic-007/sources/source-001-web-runtime-middleware.md)).
- `canonicalProject` and `unix.Flock` on `owner.lock` ensure cross-process isolation ([web/runtime.go:218-296](technical-sources/topic-008/sources/source-001-runtime-admission-lifecycle.md)).
- Workflow runs execute under a detached goroutine via `p.Run(ctx)` that inherits the project context, not the HTTP request context ([web/control.go:119-158](technical-sources/topic-008/sources/source-002-control-submission-async-run.md)).
- Headless mode (`--no-web`) disables `startWeb` but runs `startControl` ([web/runtime.go:186-189](technical-sources/topic-007/sources/source-001-web-runtime-middleware.md)).
- Current `selectedInstance` probing on `/control/runs` fails (returns 404) if the project is not already admitted due to `controlMux` project checks, distinguishing it from the new instance-only liveness path required ([web/submit.go:86-127](technical-sources/topic-007/sources/source-004-web-submit-control-probing.md)).

### Recommended Implementation
- **Middleware Bypass:** Update `projectRequest` to allow instance-scoped start remotes to pass through to `remotes.Intercept` even when `p == nil`, enabling the handler to admit the arbitrary `ProjectDir` supplied in the payload.
- **Origin Configuration:** Configure the control listener with `RemoteConfig{Origin: ""}` to allow Go CLI requests without Origin headers, while preserving Origin checks on the browser listener.
- **Headless Assembly:** Mount the shared `skgo.NewRemotes` on the control socket's `controlMux` in headless mode. `skgo.NewRemotes` relies on pure Go handlers and doesn't require frontend static assets.
- **Run Publication and Lifecycle:** The handler must spawn the workflow using `p.ctx` to survive the client request. Run publication, channel synchronization (via `conversationRunStartedKey`), and conversation association/validation must completely finish *before* the admission HTTP response is returned to guarantee terminal status is preserved.

### Remaining Uncertainties
- The precise mechanism to identify instance-scoped start remotes in the `projectRequest` middleware (e.g., matching a route pattern or adding an explicit allowlist).

## 5. SvelteKit Start Forms and End-to-End Verification

### Facts and Primary Sources
- Rule #10 forbids hidden `<form aria-hidden="true">` proxy forms ([ephemeral/research/svelte-idioms/dos-and-donts.md](technical-sources/topic-009/sources/gimbal-ui-conventions-and-dos-donts.md)).
- SvelteKit 3 forms support reactive `form.pending` and `form.fields.<name>.as('text')` which integrates natively with shadcn's `aria-invalid` ([form.svelte.js](technical-sources/topic-009/sources/sveltekit-remote-form-runtime.md)).
- Direct function calls (e.g., `routes.Skgo_...`) do not satisfy the Definition of Done for proving the client path ([gimbal-test-matrix-and-e2e.md](technical-sources/topic-010/sources/gimbal-test-matrix-and-e2e.md)).

### Recommended Implementation
- **Route Hierarchy:** Mount the forms explicitly at `web/src/routes/start/[workflow]/+page.svelte`. Provide a hybrid directory selector allowing the user to select an admitted project or enter a new path.
- **Form Bindings:** Use strict `skgo.Form` bindings (`<form {...form}>`). Bind inputs via `form.fields.<name>.as('text')` to surface field issues properly and avoid the proxy form anti-pattern.
- **Completion Navigation:** The server must return `{ project_id, run_id }`. The client uses `goto` to navigate to `/projects/[project]/runs/[run_id]`.
- **Test Updates:** Replace in-process tests like `web/submit_test.go` with HTTP/UDS round trips using the real generated client. Prove client disconnect survival and invalid input rejection via the wire protocol.
- **Live Proof:** Provide manual behavioral proof using Codex `gpt-5.6-luna`, Claude Haiku, or Gemini Flash to verify multi-project concurrency on a single PID. No screenshots, transcript dumps, or run logs should be committed to the repository, though they may be captured locally.

### Remaining Uncertainties
- Since the wire protocol's `validate_only` immediately returns empty issues and bypasses the Go handler, it cannot perform domain-level admission validation of an arbitrary project directory on the client side before full form submission. Any real directory path validation must happen synchronously upon full form submission.
