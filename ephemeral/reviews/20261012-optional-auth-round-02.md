# Adversarial review: optional OAuth login design, round 02

## Target

`ephemeral/auth/design.md` at commit `91a0e55e` (branch
`codex/optional-auth-design`), reviewed in full against the authoritative
request in `ephemeral/auth/requirements.md` and the repository instructions in
`AGENTS.md`, including the Svelte rules in
`ephemeral/research/svelte-idioms/dos-and-donts.md`, which `AGENTS.md` names.

The launch prompt's limits were operating constraints only: a read-only review,
an artifact path, and the same authoritative sources. Nothing in it narrowed
the review's scope or asked for a particular verdict, so there was no
narrowing to ignore.

## Evidence inspected

- `ephemeral/auth/requirements.md`, `ephemeral/auth/design.md` (all 325
  lines), `ephemeral/auth/provider-references.md`, and
  `git diff 57d13530 91a0e55e`.
- Round 01 review, `ephemeral/reviews/20261012-optional-auth-round-01.md`.
  All five of its findings are addressed in the text:
  - the selection is now an explicit `--auth-file` that fails closed;
  - there is a single origin, and a conflicting `GIMBAL_WEB_ORIGIN` fails
    startup;
  - the gate now runs before `projectRequest`, all inside `NewHandler`;
  - the `Users` store is the only user registry;
  - the `"person"` attribution is kept.
- `cmd/gimbal/main.go:146-182,243-300`: the root command, `serverFlags`, and
  `serve`.
- `web/runtime.go`, `web/server.go`, `web/control.go:150-260` (control socket
  and `controlMux`).
- `web/src/routes/+layout.svelte`, `web/src/routes/layout.server.go`: the root
  layout every SvelteKit page inherits, and its project-list load.
- `web/src/lib/workspace/RecorderBridge.svelte`,
  `web/src/lib/transcript.remote.go:91-121`: the only `skgo.LiveQuery`.
- `internal/observation/http.go:21-30,90`: the observation routes and the SSE
  stream.
- `internal/host/host.go:71-74`.
- `ephemeral/research/svelte-idioms/dos-and-donts.md:7-14,36-40,171-177,259-269`.
- I built the binary at HEAD and ran `gimbal serve --port 0 --instance-dir
  /tmp/...` (reproduction under finding 1). The binary and the instance
  directory were deleted afterwards.

No implementation exists yet, so there was no implementation proof to inspect.

## Findings

### 1. Issue: the "concrete selection surface" names a command that does not exist

Design: "The concrete selection surface is `gimbal serve --auth-file
/absolute/auth.json`" (`design.md:199`). It adds: "The flag is added to the
common serverFlags path so each command using those flags honors it"
(`design.md:209-210`), and "The serve command must start its optional
--task-listen listener only after NewInstance has resolved auth"
(`design.md:297-298`).

Gimbal has no `serve` subcommand. The root command serves: "Without a
subcommand, gimbal serves the current repository" (`cmd/gimbal/main.go:151-152`,
with `RunE` calling `serve` at `main.go:165-167`). `serverFlags` is bound only
on the root (`main.go:172`), so no other command shares it.

Reproduction at `91a0e55e`:

```
$ go build -o /tmp/gimbal-review ./cmd/gimbal
$ /tmp/gimbal-review serve --port 0 --instance-dir /tmp/gimbal-review-inst
gimbal: unknown command "serve" for "gimbal"
```

The flag is the security switch, and the design tells operators and the
command help to use exactly this invocation. Following it errors before auth
is ever selected. The "each command using those flags" sentence also promises
coverage across commands that does not exist. That matters because
`run-prompt` and the examples start instances through their own paths
(`cmd/gimbal/run_prompt.go:87`, `cmd/examples/interview/main.go:43`).

**Fix:** name the real surface (`gimbal --auth-file ...` on the root
command) and drop the claim about shared flags.

### 2. Issue: the bounded pending-flow map lets one anonymous client lock everyone out of login

Design: "Bound the pending-flow map to 1024 entries and reject new attempts
when full rather than growing without limit" (`design.md:145-147`). The flows
live for five minutes (`design.md:141`). POST `/auth/login` is
unauthenticated by design (`design.md:180-181`). Its only guard is the Origin
check (`design.md:139-140,189`), which only constrains browsers: a
non-browser client sets any Origin header it likes. "a new attempt replaces
the browser's prior attempt" (`design.md:145`) is keyed on the flow cookie,
and an attacker simply omits it.

Reproduction against the design as written:

1. An anonymous client sends 1024 requests of
   `curl -X POST -H 'Origin: https://gimbal.example.com' https://gimbal.example.com/auth/login`,
   with no cookie.
