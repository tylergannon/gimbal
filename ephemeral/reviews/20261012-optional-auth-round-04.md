# Adversarial review: optional OAuth login design, round 04

## Target

`ephemeral/auth/design.md` at commit `5c1255c4` (branch
`codex/optional-auth-design`), reviewed in full against the authoritative
request in `ephemeral/auth/requirements.md` and the repository instructions in
`AGENTS.md`, including the Svelte rules in
`ephemeral/research/svelte-idioms/dos-and-donts.md`.

The launch prompt's limits were operating constraints only: a read-only review,
an artifact path, and the same authoritative sources. Nothing in it narrowed
the review's scope or asked for a particular verdict, so there was no
narrowing to ignore.

## Evidence inspected

- `ephemeral/auth/requirements.md`, `ephemeral/auth/design.md` (whole file),
  `ephemeral/auth/provider-references.md`.
- `git diff 67560e45 5c1255c4`.
- Round 03's findings and how this revision handles them:
  - Document, route-data, and remote refusals are now distinguished
    (`design.md:202-208`). The 401 handling for remote operations is placed
    in the UI owners (`design.md:210-217`).
  - Static assets are classified from the build manifest, with no per-page
    chunk graph (`design.md:200-201`).
  - Provisioning help gives `gh api users/LOGIN --jq .id` (`design.md:270-271`).
  - The acceptance list now includes client-side navigation and remote
    actions after expiry, cross-tab logout, or user deletion
    (`design.md:361-363`).
- I checked the new mechanisms against the SvelteKit 3.0.0 client in the pnpm
  store this app resolves:
  - `src/runtime/client/client.js:3597-3638,3669` (`load_data`, data-node
    redirect). A 200 `{type:"redirect",status,location}` data node is followed
    by the client. This matches skgo's `Loads.writeRedirect`
    (`skgo@v0.29.0/data.go:462-473`).
  - `src/runtime/client/remote-functions/shared.svelte.js:113-133`
    (`remote_request`). A bare non-OK response becomes
    `HttpError({status, message})`.
  - `src/exports/index.js:106-109`, `exports/internal/shared.js:3,24`.
    `isHttpError(e, 401)` matches both `HttpError` and `HandledHttpError`, so
    the design's handling of remote 401s works with the client as it is.
  - `remote-functions/command.svelte.js:55-69`. Commands reject a redirect
    envelope, so 401 plus a catch is the correct path for commands.
- `web/src/routes/projects/[project]/runs/[runID]/+page.svelte:310-366`: the
  run page's observation `EventSource` and its retry loop.
- `web/src/lib/observation/index.ts:238-260` (`retryConnection`,
  `connectionLost`).
- `web/src/lib/run/DetailPane.svelte:485,500` and
  `web/src/lib/run/detail/ContextList.svelte:66-68`: links to `/api/` file
  routes that open in a new tab.
- Remote call sites (11 files import a `*.remote` module) and
  `<svelte:boundary>` uses (4) under `web/src`, to size the per-owner 401
  handling. That scope is reasonable.

No implementation exists yet, so there was no implementation proof to inspect.

## Findings

### 1. Issue: after expiry, logout, or user deletion, the run page's observation stream reconnects every 250 ms forever and never reaches sign-in

The design requires the server to close every long-lived response, including
observation SSE, within 30 seconds of the session ending (`design.md:221-225`).
It also requires every protected request, including streams, to be refused
with 401 (`design.md:191-208`). The only client behavior it specifies for
rejected authentication covers "UI owners of remote operations", through
`isHttpError(error, 401)` (`design.md:210-217`). The run page's live view is
not a remote operation. It is a hand-built `EventSource` on the observation
route:

- `+page.svelte:339-341` opens
  `/projects/<id>/api/runs/<run>/events?...`.
- On `onerror` (`+page.svelte:349-357`), it closes the stream, asks
  `observation.connectionLost(generation)`, and if that returns true
  schedules `connect` again after 250 ms.
- `connectionLost` returns true whenever the run is still `running`
  (`web/src/lib/observation/index.ts:246-254`). `retryConnection` checks only
  that the run is running (`index.ts:238-240`). Nothing backs off, and the
  page has no way to tell an authentication refusal from a network drop:
  `EventSource` does not expose the HTTP status of a failed connection.

Causal sequence under the design as written:

1. A signed-in user has a running run's page open.
2. The session expires at eight hours (`design.md:165`), or the user logs out
   in another tab (`design.md:174-178`), or the operator runs
   `gimbal user remove` (`design.md:261-262`).
3. Within 30 seconds, the gate's recheck closes the SSE response
   (`design.md:221-225`).
4. `onerror` fires and the page schedules `connect` 250 ms later.
5. The new `EventSource` request reaches the gate and gets 401
   (`design.md:206-207`).
6. `onerror` fires again with the same outcome. The loop repeats for as long
   as the run is `running`. Agent runs often last hours.

Each open run tab then sends about four refused requests a second. Each one
costs the server a cookie hash and a `Users.Session` lookup. The page sits on
"disconnected" and never navigates to `/auth/login`.

This breaks the design's own acceptance criterion that navigation and actions
return "to sign-in after expiry, cross-tab logout or user deletion"
(`design.md:361-363`) on the page where people spend most of their time. The
design's own stream closure is what starts the loop.

**Fix:** specify the stream owner's behavior. One way: before the server
closes a stream for an authentication failure, it sends a terminal SSE event,
which the page handles with `goto('/auth/login')`, and the page stops
retrying. Covering a reconnect that starts after expiry also needs a signal
the page can read. For example, on `onerror` the page calls a cheap remote
`query`, which yields the 401 the design already handles. Add this stream to
the acceptance case.

### 2. Nitpick: `/api/` file links that open in a new tab get a bare 401 rather than the sign-in redirect

The design redirects "direct document navigation to a page" and answers "API
and remote requests" with 401 (`design.md:202-207`). The app links to
`/projects/<id>/api/runs/.../commands/stdout/...`, `.../artifacts/...`, and
`.../files/...` with `target="_blank"` (`web/src/lib/run/DetailPane.svelte:485,500`,
`web/src/lib/run/detail/ContextList.svelte:66-68`). After expiry, clicking
"Open complete stdout" opens a tab showing a raw 401.

**Fix:** have the gate redirect any top-level browser navigation
(`Sec-Fetch-Mode: navigate` / `Sec-Fetch-Dest: document`), whatever the path,
and return 401 for everything else. That would make the
document-versus-API split one header check rather than a path classification.

## Outcome

material findings remain
