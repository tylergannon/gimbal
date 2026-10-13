# Gimbal

**Agent workflows that read like pseudocode.** Gimbal is a Go runtime for
multi-agent workflows: write the workflow as a plain Go function, and Gimbal
gives it scoped context, planner loops, supervision, and a live console, on
Codex, Claude Code, Antigravity, OpenCode, or your own harness.

[![The Gimbal console showing a planner loop with a watched coding turn](https://gimbal-view.fly.dev/shots/console.png)](https://gimbal-view.fly.dev/)

- **[Live multi-agent console](https://gimbal-view.fly.dev/docs/console/)** — watch every scope, turn, and command on one map; steer an agent, answer its questions, or stop it.
- **[Workflows that read like pseudocode](https://gimbal-view.fly.dev/docs/workflows/)** — the whole process is one Go function, with prompts visible at the call site.
- **[Primitives for agent work](https://gimbal-view.fly.dev/docs/primitives/)** — scoped context delivery, promise loops, supervision, and a graph read from your source.
- **[Curated roles, bound to models](https://gimbal-view.fly.dev/docs/roles/)** — workflows name kinds of cognitive work; a run binds each to a harness, model, and effort.
- **[Built-in workflows](https://gimbal-view.fly.dev/docs/built-in/)** — `implement`, `review`, `research-document`, and `pyramid-summary` from the command line.
- **[Any harness](https://gimbal-view.fly.dev/docs/harnesses/)** — Codex, Claude Code, Antigravity, and OpenCode adapters ship in the box; five methods add another.

Start with the [quickstart](https://gimbal-view.fly.dev/docs/quickstart/).
The public programming contract is the root package's Godoc and compiling
examples:

```sh
go doc -all github.com/tylergannon/gimbal
```

Adapters own connection maintenance and reconnection. They make bounded, observable, responsible best-effort attempts to finish each requested turn, reconciling native state before continuing so a lost response does not duplicate work. Cancellation remains cancellation, and failed remote cleanup retains stop control. The workflow owns task policy and independent validation.

The persistent instance starts the SvelteKit web application automatically.
It listens on loopback port 8080 by default; `web.WithPort`, `web.WithUDS`, and
`web.WithNoWeb` select another listener shape. Node is a build-time dependency
only.

## Optional GitHub login

Authentication is off until the serving instance selects `--auth-file` (or
`web.WithAuthFile`). Build the provider and durable user-store plugins with the
same Go toolchain and source as the host:

```sh
mkdir -p /absolute/plugins
go build -buildmode=plugin -o /absolute/plugins/auth-github.so ./plugins/auth-github
go build -buildmode=plugin -o /absolute/plugins/auth-files.so ./plugins/auth-files
```

Register a GitHub App with no repository or organization permissions, disable
webhooks, and set its callback to the configured origin plus `/auth/callback`.
Keep its client secret in the environment named by `clientSecretEnv`. An auth
file selects the two plugins independently:

```json
{
  "origin": "http://localhost:8096",
  "provider": {
    "name": "github",
    "configuration": { "clientID": "YOUR_APP_CLIENT_ID", "clientSecretEnv": "GIMBAL_GITHUB_CLIENT_SECRET" }
  },
  "users": {
    "name": "files",
    "configuration": { "directory": "/absolute/instance/auth" }
  }
}
```

The file store initializes a new directory beneath an existing parent. Existing
missing or malformed state fails startup. HTTP is allowed for an explicit
localhost or loopback origin; other origins require HTTPS. Start the instance,
then provision the exact numeric GitHub identity through its local control
socket:

```sh
gimbal --port 8096 --instance-dir /absolute/instance --plugin-dir /absolute/plugins --auth-file /absolute/auth.json
# In another terminal; obtain the numeric subject with gh api users/LOGIN --jq .id:
gimbal user add --instance-dir /absolute/instance --issuer https://github.com --subject 123456 --name Tyler
```

Login never creates users. An empty store admits nobody. `gimbal user remove
--instance-dir /absolute/instance USER_ID` revokes access. Sessions survive
restart, expire after eight hours, and are revoked on sign-out. Active streams
recheck within 30 seconds. The separate task listener and local Unix control
socket remain trusted operator transports; publish only the authenticated web
listener. Authentication applies to the explicitly selected instance.

## Build and run

This project uses Justfile for its build commands. It requires Go 1.27, Node 24,
pnpm 12.10.1, and just. Set `TYPESAFE_API_KEY` only when using `WithSupervisor`
for Jev supervision. Instance startup, `run-prompt`, and workflows without Jev
work without it. Jev screens completed exposed thinking for attached supervisors; timed supervision
and `WithInterval` have been removed. Providers without completed thinking
events do not trigger automatic reviews.

From a source checkout:

```sh
just build
./bin/gimbal
```

`just build` runs Go generation and the skgo/SvelteKit production build before
compiling the CLI. It checks for `web/build/skgo.manifest.json` and embeds
`web/build/` directly with Go's `embed.FS`. Generated frontend files stay local;
run this checkout's binary as `./bin/gimbal`. A bare versioned `go install`
does not build or include the console. Install published releases from the
Homebrew tap in the
[distribution instructions](skills/gimbal-release/references/distribution.md).

`gimbal --version` reports the release version, commit and build date. Release
maintainers use GoReleaser for CLI archives and Apple's signing/notarization
tools for the desktop app; see the
[distribution instructions](skills/gimbal-release/references/distribution.md).

The macOS app starts its bundled Gimbal CLI as a managed local server by
default. Its central project catalog and instance control state live in
`~/.gimbal/desktop`; each project's runs and conversations remain under that
project's `.gimbal` directory. Set `GIMBAL_INSTANCE_DIR` before launching the app
to keep that state somewhere else, for example to run a test catalog beside your
own. Set `GIMBAL_SERVER_URL` before launching the app
to proxy an external server instead. In that mode, the app does not start or
manage the external server.

To build a local Debug app, stage the CLI produced by `just build` as a native
resource before generating the Xcode project:

```sh
just build
mkdir -p native/Resources
install -m 755 bin/gimbal native/Resources/gimbal
go tool skgo native build --platform macos --preset real
```

The red close button hides the app and leaves its managed server running.
Command-Q stops the app and its bundled server. The release script builds and
stages the exact-version CLI itself.

Then open http://127.0.0.1:8080. The page's heading, the Go version below it, and the
greeting counter all come from `web/src/routes/hello.remote.go` — the
`hello.remote.ts` beside it is generated and every one of its bodies throws, so
anything that renders is proof that Go answered.

For development, use one command to start the Go application and Vite:

```sh
just dev
```

## Run a workflow

Start an instance, then use the same build of the binary to submit a workflow
compiled into it:

```sh
./bin/gimbal --instance-dir /tmp/gimbal-instance --project /absolute/project
./bin/gimbal run --help
./bin/gimbal run <workflow> --help
./bin/gimbal run review --instance-dir /tmp/gimbal-instance --project /absolute/project --input '{"goal":"Review the current changes"}' --follow
```

Run the first command in its own terminal. A start command admits its `--project`
repository on first use, even if the instance did not open it at startup;
`--instance-dir` holds instance control and discovery, separately from the
project's `.gimbal/runs` and conversation files. Repeat `--project` to admit
more repositories at startup. Generated subcommands submit JSON to the daemon.
Use `--input` for a JSON parameter object, or the named parameter flags. Role
flags select models; omitted optional parameters retain workflow defaults.
A project has one active instance owner; a
second instance refuses it, including through a path alias. Different projects
can run on independently configured instances, and a project can reopen after
its owner exits. `--work-dir` controls execution independently of the owning
project and defaults to that project. `--follow` waits for the
terminal result. Each run executes ordinary Go in an independent child process
using the daemon startup PATH, executables, and provider configuration. Stopping
or restarting the daemon leaves runs executing; reopen the project to observe,
steer, or cancel the same run. The UI observes and controls runs; start them
through the CLI. The CLI
does not send or execute a Go closure. A server build without that workflow
cannot run it.

The binary includes the `implement`, `review`, `validate-product`,
`research-document`, and `pyramid-summary` workflows. Each role's model defaults from
`internal/builtin/defaults.json`; pass the role's flag, such as `--code-review`, to
override it.

Select the legacy OpenCode adapter with `opencode/<model-id>`, or name an
explicit OpenCode provider with `opencode/<provider>/<model-id>`. The same
forms work with `gimbal run-prompt --model` and every workflow role-model flag:

```sh
gimbal run-prompt --model opencode/ling-3.0-flash-fin-free "Reply exactly OK"
gimbal run review --code-review opencode/opencode/ling-3.0-flash-fin-free --goal "Review the current changes."
```

The shared server starts on first use and remains running when a session
closes. `gimbal opencode start` is idempotent; `gimbal opencode stop` stops the
server and interrupts its active work. Runtime state defaults to
`~/.gimbal/opencode`; set `GIMBAL_OPENCODE_DIR` for another default or pass
`--state-dir` to those lifecycle commands. Raw request, result, and SSE capture
files are under `<state-dir>/captures/` for correlating native events with
completed turns.

Use Pi with `pi/diffusion/<model-id>` in `gimbal run-prompt --model` or a
workflow role flag, for example `pi/diffusion/deepseek-4.1-flash-background`.
Set `DIFFUSION_API_KEY` in the Gimbal server environment. The owned Go Pi
engine runs inside Gimbal; no Pi, Bun, or Node installation is required for
the harness. Shell tools still launch the commands requested by the agent.
List the chat models with tools available to your Router key, then prefix a
chosen open-weight ID with `pi/diffusion/`:

```sh
curl -fsS https://router.diffusion.io/v1/models \
  -H "Authorization: Bearer ${DIFFUSION_API_KEY}" |
  jq -r '.data[] | select(.capabilities.openai_chat == true and .capabilities.tools == true) | .id'
```

Workflow roles name cognitive work, not positions in a workflow. Gimbal's
prescribed `WorkflowRole` constants and their descriptions live together in
`roles.go`; applications may define additional typed constants when they need
a role the catalog does not provide.

`go generate ./...` regenerates workflow graphs, the stock JSON input adapters and
CLI commands selected in `internal/builtin/workflows.go`, and the SKGO browser
and Go bindings. `internal/generate/gimbalgen` emits workflow graph metadata;
`internal/generate/stockgen` emits the built-in start entrypoints from one
selection.

The generated file registers the workflow graph under the run's workflow name.
Each run saves that graph with its own state. A later daemon build reads the saved
graph, so it cannot change the structure displayed for an existing run. Regenerate
and rebuild after changing a workflow's shape, before starting a new run.

The supported built-in authoring path is in this Gimbal checkout. Add a package
under `internal/workflows/`, using `internal/workflows/review/` as the small
example. Give it an entry such as `func Name(ctx context.Context, env
gimbal.Env, params NameParams) error` and a `//go:generate` directive for
`gimbalgen`. For structured result types, add a `//go:build jsonschema` file
and Polytype generation as the review package does. Add the built-in to
`internal/builtin/workflows.go`; `go generate ./...` produces its graph, typed
JSON input adapter and CLI command from that selection. The child calls the
workflow through that adapter. Rebuild and restart the instance with that binary.
Each run saves its own graph for subsequent display.
An unknown role is required on the command line when
`internal/builtin/defaults.json` has no default for it. The stock generator uses Gimbal internal packages and the application web build.
Submitting an arbitrary closure to an existing instance remains unsupported.

Standalone `gimbal.Run(gimbal.Project(ctx, dir), ...)` executes in its caller's
process and writes durable state under `dir/runs`; it does not join a running
instance's live registry or controls. `gimbal run-prompt` runs its own headless
runtime in the CLI process. Its logs default to a temporary directory; `--logs`
must name a fresh, empty directory. Both inherit the caller's environment,
not the hosted instance's environment. Keep these directories separate from
projects admitted to a running instance: concurrent shared-state use is not a
supported contract. Their records are local durable state, not live hosted
runs, even while an instance happens to be running elsewhere.

Use `Iterate(ctx, name, items)` to give each item in a finite slice its own
scope. Use `PromiseLoop(ctx, name, goal, planner)` and range over its `Tasks`
when a planner chooses work adaptively; check `Err()` afterward.

## Lint workflows

The distributed `gimbal` binary also checks deterministic workflow authoring
mistakes:

```sh
./bin/gimbal lint ./...
go vet -vettool="$(pwd)/bin/gimbal" ./...
```

The second command is the direct Go vet-tool protocol path. A custom vet tool
replaces Go's ordinary analyzers for that invocation, so `just vet` first runs
ordinary `go vet ./...`, builds the same `bin/gimbal`, and then runs
`bin/gimbal lint ./...`.

Read the [lint rule reference](internal/gimballint/rules.md) for the accepted
rules, diagnostics, rewrites, and analysis limits. The same text is printed by
`gimbal lint -help` and is available at [docs/lint-rules.md](docs/lint-rules.md).

The runtime derives the public origin from the TCP listener, including when
port 0 selects an available port.

## Where things are

| | |
| --- | --- |
| `web/` | an ordinary SvelteKit app: kit's tooling, kit's conventions |
| `@skgo/sveltekit-adapter` | kit's adapter, an ordinary devDependency paired with the skgo version `go.mod` requires |
| `web/src/routes/*.remote.go` | server logic, colocated with the routes that call it |
| `web/src/routes/**/server.go` | ordinary Go HTTP handlers for SvelteKit `+server.ts` routes |
| `internal/skgo/` | skgo's generated Go implementation; never edited by hand |
| `cmd/gimbal/` | the binary |
| `web/server.go` | the one composition the binary and any test both use |

Write a remote function by adding a Go function to a `*.remote.go` file beside
the route that needs it and marking it with `skgo.Query` or `skgo.Command`, then
run `just build`. skgo writes the `.remote.ts` module kit compiles, the
TypeScript types its callers see, and the Go registration the server mounts.

Write a server route with an ordinary `net/http` handler in `server.go` beside
the route and mark it with `skgo.GET`, `skgo.POST` or another HTTP method. The
same build gesture writes the throwing `+server.ts` stub Kit compiles and the Go
registration the server answers in both production and development.

## The listener

`web.NewRuntime` owns the project's runs and starts the web application before
it returns. TCP is loopback-only. A Unix-domain socket is removed when the
runtime context ends, and headless workflows use `web.WithNoWeb`.

## Development

Start the application and frontend together:

```sh
just dev
```

Open `http://127.0.0.1:8080/workflows`. Every registered built-in has a page
with a source diagram, a guide generated from its Go documentation, and a typed
Run form. Inspection never starts a workflow. Select a step for its prompt,
context shape and source; expand scopes and consecutive context writes in place.

During development, edits to workflow Go source and its local dependencies
refresh the diagram, guide and parameter descriptions together. Invalid source
keeps the last successful version with an error indication. Svelte changes use
Vite HMR. Selection, independent expansions, inspector disclosures, active view
and camera are URL state, so reloads and shareable links preserve supported
choices. Removed or changed source identifiers default without migration.

`just dev` generates bindings and builds the backend; on a fresh checkout it
also builds the initial frontend build. Stop it with Ctrl-C. Set
`GIMBAL_DEV_PORT` and `GIMBAL_DEV_VITE_PORT` to change the default ports 8080
and 5173. Restart after extractor changes; regenerate and restart after changes
to callable Go signatures. Source watching reflects code, not execution values.
Installed builds use embedded workflow snapshots and need no source checkout or
Go compiler to show these pages. `just dev-web` and `just dev-go` remain available
for running the two processes separately.

`#lib` is a Node subpath import (`package.json` → `imports`), and TypeScript
resolves those without probing for extensions: write `#lib/model.ts`, not
`#lib/model`. Vite accepts both, so only the type check would tell you.

Go answers loads, remote functions and server routes in both modes. In
production the binary renders pages from the embedded frontend build. In
development it still renders each document in Go, using modules transformed by
Vite; modules, styles, static assets and HMR continue through to Vite.

## Browser acceptance

The Playwright-BDD suite in `e2e/` checks browser behavior. Build the application,
then run:

```sh
just build
just e2e
```

By default, Playwright starts `bin/gimbal` with its embedded frontend on a free
port, waits for its listening address, and stops it after the suite. It uses the
existing build, so rebuild after changes. To check a server you have already
started, use `BASE_URL=http://127.0.0.1:8080 just e2e`; Playwright then leaves that
server running. The scenarios need a recorded run in the project's `.gimbal/runs/`;
for a clean checkout, seed the existing test fixture first:

```sh
mkdir -p .gimbal/runs/e2e-cancelled
cp internal/observation/testdata/cancelled-run.jsonl .gimbal/runs/e2e-cancelled/run.jsonl
```

Each outcome leaves a screenshot under `e2e/screenshots/` so a successful run can
be inspected.
