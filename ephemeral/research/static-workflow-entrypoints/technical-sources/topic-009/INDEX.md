# Topic 009 Index: SvelteKit Start Form Implementation and UI Conventions

Routes assigned research questions for Topic 009 to primary local evidence, annotated excerpts, and architectural findings.

## Local Sources

- [sveltekit-remote-form-runtime.md](./sources/sveltekit-remote-form-runtime.md): Pinned SvelteKit 3 client runtime (`form.svelte.js`, `form-utils.js`, `shared.svelte.js`) for `form` remotes, field proxies, `.as(...)`, `aria-invalid`, `issues()`, `pending`, and server error throwing.
- [gimbal-ui-conventions-and-dos-donts.md](./sources/gimbal-ui-conventions-and-dos-donts.md): Owner's hard rules in `ephemeral/research/svelte-idioms/dos-and-donts.md`, web app features in `docs/web-app.md`, and shadcn-svelte primitives (`web/src/lib/components/ui/input/input.svelte`).
- [gimbal-route-hierarchy-and-middleware.md](./sources/gimbal-route-hierarchy-and-middleware.md): Route layout in `web/src/routes/+layout.svelte`, root projects page `web/src/routes/+page.svelte`, and project-scoping middleware in `web/runtime.go`.
- [skgo-form-binding-clip.md](./clips/skgo-form-binding-clip.md): Concrete implementation clip demonstrating idiomatic SvelteKit 5 / shadcn start form integration.

---

## Question Routing and Local Evidence

### Q1: Where in the SvelteKit route hierarchy (web/src/routes/) should the start forms for the five built-ins be mounted to align with existing page and path conventions?

