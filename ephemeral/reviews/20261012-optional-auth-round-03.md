# Adversarial review: optional OAuth login design, round 03

## Target

`ephemeral/auth/design.md` at commit `67560e45` (branch
`codex/optional-auth-design`), reviewed in full against the authoritative
request in `ephemeral/auth/requirements.md` and the repository instructions in
`AGENTS.md`, including the Svelte rules in
`ephemeral/research/svelte-idioms/dos-and-donts.md`.

The launch prompt's limits were operating constraints only: a read-only review,
an artifact path, and the same authoritative sources. Nothing in it narrowed
the review's scope or asked for a particular verdict, so there was no
narrowing to ignore.

## Evidence inspected

- `ephemeral/auth/requirements.md`, `ephemeral/auth/design.md` (all 349
  lines), `ephemeral/auth/provider-references.md`, including the new RFC 6749
  §4.1.2 note.
- `git diff 91a0e55e 67560e45`, and the worklog entries for rounds 01 and 02.
- Round 02 findings and how this revision handles them:
  - Root-command `--auth-file` (`design.md:218-230`) matches
    `cmd/gimbal/main.go:146-172`, where `serverFlags` is bound only on the root.
    The note that `run-prompt` uses `WithNoWeb` matches
    `cmd/gimbal/run_prompt.go:87`.
  - The capped pending-flow map is replaced by an AEAD flow cookie with a
    per-process key (`design.md:139-148`). There is no server state left to
    exhaust.
  - Login initiation and logout are now skgo `command`s, and only the provider
    callback is a server route (`design.md:138-178`). That matches
    `dos-and-donts.md:12,36`.
  - The public surface now names the route-data request and the exact
    `beginLogin` remote. Long-lived rechecks cover observation SSE and
    `watchRecording` (`design.md:191-214`).
  - The task listener is named at startup (`design.md:330-331`).
- skgo v0.29.0:
  - `event.go:106-160`: only a `command` may set or delete cookies. An empty
    `Domain` writes no `Domain` attribute (`event.go:397-398`), so the
    `__Host-` cookies the design requires can be issued from `beginLogin` and
    `logout`.
  - `data.go:20-23,38-61,470`: `__data.json` handling and Kit's
    `{"type":"redirect"}` data response.
  - `fetch.go:99-156`: in-process SSR fetch passes the page's cookies through.
- SvelteKit 3.0.0 client, `src/runtime/client/client.js:3597-3638`
  (`load_data`), from the pnpm store this app resolves.
- `web/server.go`, `web/runtime.go`, `web/control.go:150-260`,
  `web/src/routes/+layout.svelte`, `web/src/routes/layout.server.go`,
  `web/src/lib/workspace/RecorderBridge.svelte`,
  `web/src/lib/transcript.remote.go:91-121`, `internal/observation/http.go:21-30`.

No implementation exists yet, so there was no implementation proof to inspect.

## Findings

### 1. Issue: an expired or revoked session turns in-app navigation into an error page, not the login redirect the design promises

Design: "Unauthenticated page navigation redirects to login; API and remote
requests return 401" (`design.md:202`). It also says expired sessions, logout,
and user deletion all end access (`design.md:79-80,164-178,203`).

Most navigation in this app is client-side. A click within the hydrated
SvelteKit app does not request a document. It fetches the target route's
`__data.json`, which skgo answers inside the kit stack (`skgo data.go:38-61`).
The SvelteKit 3.0.0 client handles that response in `load_data`
(`client.js:3613-3635`):

- If the response is not `ok`, it throws `HandledHttpError` and renders the
  error page (`client.js:3618-3631`).
- If the response is `ok`, it parses the body as Kit's NDJSON node stream
  (`client.js:3633-3635`). It has no branch for an HTTP redirect.

The design gives the gate only two answers. Both fail for a data request:

- **The gate answers 401.** That is the "API" branch, and the client renders
  "Internal Error" with status 401.
- **The gate answers 302 to `/auth/login`.** That is the "page navigation"
  branch. `fetch` follows the redirect to the login HTML with status 200, and
  `process_stream` fails parsing HTML as the data stream.

Reproduction from the design as written:

1. Sign in.
2. Leave a project tab open past the eight-hour absolute expiry
   (`design.md:165`). Alternatively, log out in a second tab, or have the
   operator run `gimbal user remove`.
3. Click any in-app link, such as Runs or Conversations, in the first tab.

The tab shows the error page instead of the login page, and the user has to
type `/auth/login` by hand. Every signed-in user hits this at least once every
eight hours. Remote calls from an open page fail the same way, as bare 401
errors with no path to sign in (`design.md:202`).

The transport already has the right answer. Kit's data protocol carries
redirects as a data node, `{"type":"redirect","location":...}`, which skgo
already emits for a load that redirects (`skgo data.go:470`). The client then
performs the navigation.

**Fix:** state that the gate answers unauthenticated `__data.json` requests
with Kit's data-redirect response to `/auth/login`, or lets a root load
redirect. Say how a remote 401 sends the page to `/auth/login`. Add
in-app navigation after expiry to the acceptance list (`design.md:338-346`).

### 2. Nitpick: per-page "manifest-listed static build assets used by the login page" is more precise than it needs to be, and fragile

`design.md:194-195,200-201` exempts only the exact build assets the login page
uses, and correctly forbids exempting all of `/_app/` because remotes live
under `/_app/remote/`.

The login page is a route inside the full app. It inherits the root layout
(`web/src/routes/+layout.svelte`), its CSS, its favicon import, and its chunk
graph. Deriving "used by the login page" from the manifest means tracking
layout imports, CSS-referenced assets, and dynamic imports. Missing one
silently breaks the sign-in page, which is the one page an operator cannot
debug while signed out.

The immutable build output under `/_app/immutable/`, plus `version.json`, is
the same open-source bundle for every Gimbal instance and holds no instance
data. Exempting exactly that directory is simpler and still excludes the
remote endpoints and every project route.

### 3. Nitpick: `gimbal user add --subject 123456` needs a value the operator does not normally know

`design.md:248-249` provisions users by GitHub's numeric account ID, which is
correct for identity stability (`design.md:122-124`;
`provider-references.md`, GitHub `/user`). Operators know logins, not IDs.

Command help should say how to get the ID, for example
`gh api users/<login> --jq .id`. Otherwise the first provisioning attempt is
guesswork, and a wrong subject fails silently at login with a 403
(`design.md:161-163`).

## Outcome

material findings remain
