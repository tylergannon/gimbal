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

## Builtin workflows

Start with a goal, a claim, or a local design. `work` inspects the request,
asks only material questions, recommends a workflow, and lets you choose:

```sh
./bin/gimble work -repo /path/to/project -goal "Make the export usable offline"
```

Choose directly when you already know the shape:

```sh
# One supervised worker session for a small change.
./bin/gimble lfg -repo /path/to/project -goal "Fix the empty export" -check "npm test"

# Sprint Plan: orientation, independent drafts, cross-critiques, interview, and synthesis.
./bin/gimble plan -repo /path/to/project -file design.md -goal "Implement this design"

# Execute a plan with a planner, supervised coders, and independent validation.
./bin/gimble sprint -repo /path/to/project -plan /absolute/path/to/plan.md -check "npm test"

# Build a readable semantic index over a local source cache.
./bin/gimble index -repo /path/to/project -from /path/to/cache -to /path/to/index
```

Sprint Plan first reads relevant repository and recent planning context, follows
configured semantic-index routes to cited sources, and saves that orientation
in a shared `intent.md`. Three agents draft and cross-critique independently.
The synthesis shows their differences, asks up to four useful questions when
needed, and saves the actual decisions in `merge-notes.md` beside `plan.md`.

The index workflow reads local text sources and builds topic routes and cited
leaves. Its default `-mode auto` builds a new index or refreshes changed sources;
`-mode build` rebuilds, `-mode update` requires an existing index, and `-mode
audit` checks freshness and index integrity without model calls or index edits.
Sources and index output must be separate directories. Unsupported sources are
reported as coverage debt; the first version reads UTF-8 text files up to
256 KiB each. It does not sync remote caches.

After a successful index build, the planning project receives
`.gimble/semantic-index.json`. Sprint Plan discovers that pointer automatically,
or reads an existing `docs/SEMANTIC-INDEX.md`. Use `-semantic-index
/path/to/index/README.md` and `-token-cache /path/to/cache` for explicit inputs.
When the planning project is inside the source cache, index building prints
these flags instead of writing configuration into the cache. Index run records
are saved alongside the index in a separate directory printed by the command.

Use `-acceptance`, `-constraints`, repeatable `-file` context paths, and
repeatable `-check` commands to describe the request. Relative file paths
resolve from `-repo`. Checks run in that repository; their failures cannot be
overridden by an agent's verdict. General repositories do not inherit
Gimble's Go checks. Checks are optional: without `-check`, `lfg` reports
successful worker completion, not an independent acceptance verdict.

The defaults are Codex `gpt-5.6-luna`, Claude `haiku`, and, for planning,
Gemini `gemini-3.8-flash-low`. Select them with `-model`, `-review-model`,
and `-planning-model`. Their native CLIs must be installed and authenticated.
Supervisors look every 30 seconds by default; `-supervisor-interval` changes
that interval. Short turns may finish before a supervisor looks.

Changes stay local and uncommitted by default. Sprint's explicit
`-finish pr` or `-finish merge` selects publication, which requires a clean
Git worktree with `.gimble/` ignored. Planning alone never implements the
plan. Guided `work` offers execution after planning; `-yes`
accepts its recommendation and proceeds without questions. `-dry-run`
previews without model calls or execution commands; it is not evidence that
the workflow works.

Each invocation prints its unique request/artifact directory under
`.gimble/requests`; run records live under `.gimble/runs`. Previews use
`.gimble/previews`. The web server uses a free port while work runs; use
`-no-web` for terminal-only operation. Ctrl-C cancels agent work.

## Where things are

| | |
| --- | --- |
| `web/` | an ordinary SvelteKit app: kit's tooling, kit's conventions |
| `@skgo/sveltekit-adapter` | kit's adapter, an ordinary devDependency paired with the skgo version `go.mod` requires |
| `web/src/routes/*.remote.go` | server logic, colocated with the routes that call it |
| `web/src/routes/**/server.go` | ordinary Go HTTP handlers for SvelteKit `+server.ts` routes |
| `internal/skgo/` | skgo's generated Go implementation; never edited by hand |
| `cmd/` | the binary |
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
