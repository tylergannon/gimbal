# Gimbal UI Conventions, Svelte Idioms, and Form Bindings

- **Origin**: `/Users/tyler/.codex/worktrees/d798/gimbal`
- **Files**:
  - `ephemeral/research/svelte-idioms/dos-and-donts.md`
  - `docs/web-app.md`
  - `web/src/lib/components/ui/input/input.svelte`
- **Commit/Baseline**: `main` / `e161721c`
- **Retrieval Date**: 2026-09-23

---

## 1. Owner's Hard Rules for Svelte and SvelteKit

From `ephemeral/research/svelte-idioms/dos-and-donts.md` (lines 7–15):

```markdown
## Owner's hard rules (override anything a reviewer or the docs suggest)

- **No query parameters for page state or identity.** Anything worth linking to is a route path: `/conversations/[conversationID]`, `/runs/[runID]/sessions/[sessionID]`. Not `?conversation=`, `?sel=`, `?session=`, `?turn=`, `?filter=`.
  - Allowed exception (owner, 2026-09-21): a stream **position** may be a query parameter. The stream **id** is a route parameter.
- **Never touch the History API directly.** No `history.pushState` / `replaceState`, no `popstate` listeners, no `window.location` parsing. Navigation is `<a href>`, `goto`, route params, layouts.
- **Real forms for real forms, `command` for everything else.** Where the user fills in a form that needs validation (fields, issues, pending, no-JS submit), it is a `form` remote spread on that visible `<form>`. Every other mutation (stop, cancel, button actions with no fields) is a `command`. Never a hidden proxy form, never a real form driven by hand through a `command`.
- **Never bend app code around a test harness.** If Storybook or vitest can't run the idiomatic code, fix the harness (mock `$app/*`), not the app.
- The Svelte MCP `svelte-autofixer` must pass on every Svelte file written, when that server is connected.
```

---

## 2. Prohibition of Hidden Proxy Forms (Rule #10)

From `ephemeral/research/svelte-idioms/dos-and-donts.md` (lines 171–178):

```markdown
## 10. Remote `form`s driven through hidden proxy forms (5/10, high)

Don't: three `<form aria-hidden="true">` blocks (hidden by CSS) of hidden inputs, filled by `steerForm.fields.set(request); await tick(); await steerForm.submit();`, while the textarea the user types in is separate local state.
Do (owner's rule): steer, loop and interview answer are real forms with a text field that needs validation, so each is a `form` remote spread on the visible `<form>` in the component that owns it (`<form {...steerForm}>` with `steerForm.fields.message.as('text')` on the textarea). Actions without fields stay `command` (`stopTurn`, `cancelRun` already are).
Doc: `form`, line 9429; Fields, line 9507; `command`, line 10019.
Grep: `<form[^>]*aria-hidden`, `fields.set\(`, `\.submit\(\)`
Sites: `routes/runs/[runID]/+page.svelte:129-176,349-369`; `lib/run/detail/SteerBox.svelte:44-62`; interview textarea `lib/run/DetailPane.svelte:446-469`; loop textarea `lib/run/DetailPane.svelte:69,558-562`.
```

---

## 3. Real Forms vs Commands (Rule #20)

From `ephemeral/research/svelte-idioms/dos-and-donts.md` (lines 259–270):

```markdown
## 20. Real forms driven by `command()` and `onsubmit` (6/10, high)

Don't: `<form onsubmit={create}>` → `event.preventDefault()` → `await createConversation(...)`, with hand-rolled `creating`/`sending`/`feedback` state and optimistic splicing.
Do: declare it a `form` remote (`skgo.Form`) and spread it on the real form; `.pending`, `.fields`, issues, `.result` and no-JS submission come with it.
```svelte
<form {...createConversation}><input {...createConversation.fields.title.as('text')} /></form>
```
The inverse of #10: here a real form uses `command`; there a script-only action uses `form`.
Doc: `form`, line 9429; Fields, line 9507; `command` ("Prefer `form` where possible"), line 10019.
Grep: `onsubmit=\{` in a file importing a `*.remote` whose Go side is `skgo.Command(`
Sites: `routes/conversations/+page.svelte:45-79,103,212`; `routes/conversation.remote.go:27-63`.
```

---

## 4. UI Component Primitives and Workflow Start Stories

From `docs/web-app.md` (lines 17–23, 89–95, 119):

```markdown
| Decision | Choice | Why |
| --- | --- | --- |
| Component kit | shadcn-svelte, copied into `web/src/lib/components/ui/` | The design team and the coding agents read the same primitives. |
| Import alias | `#lib/...`, the Node subpath import in `web/package.json`. SvelteKit 3 removed `$lib`. | One alias in the app. `web/tsconfig.json` repeats it under `paths` only because the shadcn CLI checks for it there. |

...
| F2 | **Start a workflow.** One form per workflow, controls named by its Go input struct. The first is the sprint workflow. | designed, not built |
...
| `op.start-sprint` | Fill in the sprint form, start it, arrive on its run page. |
```

---

## 5. Built-in `aria-invalid` Support in shadcn-svelte Input

From `web/src/lib/components/ui/input/input.svelte` (lines 24–48):

```svelte
<input
    bind:this={ref}
    data-slot={dataSlot}
    class={cn(
        "dark:bg-input/30 border-input focus-visible:border-ring focus-visible:ring-ring/50 aria-invalid:ring-destructive/20 dark:aria-invalid:ring-destructive/40 aria-invalid:border-destructive dark:aria-invalid:border-destructive/50 h-9 rounded-md border bg-transparent px-2.5 py-1 text-base shadow-xs transition-[color,box-shadow] file:h-7 file:text-sm file:font-medium focus-visible:ring-3 aria-invalid:ring-3 md:text-sm w-full min-w-0 outline-none file:inline-flex file:border-0 file:bg-transparent file:text-foreground placeholder:text-muted-foreground disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-50",
        className
    )}
    {type}
    bind:value
    {...restProps}
/>
```
