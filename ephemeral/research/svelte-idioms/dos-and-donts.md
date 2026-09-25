# Svelte / SvelteKit do's and don'ts for Gimbal's web app

Working copy. Once the rounds are done and the app is being fixed, this moves up into skgo (its `skills/` and `docs/`), where it can steer every skgo app, not just Gimbal.

The rule: **if Svelte or SvelteKit has a high-level API or paradigm for it, use it.** Each entry is an anti-pattern seen in `web/` (main at `aeba1bf`, 2026-09-21), the replacement, the doc section that backs it (line numbers are in `svelte-idiomatic.txt`, this folder; see `docs-subset.md`), a grep signature for finding it mechanically, and the sites. "n/10" is how many of a round's ten reviewers raised it independently (entries 0–18 from round 1, 19–31 from round 2). Raw reports: `round-1/`, `round-2/` (kept locally, not committed: agent output). Doc line numbers: in `svelte-idiomatic.txt` the "declaration tags" section (`{let/const ...}`) was restored after line 2442 on 2026-09-21, so cited lines above 2442 are now 71 higher; the section headings are the stable reference.

## Owner's hard rules (override anything a reviewer or the docs suggest)

- **No query parameters for page state or identity.** Anything worth linking to is a route path: `/conversations/[conversationID]`, `/runs/[runID]/sessions/[sessionID]`. Not `?conversation=`, `?sel=`, `?session=`, `?turn=`, `?filter=`.
  - Allowed exception (owner, 2026-09-21): a stream **position** may be a query parameter. The stream **id** is a route parameter.
- **Never touch the History API directly.** No `history.pushState` / `replaceState`, no `popstate` listeners, no `window.location` parsing. Navigation is `<a href>`, `goto`, route params, layouts.
- **Real forms for real forms, `command` for everything else.** Where the user fills in a form that needs validation (fields, issues, pending, no-JS submit), it is a `form` remote spread on that visible `<form>`. Every other mutation (stop, cancel, button actions with no fields) is a `command`. Never a hidden proxy form, never a real form driven by hand through a `command`.
- **Never bend app code around a test harness.** If Storybook or vitest can't run the idiomatic code, fix the harness (mock `$app/*`), not the app.
- The Svelte MCP `svelte-autofixer` must pass on every Svelte file written, when that server is connected.

## Rejected suggestions

- *Put the runs-list filter and search in `?filter=&q=`* (4/10, citing "Storing state in the URL", line 9107). Rejected: owner's no-query-params rule.

## Open question for the owner

- *Selection, open dialogs and the maximized pane don't survive reload or Back* (3/10). Reviewers proposed shallow routing: `pushState` from `$app/navigation` plus `page.state` (line 11738). That is SvelteKit's API, not the bare History API, but it is history manipulation. The alternative is a nested route per selectable thing. Not decided.

---

## 0. Hand-rolled HTTP endpoints instead of remote functions (owner-flagged, high: a major no-no)