- **Supported Facts**:
  - Existing route structure in `web/src/routes/` is:
    - `/` (`routes/+page.svelte`): instance root, lists admitted projects ([`gimbal-route-hierarchy-and-middleware.md:3`](./sources/gimbal-route-hierarchy-and-middleware.md#3-instance-root-projects-page)).
    - `/projects/[project]` (`routes/projects/[project]/+page.svelte`): project runs list.
    - `/projects/[project]/runs/[runID]`: run workspace and inspection.
    - `/projects/[project]/conversations`: chat and worktree manager.
  - Owner's hard rule forbids query parameters for identity or page state ([`gimbal-ui-conventions-and-dos-donts.md:1`](./sources/gimbal-ui-conventions-and-dos-donts.md#1-owners-hard-rules-for-svelte-and-sveltekit)). Identity must be expressed as path segments.
  - Start remotes are instance-scoped and admit arbitrary directories not previously known to the host ([`implementation-plan.md:138-144`](../../implementation-plan.md#3-generate-concrete-starts-and-admit-projects-on-first-use)). Under `web/runtime.go:391`, the existing middleware intercepts `/projects/` and `/remote/` and returns 404 unless a matching admitted project ID is found in the path or Referer ([`gimbal-route-hierarchy-and-middleware.md:1`](./sources/gimbal-route-hierarchy-and-middleware.md#1-project-scoping-middleware-in-webruntimego)).
- **Inference & Recommendation**:
  - The start forms must be accessible outside of `/projects/[project]` so a user can admit a new project.
  - Mounting route: `web/src/routes/start/[workflow]/+page.svelte` (e.g. `/start/review`, `/start/implement`, `/start/research-document`, `/start/pyramid-summary`, `/start/validate-product`), or a unified hub at `web/src/routes/start/+page.svelte`.
  - To support launching directly within an admitted project without retyping its path, a parallel route `web/src/routes/projects/[project]/start/[workflow]/+page.svelte` (or navigating to `/start/[workflow]` with a route-scoped prefill) maintains path-based identity.
- **Unresolved / Decision Points**:
  - Whether to use discrete dynamic route `/start/[workflow]` or explicit static directories `/start/review/`, etc. Given that the five built-ins are fixed at compile time, static directories or a validated `[workflow]` slug with a type switch align cleanly with SvelteKit.

---

### Q2: How do existing forms in web/src/ integrate with skgo.Form bindings to display pending states, field-specific validation errors, and server failure envelopes?

- **Supported Facts**:
  - In existing code (`web/src/routes/projects/[project]/runs/[runID]/+page.svelte:427-447`), forms (`steerForm`, `loopForm`, `answerForm`) were driven via hidden `<form aria-hidden="true">` proxy elements with manual `.fields.set()` and `.submit()` calls.
  - `ephemeral/research/svelte-idioms/dos-and-donts.md` rule #10 explicitly identifies hidden proxy forms as a severe anti-pattern, requiring real `<form {...formRemote}>` bindings ([`gimbal-ui-conventions-and-dos-donts.md:2`](./sources/gimbal-ui-conventions-and-dos-donts.md#2-prohibition-of-hidden-proxy-forms-rule-10)).
  - SvelteKit 3 form runtime (`form.svelte.js` and `form-utils.js`) provides:
    - **Pending State**: `form.pending` (reactive number of in-flight submissions) ([`sveltekit-remote-form-runtime.md:2`](./sources/sveltekit-remote-form-runtime.md#2-field-proxy-accessor-and-reactive-binding)).
    - **Field-specific Errors**: `form.fields.<name>.issues()` returns `InternalRemoteFormIssue[]` or `undefined`. Spreading `form.fields.<name>.as('text')` automatically binds `name`, `value`, `defaultValue`, and reactive `aria-invalid` when issues exist ([`sveltekit-remote-form-runtime.md:3`](./sources/sveltekit-remote-form-runtime.md#3-field-as-properties-and-aria-invalid-generation)).
    - **Shadcn Integration**: Shadcn `Input` (`web/src/lib/components/ui/input/input.svelte`) styles `aria-invalid` natively with `aria-invalid:border-destructive aria-invalid:ring-destructive/20` ([`gimbal-ui-conventions-and-dos-donts.md:5`](./sources/gimbal-ui-conventions-and-dos-donts.md#5-built-in-aria-invalid-support-in-shadcn-svelte-input)).
    - **Server Failure Envelopes**: When Go handlers return HTTP errors (status 400/500), `remote_request` throws `HandledHttpError` on `result.type === 'error'` ([`sveltekit-remote-form-runtime.md:4`](./sources/sveltekit-remote-form-runtime.md#4-server-error-envelopes-in-remote_request)). Client code catches this in `form.enhance` or display it in an alert box.
- **Inference & Recommendation**:
  - Real forms must spread `form` on `<form {...form}>` and use `form.fields.<field>.as('text')` on inputs. See [`skgo-form-binding-clip.md`](./clips/skgo-form-binding-clip.md) for the verified code pattern.
- **Contradictions Identified**:
  - The historical codebase in `+page.svelte` violated the owner's rule #10. The new start forms must strictly adhere to rule #10 and rule #20.

---

### Q3: How should the UI structure the project directory input (choosing existing projects vs entering new paths) alongside workflow parameters and model overrides, navigating to the admitted run page upon completion?

- **Supported Facts**:
  - Server context projects `data.projects` (`ProjectChoice{ID, Path}`) into load functions via `hooks.Projects(ctx)` ([`gimbal-route-hierarchy-and-middleware.md:4`](./sources/gimbal-route-hierarchy-and-middleware.md#4-context-projection-types-in-websrcprojectgo)).
  - Model defaults are structured per-role in `cmd/gimbal/defaults.json` (`gpt-6-luna`, `claude-opus-5-5`, `gemini-3.8-flash-medium`, etc.) ([`cmd/gimbal/defaults.json`](../../../../../cmd/gimbal/defaults.json)).
  - SvelteKit navigation must not use raw History API; it must use `goto` from `$app/navigation` ([`gimbal-ui-conventions-and-dos-donts.md:1`](./sources/gimbal-ui-conventions-and-dos-donts.md#1-owners-hard-rules-for-svelte-and-sveltekit)).
  - Remote form handlers must return admission data (`{ project_id: string, run_id: string }`) rather than an HTTP redirect, so that SvelteKit performs client-side router navigation to the admitted run page ([`implementation-plan.md:171`](../../implementation-plan.md#4-connect-the-cli-and-human-start-forms)).
- **Inference & Recommendation**:
  - **Project Directory Input**: Structure as a hybrid selector:
    - If `data.projects` is non-empty, provide a Shadcn `Select` or radio toggle to choose an existing admitted project path, plus an option to "Enter new repository path" which reveals an `Input` for arbitrary filesystem paths.
    - If accessed from `/projects/[project]/start/...`, prefill and freeze the project path to the active project.
  - **Workflow Parameters & Overrides**:
    - Group into two visual sections: "Parameters" (required inputs like target commit, spec file path, prompt) and an expandable/collapsible "Model Overrides" section.
    - Model override fields must allow selecting known models or leaving blank to inherit role defaults from `defaults.json`.
  - **Completion Navigation**:
    - Inside `form.enhance(async ({ submit }) => { const ok = await submit(); if (ok && form.result) { await goto('/projects/' + form.result.project_id + '/runs/' + encodeURIComponent(form.result.run_id)); } })`.
- **Unresolved / Decision Points**:
  - Autocomplete / validation of filesystem directory paths before submission: browser security restrictions prevent client-side local directory scanning, so project validation happens on form submission or via remote validation-only preflight.
