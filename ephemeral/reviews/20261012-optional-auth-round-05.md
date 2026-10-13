# Adversarial review: optional OAuth login design, round 05

## Target

`ephemeral/auth/design.md` at commit `d3add1dc` (branch
`codex/optional-auth-design`), all 383 lines, reviewed in full against:

- the authoritative request in `ephemeral/auth/requirements.md`;
- the repository instructions in `AGENTS.md`, including the Svelte rules in
  `ephemeral/research/svelte-idioms/dos-and-donts.md`;
- the provider references in `ephemeral/auth/provider-references.md`.

The launch prompt's limits were operating constraints only: a read-only review,
an artifact path, and the same authoritative sources. Nothing in it narrowed
the review's scope or asked for a particular verdict, so there was no
narrowing to ignore.

## Evidence inspected

- `ephemeral/auth/requirements.md`, `ephemeral/auth/design.md` (whole file),
  `ephemeral/auth/provider-references.md`, and `git diff 5c1255c4 d3add1dc`.
- Round 04's findings and how this revision handles them:
  - **The run page's observation stream retry loop.** On a failed connection,
    the EventSource owner now refreshes and awaits a protected `currentUser`
    query before reconnecting. A 401 stops the retries and navigates to
    sign-in. The connection-generation and teardown checks are preserved
    (`design.md:221-232`). The acceptance list now includes this stream
    (`design.md:376-379`).
  - **New-tab `/api/` links.** Any unauthenticated top-level navigation, on any
    path, now gets a 303 to `/auth/login`, selected by
    `Sec-Fetch-Mode`/`Sec-Fetch-Dest` (`design.md:202-205`).
- I checked the new client mechanism against the SvelteKit 3.0.0 client in the
  pnpm store this app resolves:
  - `runtime/client/remote-functions/query/proxy.js:30-48,80-81`. A
    `currentUser()` proxy built outside a reactive context creates or reuses
    its cache entry, and `refresh()` delegates to that cached instance.
  - `query/instance.svelte.js:110-165,231-233`. `refresh()` drops any
    hydrated response and starts a new request. On failure it rejects with
    `new HandledHttpError(await handle_error(e, …))`.
  - `remote-functions/shared.svelte.js:113-133`. A bare 401 becomes
    `HttpError({status: 401, …})`.
  - `runtime/client/client.js:2474-2505`. `handle_error` of an `HttpError`
    returns `{...error.body, ...hookResult}`. This app defines no client
    `handleError` hook: `web/src/hooks.ts` exports only `transport`, and there
    is no `hooks.client.*`. The status therefore stays 401.
  - `exports/internal/shared.js:3-24`. `HttpError` takes `status` from the
    body, and `HandledHttpError` extends it, so `isHttpError(e, 401)` holds
    for the awaited refresh. The round-05 probe works as designed.
  - `query-live/instance.svelte.js:184`. An `HttpError` on a `query.live`
    stream fails it hard, with no automatic retry, so `watchRecording` does
    not loop after expiry.
- Long-lived client connections in `web/src`: the run page's EventSource is
  the only one. `watchRecording` (`web/src/lib/transcript.remote.ts:50`) is the
  only `query.live`.
- `web/src/routes/+layout.svelte:1-52` and `web/src/routes/layout.server.go:8-29`
  are the root layout and its load. They render every route, including the
  proposed `/auth/login`.
- Surrounding code from earlier rounds, rechecked where the design refers to
  it:
  - `web/server.go`, `web/runtime.go` (`projectRequest`, `startWeb`,
    `GIMBAL_WEB_ORIGIN`), `web/control.go`, `cmd/gimbal/main.go:146-300`;
  - `web/src/routes/projects/[project]/runs/[runID]/+page.svelte:310-366`;
  - `web/src/lib/observation/index.ts:238-260`.

No implementation exists yet, so there was no implementation proof to inspect.

## Findings

### 1. Issue: the new account UI's `currentUser` query has no defined behavior on the anonymous sign-in page or with auth off, and the sign-in page renders inside the layout that would call it

The design introduces an application-owned `currentUser` skgo query that
"supplies the account UI with the explicit ID/name DTO" (`design.md:221-222`).
The UI shows sign-out and the current user's name (`design.md:363`).

The design does not give that query an exception. The only unauthenticated
remote is "the exact generated POST remote for beginLogin" (`design.md:191-195`),
so `currentUser` is protected and answers 401 to an anonymous request
(`design.md:208-209`). The design also says: "With auth off, the account UI
and this auth check are disabled" (`design.md:231-232`).

Two cases are left unspecified.

**The sign-in page is a child of the root layout.** It "shows a sign-in page
in the existing SvelteKit application" (`design.md:138`). SvelteKit always
applies the root `+layout.svelte`. Today that layout renders the site nav,
`main`, and `RecorderBridge` on every route (`web/src/routes/+layout.svelte:19-51`).
The design already says what the anonymous layout must not do: it does not
start `RecorderBridge` or project subscriptions (`design.md:196-199`). It does
not exclude the account UI it adds. Sign-out belongs in that shared chrome, so
an implementer following the text puts `currentUser` in the root layout. Then:

1. An anonymous browser opens `/auth/login`, either directly or through the
   303 at `design.md:202-205`.
2. SSR renders the root layout. The account UI awaits `currentUser()`. The
   in-process remote request carries no session, and the gate answers 401.
3. There is no boundary during SSR, so the 401 renders the error page instead
   of the sign-in page.
4. If the call happens only on the client, the "query error boundary" handling
   (`design.md:212-214`) catches the 401 and calls `goto('/auth/login')` from
   `/auth/login`. The page re-runs its loads, and the account UI either errors
   again or re-navigates.

Either way the one page an anonymous user needs, which an operator cannot
debug while signed out, is broken by the feature meant to show who is signed
in.

**The auth-off signal has no source.** "Disabled with auth off" needs the
client to know whether auth is on. No load, DTO field, or query result carries
that today: `WorkspaceLayout` holds only `desktop` and `workspaceProjects`
(`layout.server.go:8-11`). The design also never says what `currentUser`
returns when auth is off and no user is in context. If it returns 401, the run
page probe (`design.md:222-226`) sends auth-off users to a `/auth/login` route
that does not exist there. Auth-off is the default and the first acceptance
case (`design.md:368-369`).

**Fix:** say where the account UI gets its signed-in and auth-enabled state.
For example, the root layout load, which already runs for the anonymous page
before `projectRequest`, returns `authEnabled` and the current user's ID and
name, or null. The account UI renders only when a user is present. Keep
`currentUser` as the protected probe the EventSource owner calls, only when
auth is enabled. Alternatively, define `currentUser`'s anonymous and auth-off
results explicitly and state that the sign-in page never renders the account
UI. Add "the sign-in page renders while signed out" to the acceptance list.

## Outcome

material findings remain
