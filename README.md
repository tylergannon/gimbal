# Gimble

Gimble is a Go library for writing agent workflows as ordinary Go. It is a
host-neutral runtime: a workflow can drive Codex, Claude Code, another agent
harness, or an adapter you provide. Gimble is not a feature of any one coding
agent or development environment, and it can run wherever your Go program can
run. Its public programming contract is the root package's Godoc and compiling
examples:

```sh
go doc -all github.com/tylergannon/gimble
```

The project runtime starts the SvelteKit web application automatically. It
listens on loopback port 8080 by default; `web.WithPort`, `web.WithUDS`, and
`web.WithNoWeb` select another runtime shape. Node is a build-time dependency
only.

## Build and run

Gimble has not been publicly announced. For now, distribution targets agents
running on this machine in other projects. Building locally and installing
with `go install ./cmd/gimble` is sufficient; signed binaries and scalable
public distribution are outside the current scope. Build the frontend first
as described below so the installed binary includes the web application.

This local distribution scope does not lower the standard for the skills:
agents in other projects must be able to use the workflow-authoring and
run-operation instructions without relying on context from Gimble's development.

This project uses Justfile for its build commands. It requires Node 24, pnpm 11, and just.

```sh
just build
./bin/gimble
```

Then open http://127.0.0.1:8080. The page's heading, the Go version below it, and the
greeting counter all come from `web/src/routes/hello.remote.go` — the
`hello.remote.ts` beside it is generated and every one of its bodies throws, so
anything that renders is proof that Go answered.

For development, build once and run these in separate terminals:

```sh
just dev-web
just dev-go
```

## Run a workflow

The same binary runs the workflows built into it:

```sh
./bin/gimble run --help
./bin/gimble run <workflow> --help
```

Each workflow's subcommand is generated from its source: Gimble's `--work-dir`
environment flag, one flag per field of its workflow parameter struct, and one
model flag per role its graph names. The absolute work directory is passed to
the entry in `gimble.Env`; its `.gimble` holds the run, served as above.

The binary currently includes the read-only `review` workflow. Its code-review
model defaults to `gpt-5.6-luna` from `cmd/gimble/defaults.json`; pass
`--code-review` to override it.

Workflow roles name cognitive work, not positions in a workflow. Gimble's
prescribed `WorkflowRole` constants and their descriptions live together in
`roles.go`; applications may define additional typed constants when they need
a role the catalog does not provide.

`go generate ./internal/workflows/...` runs Polytype for each workflow's
declared structured outputs, then the independent workflow generator in
`internal/generate/`. It can rebuild missing or stale generated commands
without first building the application CLI.

Author a workflow in one Go file with its `func Name(ctx context.Context, env
gimble.Env, params NameParams) error` entry, workflow-specific parameter and
result structs, and
generate directives; declare its result structs to Polytype in a
`//go:build jsonschema` file beside it, as the root package does; then import
its generated `Command(defaults)` and register it in
`cmd/gimble/workflows.go`. The application reads the shared
`cmd/gimble/defaults.json` once; an unknown role is required on the command
line when that file has no default for it.
Use `Iterate(ctx, name, items)` to give each item in a finite slice its own
scope. Use `PromiseLoop(ctx, name, goal, planner)` and range over its `Tasks`
when a planner chooses work adaptively; check `Err()` afterward.

## Lint workflows

The distributed `gimble` binary also checks deterministic workflow authoring
mistakes:

```sh
./bin/gimble lint ./...
go vet -vettool="$(pwd)/bin/gimble" ./...
```

The second command is the direct Go vet-tool protocol path. A custom vet tool
replaces Go's ordinary analyzers for that invocation, so `just vet` first runs
ordinary `go vet ./...`, builds the same `bin/gimble`, and then runs
`bin/gimble lint ./...`.

Read the [lint rule reference](internal/gimblelint/rules.md) for the accepted
rules, diagnostics, rewrites, and analysis limits. The same text is printed by
`gimble lint -help` and is available at [docs/lint-rules.md](docs/lint-rules.md).

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
| `cmd/gimble/` | the binary |
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

`just build` once, then two terminals:

```sh
just dev-web   # vite, on 127.0.0.1:5173
just dev-go    # the Go server, rendering from vite's modules
```

`#lib` is a Node subpath import (`package.json` → `imports`), and TypeScript
resolves those without probing for extensions: write `#lib/model.ts`, not
`#lib/model`. Vite accepts both, so only the type check would tell you.

Go answers loads, remote functions and server routes in both modes. In
production the binary renders pages from the embedded frontend build. In
development it still renders each document in Go, using modules transformed by
Vite; modules, styles, static assets and HMR continue through to Vite.

## Browser acceptance

The generated Playwright-BDD suite in `e2e/` is the starter application's
executable contract. Start either the production binary or both development
processes, then run:

```sh
just e2e
```

The same scenarios run in both modes. They prove that Go rendered the initial
document, a greeting visibly refreshes without reloading, and client navigation
and a direct deep link both reach the About route. Each outcome leaves a
screenshot under `e2e/screenshots/` so a successful run can be inspected.
