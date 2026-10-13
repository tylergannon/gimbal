# Adversarial review: optional OAuth login design, round 01

## Target

`ephemeral/auth/design.md` at commit `57d13530` (branch
`codex/optional-auth-design`), reviewed against the authoritative request in
`ephemeral/auth/requirements.md` and the repository instructions in `AGENTS.md`.

The launch prompt's limits were operating constraints only: a read-only review
and an artifact path. Nothing in it narrowed the review's scope or asked for a
particular verdict, so there was no narrowing to ignore.

## Evidence inspected

- `ephemeral/auth/requirements.md`: Tyler's requirements. Auth should be
  "SUPER SIMPLE. no organizations. just users", supplied by a plugin and
  selected at runtime, GitHub first, with `User` as an interface, durable
  users, and application-owned OAuth endpoints.
- `ephemeral/auth/design.md` (all 285 lines), `ephemeral/auth/provider-references.md`,
  `ephemeral/worklog/optional-auth-design.md`.
- `AGENTS.md`: "As simple as possible", "A new exported name exists only when
  Tyler asks for it by name", "Build what was asked".
- `registry.go`, `backend_registration.go`, `messaging_registration.go`,
  `resource_registration.go`: the existing Registry and Definition patterns.
- `web/server.go`: the `NewHandler` composition, the skgo origin configuration,
  `explainOriginRefusals`, and the internal SSR fetch host.
- `web/runtime.go`: `NewInstance`, `startWeb`, the `GIMBAL_WEB_ORIGIN` origin
  derivation, and `projectRequest`.
- `web/control.go`: the control socket and its discovery file.
- `cmd/gimbal/main.go:243-300`: `serverFlags` and `serve`.
  `cmd/gimbal/secrets.go`: `~/.gimbal/config.json`. `cmd/gimbal/daemon.go`:
  the `--task-listen` listener. `cmd/gimbal/run_prompt.go:87` and
  `cmd/examples/interview/main.go:43`: other `NewInstance` callers.
- `run.go:383-404`, `session.go:682`, `supervise_jev.go:186,200`,
  `web/src/routes/control.remote.go:38,51`, `internal/host/routes.go:194`: the
  Steer `Source` and `Killed.By` attribution.
- `internal/host/host.go:71-74`: how project IDs are derived.
- skgo v0.29.0 `fetch.go:99-156`. This confirms that in-process `event.fetch`
  passes the page's cookies through, so the design's SSR-identity requirement
  can be met.
- `docs/web-app.md:155`: "Accounts, login, roles" is listed as deliberately
  not on the page.

No implementation exists yet. The design itself says it is a proposal, so
there was no proof to inspect.

## Findings

### 1. Critical: auth is turned on by a configuration the design never locates, and a missing configuration fails open

Requirements: "a way to lock down Gimbal that can be turned on" and "selected
at runtime" (`requirements.md:4-5,17`).

Design: "No auth configuration means authentication is off and no auth store
is opened" (`design.md:191`). The configuration is described only as
"Conceptual configuration (field casing may follow the existing config
format)" (`design.md:199`), and "The auth configuration is passed explicitly
into handler construction" (`design.md:259-260`).

The repository has no existing instance configuration format to follow:

- `serve` is driven entirely by flags (`cmd/gimbal/main.go:243-262,276-294`).
- The only config file, `~/.gimbal/config.json`, is per-HOME and holds only
  secrets and upload settings (`cmd/gimbal/secrets.go:44`,
  `cmd/gimbal/upload_artifact.go:158`).
- `web.NewInstance(ctx, dir, projects, opts...)` has no auth input
  (`web/runtime.go:99`). `startWeb` builds the handler with only
  `(dist, proxy, origin)` (`web/runtime.go:180`).

The design never says which file, flag, or `Option` carries the selection, who
reads it, or which commands honor it. `serve`, `run_prompt.go:87`,
`cmd/examples/interview/main.go:43`, and test hosts all call `NewInstance`.

Because absence means "off", every way of failing to find the configuration
leaves the server open, and each of these is ordinary:

- the file is in a different HOME than the one a service manager runs under;
- the path has a typo;
- a command or example never reads the file;
- `GIMBAL_WEB_ORIGIN` points a public proxy at the listener.

The operator believes the instance is locked down, and the full web stack
(pages, remotes, artifacts, steering, cancellation) is served anonymously.

