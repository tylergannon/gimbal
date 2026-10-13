# Adversarial review: optional OAuth login design, round 07

## Target

`ephemeral/auth/design.md` at commit `fe0e6edc` (branch
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

- `ephemeral/auth/design.md` (whole file) and `git diff b904137f fe0e6edc`.
- How this revision handles round 06's finding:
  - The root layout load now calls `event.RouteID()` in both auth modes, so
    any navigation into the login route reruns it and replaces the old user
    summary with null (`design.md:229-234`).
  - After a successful logout, the logout owner calls `goto('/auth/login')`
    (`design.md:178-179`).
- Whether that mechanism works:
  - skgo v0.29.0 `loadevent.go:185-195`. `RouteID()` sets the node's
    `uses.route`.
  - `document_assemble.go:349-353`. Server-rendered hydration data carries
    each node's `uses`.
  - `data.go:455-459`. `__data.json` nodes carry each node's `uses`.
  - The client therefore knows of the root layout's route dependency from the
    first page load onward.
  - SvelteKit 3.0.0 `runtime/client/client.js:1286-1300`. `has_changed`
    returns true when `uses.route` is set and the route changed.
  - `client.js:1464-1480`. The root node is marked invalid and requested
    from the server.
  - `client.js:2026-2040`. A data-redirect envelope navigates to
    `/auth/login` from the previous route, so `route_changed` holds on that
    path too.
  - The login route's data request is public and dispatched before
    `projectRequest` (`design.md:193-199`). Its rerun layout load returns
    `user: null`, which removes the account UI.
  - Navigation between two pages of the same route, such as one run to
    another, does not rerun the load. That does not affect auth correctness:
    protected requests are still refused at the gate.
- The cost is one root layout rerun per route change. That load only lists
  admitted projects (`web/src/routes/layout.server.go:13-27`), which is cheap.
  The design adds no auth-state mirror or polling (`design.md:233-234`).
- The rest of the design, re-read in full: storage and provider contracts,
  the callback and session lifecycle, gate exceptions and refusal shapes,
  single origin, root-command `--auth-file`, control-socket provisioning, the
  file-backed store, integration order, the task-listener boundary, and
  acceptance.
- Findings from rounds 01-06, checked against the current text. All remain
  resolved:
  - fail-closed explicit selection;
  - one origin;
  - gate before `projectRequest`, inside `NewHandler`;
  - one user registry;
  - `"person"` attribution kept;
  - the real root command;
  - stateless AEAD flow cookie;
  - skgo commands for login and logout;
  - data-redirect envelope;
  - `Sec-Fetch` document redirect;
  - EventSource probe through `currentUser`;
  - anonymous and auth-off layout state;
  - layout refresh on navigation to login.

No implementation exists yet, so there was no implementation proof to inspect.

## Findings

No material findings remain. The design meets each requirement in
`requirements.md`:

- auth that can be turned on, selected at runtime;
- plugins for the provider and for users;
- GitHub first;
- `User` as an interface;
- Gimbal owning the OAuth endpoints;
- durable users without organizations.

It follows the repository rules on remotes, navigation, and new names. Each
mechanism I checked against the vendored skgo and SvelteKit sources behaves as
the design states. I found no nitpick worth reporting.

## Outcome

no findings