2. Each request creates a new pending entry, and the map is full.
3. For the next five minutes every legitimate POST `/auth/login` is rejected.
4. Repeating step 1 every five minutes keeps login unavailable indefinitely.

Existing sessions keep working, but nobody can sign in again. That includes
an operator whose eight-hour session has expired, and anyone just
provisioned with `gimbal user add`.

The cap trades unbounded memory for a remote lockout. The design also adds
in-process state and a sizing policy that the flow does not need. State,
verifier, nonce, return path, and expiry are small. They can travel in the
HttpOnly, `__Host-`, five-minute flow cookie itself, authenticated with a
per-process random key. That makes "a restart invalidates them" automatic and
leaves no server-side map to fill.

**Fix:** use a stateless, MAC'd flow cookie instead of a capped map, or at
minimum say how the cap avoids becoming a lockout.

### 3. Issue: login and logout are bespoke POST endpoints that the app's own pages call, which the owner's Svelte rules forbid

Tyler's hard rules, which `AGENTS.md` requires reading before any `web/`
change: "Real forms for real forms, `command` for everything else ... Every
other mutation (stop, cancel, button actions with no fields) is a `command`"
(`dos-and-donts.md:12`). The same file says: "every page-to-server call is a
remote function ... a bespoke endpoint gives none and invites a hand-built
client" (`dos-and-donts.md:36`). It names Go `HandleFunc` routes that the
browser calls as the anti-pattern to grep for (`dos-and-donts.md:38-40`).

The design does exactly that:

- The sign-in page lives "in the existing SvelteKit application"
  (`design.md:138`), but the page posts to a Gimbal-defined
  `POST /auth/login` (`design.md:139-140`).
- The app's sign-out control posts to `POST /auth/logout`
  (`design.md:165-168,309`).
- Both are button actions with no fields, so the rule makes both `command`s.

Because they are bespoke endpoints, the design has to re-specify protections
that remote functions already provide:

- "Check the expected Origin on all cookie-authenticated unsafe requests,
  including login and logout" (`design.md:189`), which skgo's remote origin
  check already does (`web/server.go:41,153-176`);
- separate 401/503 contracts for these routes (`design.md:166-168,185-187`).

Only GET `/auth/callback` has to be a route of its own, because GitHub
navigates the browser to it.

**Fix:** make login initiation and logout skgo `command`s. Login initiation
sets the flow cookie and returns the provider URL for a top-level navigation;
logout deletes the session and expires the cookie. Keep only the callback as a
route. If the design deliberately departs from the rule, it has to say so and
justify it. It does neither.

### 4. Nitpick: the public and long-lived surfaces are listed incompletely

- **Login route data.** The exceptions are "the login page, login initiation,
  callback, and manifest-listed static build assets" (`design.md:180-184`). A
  SvelteKit page reached by client-side navigation also fetches its route
  data, `/auth/login/__data.json`. Every page also runs the root layout load
  (`web/src/routes/+layout.svelte`, `web/src/routes/layout.server.go:13-27`),
  which returns every admitted project's absolute path when an `Owner` is in
  context.
  - The design's claim that "the login layout reads no project data"
    (`design.md:182`) holds only because the gate dispatches the login page
    before `projectRequest` attaches the Owner (`web/runtime.go:233`). The
    design should say that this ordering is what keeps project paths out of
    the anonymous page, and list the route-data request among the
    exceptions.
- **Stream rechecks.** The 30-second recheck covers only "Active observation
  streams" (`design.md:192-194`). The app also has a long-lived
  `skgo.LiveQuery`, `watchRecording` (`web/src/lib/transcript.remote.go:91-121`).
  The design should state that the recheck covers every long-lived response,
  not just the observation SSE (`internal/observation/http.go:90`).

### 5. Nitpick: `--task-listen` remains an unauthenticated command-execution listener in the same process

The design states the boundary honestly (`design.md:303-307`), and the new
ordering requirement (`design.md:297-298`) matches the current code
(`cmd/gimbal/main.go:276-285`). Still, with `--auth-file` selected,
`--task-listen 0.0.0.0:9000` is accepted, and the design's own words describe
that listener as executing "trusted commands" (`design.md:304`). An operator
who has just "turned on" lockdown gets no signal that the same process serves
an open execution endpoint.

**Fix:** have command help, or a startup log line when both are selected, name
the listener as outside auth.

## Outcome

material findings remain