Don't: a Go `http.ServeMux` (or a `+server.ts`) serving JSON or a stream that the app's own pages call with `fetch` / `EventSource`.
```go
mux.HandleFunc("GET /api/runs/{runID}", serveSnapshot)
mux.HandleFunc("GET /api/runs/{runID}/events", serveEvents)
```
```svelte
new EventSource(`/api/runs/${id}/events?${query}`)
```
Do: every page-to-server call is a remote function (skgo `query`, `query.live`, `form`, `command`) or a load. Remote functions give typed, validated arguments, transport of custom types, single-flight refreshes and reconnects; a bespoke endpoint gives none and invites a hand-built client (#2, #3).
Doc: Remote functions, line 9115 onward (`query` 9150, `query.live` 9368, `form` 9429, `command` 10019).
Grep (TS): `fetch\(`, `/api/`, `new EventSource`, `+server.ts`. Grep (Go): `HandleFunc\(`, `NewServeMux`, `"/api/`.
Sites: `internal/observation/http.go:22-27` (`/api/runs/{runID}` and `/events`), wrapped around the app in `web/server.go:71,85,125`; consumed by `routes/runs/[runID]/+page.svelte:252`. Also consumed by the CLI (`cmd/gimbal/runs.go:105`) and a second hand-built mux in `web/control.go:34-45` (`/control/runs`, `/control/steer`, `/control/steer-loop`, overlapping the `steer` remotes). A non-browser client may need a plain HTTP surface; that is a separate decision, but the browser must not use it.
skgo should detect this: a route registered beside skgo's handlers that the app's own client code fetches.

## 1. Resource identity in a query string, plus forced reloads (10/10, high)

Don't:
```svelte
<a href={`/conversations?conversation=${encodeURIComponent(item.id)}`} data-sveltekit-reload>
await goto(`/conversations?conversation=${encodeURIComponent(created.id)}`);
```
Do: a dynamic route `routes/conversations/[conversationID]/` whose load reads the param (in Go: `event.Param("conversationID")`), and plain links that SvelteKit's router handles.
```svelte
<a href="/conversations/{item.id}">
await goto(`/conversations/${created.id}`);
```
Doc: Routing, line 6939 (`[slug]` 6945); Link options / `data-sveltekit-reload`, line 11567.
Grep: `\?[a-z]+=\$\{`, `searchParams`, `URL.Query()`, `data-sveltekit-reload`
Sites: `routes/conversations/+page.svelte:53,131-132`; `routes/conversations/page.server.go:32-33` (`request.URL.Query().Get("conversation")`, the server half; compare `runs/[runID]/page.server.go:44` `event.Param("runID")`).

## 2. Hand-built `EventSource` with its own reconnect loop (9/10, high)

Don't: `new EventSource(...)` inside `$effect`, generation/attempt counters, `setTimeout(connect, 250)` retry, `onopen`/`onerror` bookkeeping.
Do: a live remote query. The server side is Go through skgo (`remote_live.go` exists in skgo v0.5.0); the client reads it like any query and gets `connected` and `reconnect()` for free.
```svelte
const events = runEvents(runID);   {await events}   {events.connected}
```
If a live query can't carry the delta protocol, wrap the connection once with `createSubscriber` in a `.svelte.ts` module; never inline it in a page.
Doc: `query.live`, line 9368; reconnecting in mutations, line 10140; `createSubscriber`, line 6674.
Grep: `new EventSource`, `setTimeout\(connect`
Sites: `routes/runs/[runID]/+page.svelte:228-277`; connection state machine in `lib/observation/index.ts:196-247`.
skgo should detect this: an app with remotes enabled that constructs `EventSource` is almost certainly missing a `query.live`.

## 3. Polling with `setInterval` + `invalidateAll` (8/10, high/medium)

Don't:
```svelte
onMount(() => { const refresh = window.setInterval(() => void invalidateAll(), 2_000); … });
```
Do: `query.live` for data that changes on the server (runs list, conversation status). `invalidateAll` reruns every load; if a refresh is truly needed, `refresh()` the one query.
Doc: `query.live`, line 9368; `invalidateAll`, line 12421.
Grep: `setInterval\(.*invalidate`, `invalidateAll\(\)`
Sites: `routes/+page.svelte:11-14`; `routes/conversations/+page.svelte:30-38`.

## 4. Manual `window` resize listener (10/10, high)

Don't:
```svelte
$effect(() => { viewportWidth = window.innerWidth; window.addEventListener("resize", onresize); return () => window.removeEventListener("resize", onresize); });
```
Do:
```svelte
import { innerWidth } from 'svelte/reactivity/window';
const narrow = $derived((innerWidth.current ?? 1280) < 1024);
```
(or `<svelte:window bind:innerWidth={viewportWidth} />`). The docs say plainly: for `window`/`document` listeners "avoid using `onMount` or `$effect`" (Best practices, ~line 4683).
Doc: `svelte/reactivity/window`, line 6089; `<svelte:window>`, line 3620.
Grep: `window.addEventListener`, `document.addEventListener`, `window.innerWidth`
Sites: `lib/run/Workspace.svelte:42-52`.

## 5. `$effect` that computes state from other state (9/10, high/medium)

Don't:
```svelte
$effect(() => { const next = selected; selectedKey = selectionKey(next); });
$effect(() => { const current = untrack(() => selection); selection = rebindSelection(current, snapshot); });
```
Do: `$derived`. Where the user can also set it (a click), reassign the derived: since Svelte 5.25 a derived can be overridden and snaps back when its inputs change.
```svelte
let selectedKey = $derived(selected ? selectionKey(selected) : undefined);
function select(k) { selectedKey = k; }   // optimistic override
```
`untrack` inside an effect to dodge its own dependency is the tell.
Doc: When not to use `$effect`, line 878; Overriding derived values, line 521; `untrack` as last resort, line 979.
Grep: `untrack\(\(\) =>`, effect bodies that only assign
Sites: `lib/run/Map.svelte:126-133`; `routes/runs/[runID]/+page.svelte:89-111`; `lib/run/Topbar.svelte:51-57` (`disconnectedAt` set from `connection`).

## 6. `$effect` that resets local state when an id changes (7/10, medium)

Don't:
```svelte
$effect(() => { scopeKey; scopeActive = "overview"; });
$effect(() => { turn; visibleCount = WINDOW; });
```
Do: scope the state to the identity with `{#key}` (remount resets it), or an overridable `$derived` keyed on the id.
```svelte
{#key scopeKey}<ScopeTabs {scope} />{/key}
let active = $derived(turn.ended ? 'result' : 'activity');   // click: active = id
```
Doc: `{#key}`, line 1676; Component and page state is preserved, line 9055; Overriding derived values, line 521.
Grep: `\$effect\(\(\) => \{\s*\w+;` (bare dependency read, then assignment)
Sites: `lib/run/DetailPane.svelte:244-250`; `lib/run/detail/SessionDetail.svelte:125-138`; `lib/run/detail/Activity.svelte:36-40`.

## 7. Asking the DOM whether a dialog is open (10/10, medium)

Don't:
```ts
function dialogOpen() { return Boolean(document.querySelector('[role="dialog"], [role="alertdialog"]')); }
```
Do: the app already knows (`cancelOpen` in the run route). Pass it down, or publish it through a narrow route-scoped context set by the dialog owner.
Doc: Context, line 4085; factoring skill, Context And Events.
Grep: `document.querySelector`, `querySelectorAll`, `\[role=`
Sites: `lib/run/Workspace.svelte:76-78`.

## 8. Finding your own elements by id or attribute (8/10, medium)

Don't:
```ts
await tick(); document.getElementById(`tab-${id}`)?.focus();
viewport?.querySelectorAll<HTMLElement>("[data-selection-key]") … .find(…)
```
Do: keep references with `bind:this` (a keyed record inside `{#each}`) or an attachment that registers the node.
```svelte
let tabs: Record<string, HTMLButtonElement> = {};
<button bind:this={tabs[tab.id]}>   …   tabs[id]?.focus();
```
Doc: `bind:this`, line 2794; `{@attach}`, line 2255.
Grep: `getElementById`, `querySelector(All)?\(`
Sites: `lib/run/detail/TabBar.svelte:17-19`; `lib/run/Map.svelte:120-123`; `lib/run/detail/concepts/SessionScroll.svelte:120-124`.

## 9. A private 1-second clock in every component (6/10, low/medium)

Don't: `let now = $state(Date.now()); $effect(() => { const t = setInterval(() => (now = Date.now()), 1000); return () => clearInterval(t); });` repeated per component.
Do: one shared clock in a `.svelte.ts` module built with `createSubscriber` (one interval, running only while something reads it).
```ts
// lib/clock.svelte.ts
const subscribe = createSubscriber((update) => { const t = setInterval(update, 1000); return () => clearInterval(t); });
export const clock = { get now() { subscribe(); return Date.now(); } };
```
Doc: `createSubscriber`, line 6674; `.svelte.js and .svelte.ts files`, line 76.
Grep: `setInterval\(\(\) => \(now = Date.now\(\)\)`
Sites: `lib/run/Topbar.svelte:59-64`; `lib/run/Map.svelte:77-81`; `lib/run/detail/SessionDetail.svelte:70-74`; `lib/run/detail/ToolCall.svelte:71-76`; `lib/run/detail/concepts/SessionScroll.svelte:59-63`.

## 10. Remote `form`s driven through hidden proxy forms (5/10, high)

Don't: three `<form aria-hidden="true">` blocks (hidden by CSS) of hidden inputs, filled by `steerForm.fields.set(request); await tick(); await steerForm.submit();`, while the textarea the user types in is separate local state.
Do (owner's rule): steer, loop and interview answer are real forms with a text field that needs validation, so each is a `form` remote spread on the visible `<form>` in the component that owns it (`<form {...steerForm}>` with `steerForm.fields.message.as('text')` on the textarea). Actions without fields stay `command` (`stopTurn`, `cancelRun` already are).
Doc: `form`, line 9429; Fields, line 9507; `command`, line 10019.
Grep: `<form[^>]*aria-hidden`, `fields.set\(`, `\.submit\(\)`
Sites: `routes/runs/[runID]/+page.svelte:129-176,349-369`; `lib/run/detail/SteerBox.svelte:44-62`; interview textarea `lib/run/DetailPane.svelte:446-469`; loop textarea `lib/run/DetailPane.svelte:69,558-562`.

## 11. A plain class plus a `revision` counter threaded as a prop (3/10, high)

Don't: `RunObservation` in a plain `.ts` file with unreactive fields; the route bumps `revision++` after every mutation, every consumer writes `$derived.by(() => { revision; return observation.snapshot(); })`, and `revision` is passed as a prop through five layers.
Do: make the model reactive where it lives: `observation.svelte.ts` with `$state` fields (or `SvelteMap` for keyed tables), so reading the data is the dependency. No `revision` prop anywhere.
Doc: `$state` Classes, line 171; `svelte/reactivity`; `.svelte.ts` files, line 76.
Grep: `revision;`, `revision\+\+`, `revision=\{revision\}`
Sites: root cause `lib/sessionstate/index.ts:51-` (`SessionProjection`, a plain class `RunObservation` wraps) and `lib/observation/index.ts:196-386`; `routes/runs/[runID]/+page.svelte:42-59`; `DetailPane`, `SessionDetail`, `Activity`, `Map`, `Topbar`, `MessageRow.svelte:8-16`; `Activity.svelte:98` even adds two counters (`revision={revision + observation.messageRevision(turn, message.id)}`).

## 12. Navigation done with a button and `goto` (1/10, high)

Don't: `<button onclick={() => openRun(item)}>` → callback prop → `goto(\`/runs/${id}\`)`. Loses open-in-new-tab, copy link, hover preview, preloading.
Do: `<a href="/runs/{item.run.id}">` around the card.
Doc: `+page.svelte` ("SvelteKit uses `<a>` elements to navigate"), line 6978; Link options, line 11511.
Grep: `goto\(` in click handlers; `on\w+=` props whose only job is navigating
Sites: `lib/run/RunsList.svelte:81-84,142,189`; `routes/+page.svelte:16-18`.

## 13. Callback bundles relayed through a component that doesn't use them (1/10, medium)

Don't: the route builds `onsteer`, `onloop`, `onanswer`, `onstop`, `onselectscope`, passes them to `DetailPane`, which forwards most of them untouched to `SessionDetail`.
Do: the component that performs the action imports the remote itself (factoring skill: "If one leaf component is the only consumer of a remote result, import the remote in that component"). Pass ids down, not controller bundles.
Doc: `svelte-component-factoring` SKILL.md, State And Remote Placement; Context And Events.
Grep: `on\w+` props that a component only passes through
Sites: `routes/runs/[runID]/+page.svelte:282-333` → `lib/run/DetailPane.svelte` → `lib/run/detail/SessionDetail.svelte:43-45`; `onselectscope`→`onscope` runs two more chains to its only caller `ContextList.svelte:51`: `DetailPane:542 → ScopeContext.svelte:13,24,29 → ContextList` and `SessionDetail:56,178 → PromptAndContext.svelte:14,41 → ContextList`.

## 14. Timer races instead of event facts (3/10, low)

Don't: `onblur={() => window.setTimeout(() => (searchOpen = false), 100)}` so a click on a result can land first.
Do: `onfocusout` on the container; close only when `event.relatedTarget` is outside it.
Doc: Basic markup / Events, line 1368; `svelte/events`, line 5987.
Grep: `setTimeout\(\(\) => \(\w+ = false\)`
Sites: `lib/run/Topbar.svelte:179`; `lib/run/detail/Payload.svelte:58` (`setTimeout(() => (copied = false), 1200)` from a click handler, never cleared, can write after unmount; own it in an `$effect` with teardown).

## 15. Autoscroll with `$effect` + `requestAnimationFrame` (1/10, medium)

Don't: post-render `$effect`, then `requestAnimationFrame(() => (el.scrollTop = el.scrollHeight))`.
Do: the docs' own chat-window pattern: `$effect.pre` checks whether you're at the bottom before the DOM updates, then `tick()` and scroll.
Doc: `$effect.pre`, line 771; Chat window example, line 4407.
Grep: `requestAnimationFrame`
Sites: `lib/run/detail/Activity.svelte:72-84`; `lib/run/detail/Payload.svelte:48-52` (plain `$effect` on `shown`: despite its "once, at mount" comment it re-pins stdout/stderr to the bottom on every chunk, yanking a reader who scrolled up — a behaviour bug, 5/10).

## 16. `untrack` where nothing is tracked (1/10, low)

Don't: `let wrapped = $state(untrack(() => wrap));` in component setup (`SessionDetail.svelte:125` does the same).
Do: `let wrapped = $state(wrap);`. If the intent is "follow the prop until the user toggles", that's #5: an overridable `$derived`.
Doc: `untrack`, line 519.
Grep: `\$state\(untrack`
Sites: `lib/run/detail/Payload.svelte:34`; `lib/run/detail/SessionDetail.svelte:125`.

## 17. The same selection kept in two places (1/10, low)

Don't: instance selection held in `Map`'s `selectedInstances` record and again as `Sheet`'s `localSelectedInstance`, synced by `oninstancechange`.
Do: one owner (the nearest common owner), `$derived` everywhere else.
Doc: factoring skill, State And Remote Placement.
Sites: `lib/run/Sheet.svelte:86-95`; `lib/run/Map.svelte:66,100-114,147-155`.

## 18. `class:` directive instead of the class attribute's object form (1/10, low)

Don't: `class:active={x}`. Do: `class={{ active: x, working: y }}`; the docs call the attribute "more powerful and composable".
Doc: class, line 3085. Grep: `class:[a-z]`. Sites: 31 uses in `lib/run/` and `routes/`.

## Housekeeping found on the way

- `lib/run/detail/concepts/` (`SessionScroll.svelte`, `SessionTabs.svelte`, stories) is design-concept scaffolding from PR 328 that no page uses, and it carries several of the anti-patterns above. Delete it.

---

# Round 2 additions

## 19. `{#each}` without a key, or keyed by index (5/10, high)

Don't: `{#each layout.sheets as sheet}` on a list rebuilt every event, rendering components that own `$state` (`Sheet`'s `localSelectedInstance`); `{#each outputParts as content, index (index)}` on a list that grows while streaming.
Do: key on identity the item already carries.
```svelte
{#each layout.sheets as sheet (sheet.selectionKey)}   {#each instances as instance (instance.key)}
```
Without a key Svelte patches by position, so a component's local state can land on a different item when the list shifts. It is also probably why the app scans the DOM for `[data-selection-key]` (#8).
Doc: `{#each}` keyed each blocks, line 1575; Best practices, Each blocks ("Do not use the index as a key"), ~line 4699.
Grep: `\{#each [^}]* as \w+(, \w+)?\}` (no key), `\((index|i)\)\}`
Sites: `lib/run/Map.svelte:296,299,306,339,345,351,367,390`; `lib/run/Sheet.svelte:145,205`; `lib/run/Services.svelte:31`; `lib/run/Group.svelte:40`; `lib/run/detail/ToolCall.svelte:136,156` (index keys).

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

## 21. `invalidateAll()` after a mutation that already returned the new state (2/10, high)

Don't: `current = await sendConversationMessage(...); await invalidateAll();` — reruns every load on every message.
Do: single-flight mutations: the server-side mutation refreshes or sets the specific queries it changed; drop `invalidateAll`.
Doc: Single-flight mutations, ~line 10086; `invalidateAll` ("all load and query functions"), ~line 12421.
Grep: `await invalidateAll\(\)` after an awaited remote call
Sites: `routes/conversations/+page.svelte:54,77`.

## 22. `typeof localStorage` guards, and remembered UI read after mount (8/10, low/medium)

Don't: `if (typeof localStorage === "undefined") return null;`, then read the stored pane width inside a mount `$effect`, so SSR renders the default and the pane jumps after hydration.
Do: `browser` from `$app/environment`; read the remembered value during component setup in the browser.
Doc: `$app/environment` `browser`, ~line 12115; Best practices on effects and SSR, ~line 4641.
Grep: `typeof (window|document|localStorage|sessionStorage) ===? ["']undefined["']`
Sites: `lib/run/paneSize.ts:19,28`; `lib/run/Workspace.svelte:43-44`.

## 23. Props that exist only for stories and tests (1/10, high — owner's harness rule)

Don't: `/** Overrides the remembered width for the first render only (stories, tests). */ initialWidth?: number; initialMaximized?: boolean;`
Do: seed or mock `localStorage` in the Storybook decorator / vitest setup; the component reads its real source.
Doc: owner's rule; "Mounting components with context" for injection at the boundary, ~line 4252.
Grep: `\((stories|tests?)[,)]`, `initial\w+\?:` props
Sites: `lib/run/Workspace.svelte:21-22,33-35,43-44`; `lib/run/Workspace.stories.svelte:55`.

## 24. A counter prop used to re-trigger an effect (2/10, medium)

Don't: `reveal = { request: ++revealSequence, selection }` passed to `Map`, whose `$effect` compares `next.request === lastRevealRequest` to re-run a scroll.
Do: an imperative request is a method: `export function revealSelection(sel)` in `Map`, then `<Map bind:this={map} />` and `map.revealSelection(sel)`.
Doc: `bind:this` on components, line 2794.
Grep: `request:\s*\+\+\w+`, `last\w*Request`
Sites: `lib/run/Map.svelte:135-140`; `routes/runs/[runID]/+page.svelte:44,50,116,305`.

## 25. `value` + `onchange` + a DOM cast instead of `bind:value` (4/10, low)

Don't: `<select value={provider} onchange={chooseProvider}>` with `provider = (event.currentTarget as HTMLSelectElement).value`.
Do: `<select bind:value={() => provider, (v) => { provider = v; model = defaults[v] ?? ''; }}>` (a function binding), or `bind:value` plus an `onchange` for the side effect.
Doc: `<select bind:value>`, line 2650; Function bindings, line 2460.
Grep: `currentTarget as HTML(Select|Input|TextArea)Element\)\.value`
Sites: `routes/conversations/+page.svelte:40-43,110`.

## 26. Load data shipped as a JSON string and parsed in the page (1/10, medium)

Don't: Go `graphJSON = string(json.Marshal(graph))` in the load, then `JSON.parse(data.graph) as Graph` in the page, right beside `Snapshot`, which already crosses through the app's `transport` hook.
Do: return the value itself (devalue carries it) or add it to `transport`.
Doc: Loading data, "must return data that can be serialized with devalue", ~line 7598; `transport` hook.
Grep (Go): `string\(encoded`, `json.Marshal` in a `page.server.go`; (TS): `JSON.parse\(data\.`
Sites: `routes/runs/[runID]/page.server.go:56-64`; `routes/runs/[runID]/+page.svelte:35-37`.

## 27. No `<svelte:boundary>` anywhere (1/10, high)

Don't: render streamed, untyped transcript and tool-call JSON with no error containment; one malformed row throws and blanks the pane (or the page).
Do: wrap the transcript list and each tool call's body in `<svelte:boundary>` with a `failed` snippet ("couldn't render this message").
Doc: `<svelte:boundary>`, line 3480; `failed`, ~line 3521.
Grep: absence of `svelte:boundary` in a component that renders server JSON in `{#each}`
Sites: `lib/run/detail/Activity.svelte:94-101`; `lib/run/detail/ToolCall.svelte`; `lib/run/MessageRow.svelte:36-77`.

## 28. `$state` arrays and records used as sets and maps (1/10, low/medium)

Don't: `let foldedScopes = $state<string[]>([])` with `[...foldedScopes, key]` / `.filter`; `selectedInstances = { ...selectedInstances, [k]: v }`.
Do: `SvelteSet` / `SvelteMap` from `svelte/reactivity`: `foldedScopes.add(key)`, `selectedInstances.set(k, v)`.
Doc: `svelte/reactivity` SvelteMap ~line 6439, SvelteSet ~line 6526.
Grep: `\$state<(string\[\]|Record<)`, `= \[\.\.\.\w+, `, `= \{ \.\.\.\w+, \[`
Sites: `lib/run/Map.svelte:66-67,101-112,148-177`.

## 29. `{@const}` is legacy syntax (2/10, low)

Don't: `{@const failed = !command.interrupted && …}`. Do: `{const failed = $derived(!command.interrupted && …)}` (declaration tags).
Doc: `{@const ...}` note, line 2429; `{let/const ...}` (restored), line 2443.
Grep: `\{@const `
Sites: `lib/run/DetailPane.svelte:472`; `lib/run/detail/ContextList.svelte:34`.

## 30. `svelte-ignore` on an a11y warning with no reason (1/10, low)

Don't: bare `<!-- svelte-ignore a11y_no_noninteractive_tabindex -->` on the splitter. Do: state why it is safe on the same line (a `role="separator"` with full keyboard handling), or use a control whose native semantics satisfy the check.
Doc: Basic markup, Comments / `svelte-ignore`, ~line 1462.
Grep: `svelte-ignore a11y_`
Sites: `lib/run/Splitter.svelte:77-78`.

## 31. Remote functions declared `"unchecked"`, validated by hand in Go (1/10, medium — an skgo matter)

Don't: `form("unchecked", …)` / `command("unchecked", …)` everywhere, with `strings.TrimSpace(arg.X) == ""` → `skgo.Invalidf` in each Go handler.
Do: validation declared once from the Go argument type, so issues land in `fields.issues()` / `aria-invalid`. The `.remote.ts` stubs are generated, so this is for skgo to provide (derive a Standard Schema, or validate from struct tags and report per-field issues), not for app code.
Doc: remote function argument validation, ~line 9247; Handling validation errors, ~line 10361.
Grep: `\("unchecked"`
Sites: `routes/steer.remote.ts`, `control.remote.ts`, `conversation.remote.ts`, `interview.remote.ts` and their `.remote.go` handlers.

## More housekeeping (round 2)

- `lib/run/SessionTimeline.svelte` is imported by nothing (only a comment in `Activity.svelte:2` names it) and carries #11. Delete it.
- `lib/run/SmallStates.svelte:44-79`: the `loading` and `disconnected` states (with `onreconnect`) are rendered only by its story. Wire `disconnected` to the run's live connection (`connected` / `reconnect()` if the stream becomes `query.live`) or delete them.

---

# For skgo (where this document is headed)

What skgo could detect or provide, so agents stay on the rails without a ten-reviewer audit:

- **Hand-rolled endpoints (#0):** a route mounted beside skgo's handlers that the app's own client fetches; `new EventSource` or `fetch('/api/…')` in app code when remotes are enabled.
- **Live data without `query.live` (#2, #3):** `setInterval(…invalidateAll)`, `new EventSource`. And a real gap to close: `query.live` resends the whole value per yield, so a large growing document (a run observation is 7–17 MB with ~100 changes/sec) cannot use it. skgo needs a documented pattern or primitive for delta or windowed live data (under study now, `research/` POCs).
- **Real forms as commands (#20) and script-only actions as forms (#10):** visible from the Go declaration (`skgo.Command` vs `skgo.Form`) and the client call site.
- **Validation (#31):** generate it from Go types instead of `"unchecked"` plus hand checks.
- **Load data as JSON strings (#26):** a `string` field holding `json.Marshal` output in a load.
- **Query params used as identity (#1):** `URL.Query().Get` in a load where a route param fits.
