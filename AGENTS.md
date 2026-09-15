# Gimble

Gimble is a Go library for writing agent workflows as ordinary Go, with a
web page that shows every run live. It restarted from an empty tree on
2026-09-10. `ephemeral/legacy/` is the old code: inspiration, not the API.

## The rule

As simple as possible. Add a name only when a workflow that exists needs
it. Everything else is ordinary Go written in the workflow.

## Workflow aesthetic

A workflow's possible structure should be explicit and statically mappable
from its Go source. As a rule of thumb, lint against constructs that hide
that structure and advise the author to make it explicit. Dynamic worker
dispatch is an authoring violation, like a dynamic `Set`/`SetJSON` key;
write direct calls in ordinary `if`/`switch` branches. Prefer clear workflow
source over increasingly elaborate analysis to accommodate hidden structure.

Primitive runtime variability is expected: the extractor cannot know how
many times a loop will run or the contents of its `[]Task`. It maps the
loop and task-body template. Branch choices, prompts, command arguments,
and other runtime values may vary while the possible operations and their
structural relationships remain explicit. This aesthetic guides bounded
lint rules; it does not require exhaustive analysis of arbitrary Go.

## Workflow graph

Describe the structure connecting agent calls, commands, and context writes with
nested ordered bodies: sequences, conditions, loops, scopes, and groups. Scoped
agent calls in the right order are the target. Avoid modeling the program: a
full control-flow/dataflow map or launch/wait timeline adds unnecessary detail.
Operation is a sealed union; reserve Group for Gimble's parallel execution
primitive.

Supervision is a separate unordered hierarchy attached beside the watched call.
Nested supervisors watch their immediate supervisor's look turns. A supervisor
session can belong to an ancestor scope while its attachment and execution are
local to a deeper call. Session ownership determines conversation lifetime;
visual placement follows the attachment. Repeated appearances reference the same
session and do not clone conversations or duplicate turns/usage. Creating a
session in each task instead gives each task a fresh conversation. Shared sessions
still accept one active turn at a time.

The accepted model is recorded in `ephemeral/issue-201/graph-model.md`; its
compilable Go contract is in `ephemeral/issue-201/graph-contract/graph.go`.
Recursive Go is the selected representation. Polytype issue 127 owns generated
recursive serialization support; wait for its merge before integrating it.

Save the generated workflow.Graph as JSON with each run when it begins. The
viewer reads that saved shape, including after the workflow changes. Go remains
canonical; polytype supplies serialization. Run visualization needs no source
fingerprint, graph revision matching, or historical lookup against compiled graphs.

## Workflow commands

The intended command API is one approved, opinionated Gimble execution function
whose calls can be recognized and captured directly. Lint against direct os/exec
use in workflow code and advise use of that function. Its implementation and
harness internals may use os/exec. Do not reconstruct arbitrary subprocess
lifecycles in the graph; command results are secondary detail. The API name and
signature remain to be settled in the issue 201 plan.

## Workflow web pages

Select available workflows and their Go Input types explicitly at build
time. Generate concrete bindings, schemas, and types through polytype/skgo.
To make a workflow startable from the web, author an actual Svelte page for
that workflow and link to it from the application. Runtime workflow
registration and generic schema-driven launch forms are not the model.

## Read first

- `go doc -all .`: the current public API and its behavioral contract.
- `ephemeral/research/api/API.md`: the design record and reasons behind the API.
- `ephemeral/research/api/SPRINTS.md`: what is being built, in what order,
  and how each sprint is proven.
- `docs/definition-of-done.md`: how work is gated, validated, and merged.
- `docs/web-app.md`: the web app's UI decisions, features, and user stories
  by name. The shadcn-svelte primitives are in `web/src/lib/components/ui/`.

## No wrappers

Tyler: "WE ARE NOT DOING HIGH LEVEL WRAPPERS OF FUNCTIONALITY THAT OBSCURES
THE MEANING OF THE CODE. There is NO SUCH THING as a `workflows.BakeOff`
function." A workflow reads like a page of pseudocode. A tactic (a
bake-off, a critique round, a worktree, a merge, a retry) is written inline
in the workflow that needs it, with `Group`, `Generate`, and direct command
operations through the approved command API once introduced. Existing workflow
`os/exec` sites must migrate to it. A new exported name exists only when Tyler
asks for it by name; the requested command primitive’s name is still pending.
Propose "write program X that does Y", never "add function Z".

## Build what was asked

Implement only what was asked. A reviewer's objection is not a
requirement. No backwards compatibility, no deprecation paths, no shims:
delete what is replaced. Unit tests for what you are building are fine;
the proof of a workflow is a live run and what it showed.

## Information lands locally

The local filesystem is the store and cache for everything crucial to a
task: it is where an agent finds things. A workflow that needs a GitHub
issue writes its text to a file first and names the file, with its
absolute path, in the prompt. No prompt points an agent at a remote
source, and prompts are plain English: "Read and implement the issue in
/path/to/168.md."

## Working here

- Commit, and push, whenever something interesting has happened.
- Attestation runs use the cheapest models: Codex `gpt-5.6-luna`, Claude
  Haiku, Gemini flash. Say which model a run used.
- Ports from `ephemeral/legacy/` are rewritten against the new contract by
  hand, never spliced by script.
- No reflection and no `runtime.Caller` to recover a call site. Every node
  is named at its call site with a constant.
- The layout: the API is the root package `gimble`. The page is a
  `tylergannon/skgo` app: `web/` is the SvelteKit app, Go beside its pages
  in `web/src/routes/*.remote.go`, `web/server.go` is the one `NewHandler`
  the binary and the tests share, `generated/` is written by `go generate
  ./...` and never by hand, `cmd/` is the binary. `just build` builds all
  of it. `docs-site/` is the fully prerendered SvelteKit documentation site.
  The page's Go imports `gimble`, so `gimble` never imports the page; that is
  why `Serve` is in package `web`.
