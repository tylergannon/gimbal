# Adversarial review: optional OAuth login design, round 06

## Target

`ephemeral/auth/design.md` at commit `b904137f` (branch
`codex/optional-auth-design`), reviewed in full against:

- the authoritative request in `ephemeral/auth/requirements.md`;
- the repository instructions in `AGENTS.md`, including the Svelte rules in
  `ephemeral/research/svelte-idioms/dos-and-donts.md`;
- the provider references in `ephemeral/auth/provider-references.md`.

The launch prompt's limits were operating constraints only: a read-only review,
an artifact path, and the same authoritative sources. Nothing in it narrowed
the review's scope or asked for a particular verdict, so there was no
narrowing to ignore.

## Evidence inspected

- `ephemeral/auth/design.md` (whole file) and `git diff d3add1dc b904137f`.
- How this revision handles round 05's finding:
  - The root layout load now returns `authEnabled` and `user` (an ID/name DTO
    or null) from server context, without calling a remote. The account UI
    renders only when a user is present (`design.md:221-227`).
  - `currentUser` is now only a protected probe. It returns null with auth
    off, and the run page calls it only when `authEnabled` is true
    (`design.md:229-233`).
  - The acceptance list now includes the sign-in page rendering while signed
    out, through both SSR and client navigation (`design.md:379-380`).
  - The anonymous-SSR and auth-off parts of round 05's finding are resolved.
- How the layout data behaves after the first load. I checked when the client
  reruns the root layout load:
  - skgo v0.29.0, `data.go:611-690`. Each data node carries a `uses` record,
    and the client decides from it whether to rerun the load.
  - `event.go:90-95`. `Event.Request()` records nothing.
  - `loadevent.go:98-121,185-195`. Only `URL()`, `Param()`, `SearchParam()`,
    `RouteID()`, `Depend()`, and `Parent()` record a dependency.
  - `web/src/routes/layout.server.go:13-27`. The root layout load reads only
    `event.Request()` (context and the desktop header), so its `uses` is
    empty. The new `authEnabled` and `user` come from that same request
    context (`design.md:221-223`).
  - SvelteKit 3.0.0 `runtime/client/client.js:1286-1311` (`has_changed`).
    With no recorded uses and no forced invalidation, a node is unchanged.
  - `client.js:1443-1475` (`load_route`). An unchanged server node is not
    requested again (`invalid_server_nodes`), and the previous branch data is
    reused.
  - `client.js:2026-2040`. A data-redirect envelope is followed by
    `navigate(...)` with no forced invalidation, so the same reuse applies.
- The rest of the design was re-read in full: storage contracts, provider,
  callback, gate exceptions, origin, selection, provisioning, file store, and
  integration order. Findings from earlier rounds remain resolved.

No implementation exists yet, so there was no implementation proof to inspect.

## Findings

### 1. Issue: the layout's `user` is loaded once and reused on client navigation, so after expiry or logout the sign-in page shows the old account and logout control

The design says the shared account UI "renders only when a user is present"
and "never renders logout while signed out" (`design.md:225-227`). It also
says every way a session ends returns the browser to `/auth/login` through
client-side navigation:

- the data-redirect envelope for route data (`design.md:206-208`);
- `goto('/auth/login')` from a remote 401 (`design.md:212-214`);
- the same `goto` from the run page's probe (`design.md:236-237`).

The root layout load records no dependency. It reads only `event.Request()`
(`layout.server.go:16-21`), which skgo does not track (`skgo event.go:90-95`).
The design adds `authEnabled` and `user` from the same request context
(`design.md:221-223`). Kit's client therefore treats the root node as
unchanged on every client navigation and reuses its previous data
(`client.js:1294-1296,1452-1462,1464-1480`). Following a data redirect does
not force invalidation either (`client.js:2026-2040`).

Reproduction from the design as written:

1. Sign in. The callback's full-document redirect to `/` loads the layout with
   `user = {id, name}`.
2. Let the eight-hour session expire (`design.md:165`), or log out in another
   tab, or have the operator run `gimbal user remove`.
3. Click any in-app link. The protected `__data.json` returns the redirect
   envelope, and Kit navigates client-side to `/auth/login`.
4. Kit asks the server only for the login page's node. It reuses the root
   layout node, which still says `user = {id, name}`.
5. The sign-in page renders the expired account's name and the logout control,
   which the design says never happens. Clicking logout sends a protected
   command, which gets 401, whose handler calls `goto('/auth/login')`, which
   is the page already shown.

The design never says what the browser does after a successful `logout`
command (`design.md:174-178`). Whether the owner stays on the page or calls
`goto('/auth/login')`, the layout keeps showing the signed-in user until a
full reload.

Sign-in itself still works, because the button runs the public `beginLogin`.
The design's own acceptance case fails, though: "the sign-in page rendering
while signed out through both SSR and client navigation" (`design.md:379-380`).
The layout also still believes a user is present, and that belief is what the
design uses to keep anonymous chrome, such as `RecorderBridge`, out of the
anonymous layout (`design.md:198-199`).

**Fix:** state how layout auth state is refreshed. The smallest option is for
the root layout load to record a dependency that changes when the route
changes, such as `event.RouteID()` (`skgo loadevent.go:185-195`). The load
then reruns on navigation to `/auth/login`, and its `user` is null there. An
alternative is for every app-owned path to sign-in, including the logout
completion, to use `goto('/auth/login', { invalidateAll: true })`. That needs
a separate answer for the data-redirect path, which the application does not
control. Name the chosen mechanism next to the layout paragraph.

## Outcome

material findings remain
