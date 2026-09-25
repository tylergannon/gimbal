# Gimbal

Gimbal is a Go library for writing agent workflows as ordinary Go, with a
web page that shows every run live. It restarted from an empty tree on
2026-09-10. `ephemeral/legacy/` is the old code: inspiration, not the API.

## The rule

As simple as possible. Add a name only when a workflow that exists needs
it. Everything else is ordinary Go written in the workflow.

## Read first

- `skills/gimbal/SKILL.md`: authoring workflows, building and releasing Gimbal,
  and using its workflows.

- `go doc -all .`: the current public API and its behavioral contract.
- `ephemeral/research/api/API.md`: the design record and reasons behind the API.
- `ephemeral/research/api/SPRINTS.md`: what is being built, in what order,
  and how each sprint is proven.
- `docs/definition-of-done.md`: how work is gated, validated, and merged.
- `docs/web-app.md`: the web app's UI decisions, features, and user stories
  by name. The shadcn-svelte primitives are in `web/src/lib/components/ui/`.
- `ephemeral/research/svelte-idioms/dos-and-donts.md`: before changing any
  Svelte or SvelteKit code in `web/`. Tyler's hard rules (no query params for
  identity, no History API, `form` remotes for real forms and `command`
  elsewhere, no code bent for a test harness) and the anti-patterns found in
  this app, each with the replacement and a grep signature. The docs it cites
  are `svelte-idiomatic.txt` beside it.

## No wrappers

Tyler: "WE ARE NOT DOING HIGH LEVEL WRAPPERS OF FUNCTIONALITY THAT OBSCURES
THE MEANING OF THE CODE. There is NO SUCH THING as a `workflows.BakeOff`
function." A workflow reads like a page of pseudocode. A tactic (a
bake-off, a critique round, a worktree, a merge, a retry) is written inline
in the workflow that needs it, with `Group`, `Generate`, and git through
`RunCommand`. A new exported name exists only when Tyler asks for it by name.
Propose "write program X that does Y", never "add function Z".

## Build what was asked

Implement only what was asked. A reviewer's objection is not a
requirement. No backwards compatibility, no deprecation paths, no shims:
delete what is replaced. Unit tests for what you are building are fine,
and they live beside the code they test.

## Proof is what you saw

Proof is running the real thing yourself and saying what you saw, in the
chat or the PR description. There are no proof programs. Nothing written
to perform or record a run is committed: no `ephemeral/attest/`, no
`result.md`, no screenshots, no run logs, no text dumps of a run. A check
that should be repeatable is a test in the package it checks. An issue's
"Proof" section is satisfied by that report.

Apart from the frozen old code in `ephemeral/legacy/`, `ephemeral/` holds
notes, never code: nothing there is built, vetted, or maintained. The
commit hook refuses new code and run output under it.

## Information lands locally

The local filesystem is the store and cache for everything crucial to a
task: it is where an agent finds things. A workflow that needs a GitHub
issue writes its text to a file first and names the file, with its
absolute path, in the prompt. No prompt points an agent at a remote
source, and prompts are plain English: "Read and implement the issue in
/path/to/168.md."

## Working here

- Commit, and push, whenever something interesting has happened.
- Live runs you start to see something work use the cheapest models:
  Codex `gpt-5.6-luna`, Claude Haiku, Gemini flash. Say which model a run
  used.
- Ports from `ephemeral/legacy/` are rewritten against the new contract by
  hand, never spliced by script.
- No reflection and no `runtime.Caller` to recover a call site. Every node
  is named at its call site with a constant.
- The layout: the API is the root package `gimbal`. The page is a
  `tylergannon/skgo` app: `web/` is the SvelteKit app, Go beside its pages
  in `web/src/routes/*.remote.go`, `web/server.go` is the one `NewHandler`
  the binary and the tests share, `generated/` is written by `go generate
  ./...` and never by hand, `cmd/gimbal/` is the binary. `just build` builds all
  of it. `docs-site/` is the fully prerendered SvelteKit documentation site.
  The page's Go imports `gimbal`, so `gimbal` never imports the page; that is
  why `Serve` is in package `web`.