The rest of the design is careful to fail closed ("selecting auth makes
failures deny access", `design.md:9`), but it never makes the selection itself
fail closed.

**Fix:** name the concrete selection surface and require enablement to be
explicit at the point the instance starts. For example, `serve` could take a
path that must exist and parse. State which `NewInstance` callers can serve
an authenticated web listener.

### 2. Issue: a second origin source contradicts the origin skgo already enforces

Design: `"origin": "https://gimbal.example.com"` in the auth configuration
(`design.md:215`), "an explicit trusted public origin" (`design.md:167-168`),
and "keep skgo's existing checks" (`design.md:181-182`).

The current code has a single origin. `startWeb` sets it to the TCP listener
address, or to empty for UDS, and `GIMBAL_WEB_ORIGIN` overrides it
(`web/runtime.go:167-173`). That one value configures skgo's remote, load,
endpoint, handle, and fetch configs (`web/server.go:41-48,142`) and
`explainOriginRefusals` (`web/server.go:153-176`).

The design adds `auth.origin` and never says how it relates to that value.
Reproduction from the design as written:

1. Configure `auth.origin = https://gimbal.example.com` behind a TLS proxy to
   `--port 8080`, without `GIMBAL_WEB_ORIGIN`.
2. skgo's origin is `http://127.0.0.1:8080`.
3. Every remote command and form POST from the browser carries
   `Origin: https://gimbal.example.com`.
4. `explainOriginRefusals` and skgo answer 403 even though auth admitted the
   session. Login works, but the app is unusable.

The other combinations fail too:

- If the two values are set differently, the auth check and the skgo check
  disagree about which mutations are same-origin.
- With `--uds`, skgo's origin is empty, so its checks are off and only the new
  auth check remains. That is not "keep skgo's existing checks".

**Fix:** keep one origin. Either `auth.origin` replaces `GIMBAL_WEB_ORIGIN`
and the listener-derived value when auth is on, or the design reuses
`GIMBAL_WEB_ORIGIN` and requires it to be an `https` origin.

### 3. Issue: the gate's placement is impossible in the current composition, and `projectRequest` answers anonymous requests first

Design: "The web gate belongs in the shared `web.NewHandler` composition,
outside `observation.Routes`, and before the project request handler can
disclose project data" (`design.md:255-257`). It also requires that
"Unauthenticated requests never start application work" (`design.md:187`).

In the code, `projectRequest` wraps the handler that `NewHandler` returns, and
it lives in `startWeb`: `Handler: i.projectRequest(handler)`
(`web/runtime.go:187`). A gate anywhere inside `NewHandler` therefore runs
after `projectRequest`. For anonymous requests, `projectRequest`:

- attaches the instance `Owner` and the full project-choice list to the
  context;
- answers `404` for an unknown `/projects/<id>` path, for a non-shared remote
  without a matching Referer, and for `/api/`
  (`web/runtime.go:232-261`);
- passes requests for admitted projects through to the gate, which answers 401
  or redirects.

A project ID is `hex(sha256(path)[:16])` (`internal/host/host.go:71-74`). An
anonymous client can confirm whether a guessed checkout path, such as
`/home/alice/src/gimbal`, is admitted by comparing 404 with 401.

The tests the design relies on for "the binary and tests exercise the same
assembly" call `NewHandler` without `projectRequest`, so they cannot see this
ordering.

**Fix:** state where the gate actually goes. Either it wraps
`projectRequest` in `startWeb`, or `projectRequest` moves into `NewHandler`
behind the gate. Say how the shared-assembly guarantee survives that choice.

### 4. Issue (over-engineering): admission lives in a second static user list, which turns "just users" into two registries and drives most of the contract's complexity

Requirements: "SUPER SIMPLE. no organizations. just users." The plugin
provides "basically that gamut across the needed operations in order to
support oauth" (`requirements.md:9,19-21`). `AGENTS.md` adds "As simple as
possible" and "A new exported name exists only when Tyler asks for it by
name."

The design keeps two lists of users:

- the durable `Users` store that Tyler asked for (`design.md:54-60`);
- a separate `allowedUsers` list of issuer/subject pairs in static
  configuration, changed by editing and restarting (`design.md:195-197,216-218`).

Admission is decided only by the config list. The customer's `Users`
implementation, which Tyler wanted to define, cannot admit or refuse anyone.
Users removed from the list stay in the store as dead records, and adding a
user takes a config edit and a restart.

The second list forces further mechanisms:

- `Session` must carry an `IdentityKey` (`design.md:47-52`).
- `CreateSession` must cross-check that the identity belongs to the user
  (`design.md:77-78`).
- Every request must recheck admission against the list (`design.md:156-157`).
- A separate "valid session but unlisted" 403 path is needed
  (`design.md:177-178`).
- Streams recheck admission every 30 seconds (`design.md:184-186`).

The design also adds other unrequested machinery:

- a pending flow that records a "selected provider", although exactly one
  provider is configured (`design.md:140-141,204`);
- a hand-written login page outside the SvelteKit app (`design.md:175-176`);
- `IdentityKey`, `Authorization`, and `Callback` as exported types where plain
  parameters would do (`design.md:37-45,100-111`);
- control-socket permission hardening (`design.md:264-265`), which is a
  separate change from browser login.

**Fix:** let the `Users` plugin own admission, for example by having
`CreateUser`/lookup return a refusal for identities it does not admit. Then
drop the config list, the identity carried in sessions, and the per-request
admission recheck. Defer the extras until a deployment needs them.

### 5. Issue (unrequested, regresses a contract): replacing `"person"` with the local user ID erases the operator/supervisor distinction

Design: "Browser actions use the authenticated local user ID for the existing
By/source attribution instead of the literal 'person'" (`design.md:271-272`).

Nothing in `requirements.md` asks for this. `"person"` is a documented value
in the root API, not a placeholder. `run.Steer` says: "the Steer record
carries Source 'person', which tells an operator's steer from a supervisor's"
(`run.go:383-385,404`). A supervisor steer's Source is the supervising
session's ID (`supervise_jev.go:186,200`).

Local user IDs are opaque store-chosen strings, so after this change an
operator steer and a supervisor steer are both arbitrary IDs. Durable run
records and the UI lose the ability to tell them apart, and existing consumers
that compare against `"person"` break: `web/control_test.go:52,60,91`,
`web/registry_test.go:200`, `web/loop_steer_test.go:192`,
`web/src/lib/run/CancellationStatus.svelte.test.ts:8`.

The change also needs new plumbing through the root `gimbal` API. `run.Steer`
has no source parameter, and `Killed.By` is set in
`web/src/routes/control.remote.go:38,51` and carried over the backend control
protocol (`internal/host/routes.go:194`, `internal/runner/runner.go:184`).
That expands the public API, which `AGENTS.md` forbids without Tyler naming it.

**Fix:** drop the item. If attribution is wanted later, it should be a
separate request that keeps an operator-versus-supervisor field.

## Outcome

material findings remain
