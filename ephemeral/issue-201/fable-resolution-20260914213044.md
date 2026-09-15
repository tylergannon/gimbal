# Fable resolution: issue 201 review round 01

Resolver: Claude Fable 5.1. Date: 2026-09-14. Worktree branch: codex/issue-201-graph-plan.
Findings from `ephemeral/reviews/20260914211117-issue-201-fable-5-1-round-01.md`.
Encoding decision from Tyler, received after the first attempt: natural recursive
Go model, handwritten JSON codec for now, Codex files the polytype request.

## Finding 1: the workflow.Graph contract is now compilable Go

`ephemeral/issue-201/graph-contract/` is package `workflow` inside the gimble
module (import path `github.com/tylergannon/gimble/ephemeral/issue-201/graph-contract`).

| File | Content |
| --- | --- |
| `doc.go` | Package doc: the shape decisions and the wire format. |
| `graph.go` | `Graph`, `Source`, `Site`, `Expression`, `Session`, sealed `Operation` and `Subgraph`, `AgentCall`, `Command`, `Set`, `SetJSON`, `Exit`/`ExitKind`, `Sequence`, `Condition`/`Branch`, `Loop`, `Scope`, `Group`/`GroupChild`, `Supervision`, `Supervisor`, `Diagnostic`. |
| `codec.go` | `//go:build !jsonschema`. Owner codecs for every struct holding `[]Operation`, mirroring polytype's generated owner codecs on `LifecycleRecord`. `"kind"` discriminator in snake_case; nil slices as `[]`; unknown and foreign fields rejected. |
| `declare.go` | `//go:build jsonschema`. `polytype.Declare(Graph.Schema)` and `polytype.SealedUnion[Operation]("kind", polytype.Snake)`, the declaration that replaces `codec.go` once polytype accepts recursion. |
| `graph_test.go` | Round trip of a Sprint-shaped graph with every variant, three nesting levels, an ancestor-owned reviewer reused in a repeated task, and a nested supervisor; rejection cases. |
| `polytype-attempt.txt` | Output of `go tool polytype -target ./ephemeral/issue-201/graph-contract`. |

Verification run in this worktree:

```
go build ./ephemeral/issue-201/graph-contract/            ok
go build -tags jsonschema ./ephemeral/issue-201/graph-contract/   ok
go vet ./ephemeral/issue-201/graph-contract/              ok
go test -count=1 ./ephemeral/issue-201/graph-contract/    ok (5 tests)
go build ./...                                            ok
go tool polytype -target ./ephemeral/issue-201/graph-contract
  -> exit 1: "cyclic dependency found" at graph.go Supervisor; no output written
```

Model choices beyond the agreed snippets, all documented in `graph.go`:

- `Supervision` lists live on the construct owning the watched call's execution
  scope (`Graph`, `Scope`, `Loop`, `GroupChild`), never inside the ordered body.
- `Supervisor` and `Session` carry `Site` so attachments and declarations have
  static identity and a source anchor; `Session.Scope` records the owner and
  `Session.Origin` the fork source.
- `Sequence` is the root body and the body of a bounded helper expansion, so two
  callers of one helper stay distinct sites.
- `Loop` covers both `gimble.Loop` (name, planner, goal, task-scope supervision)
  and an ordinary Go loop (condition only, no scope).
- `Command.Arguments` keeps the approved function's arguments as expressions,
  neutral to the still-unnamed signature.
- A `Graph` is partial iff `Diagnostics` is non-empty; no separate flag.

What works now: the Go layout, the JSON wire shape, and JSON encode/decode.
What is dependency work: pinned polytype v1.0.0 rejects the recursive types, so
there is no JSON Schema, `ValidateJSON`, TypeScript declaration, or devalue codec
for `workflow.Graph`. The handwritten codec does not teach polytype or skgo the
type. Until the polytype request lands, `graph.json` reaches the page as text
inside a transported string field, the way `page.server.go` already carries the
run snapshot, and the two acceptance rows that need generated TS/devalue are
marked not claimable. The plan and model say this in those words.

## Finding 2: one CLI, no root-package bridge, SprintWorkflow explained

- **Start binding.** The unnamed root-package bridge is gone. Route Go already
  reaches runtime-owned state through the request context (`observation.WithRegistry`
  in `web/runtime.go`, `BaseContext`, `observation.FromContext` in
  `web/src/routes/runs/[runID]/page.server.go`). The plan now specifies run
  starting the same way through `internal/live`, which already holds the
  run/runtime handshake: a `live.Start` func type with `WithStart`/`StartFrom`,
  placed by `web.NewRuntime`, fetched by the generated `startSprint`. No public
  name is added. Caller-module web apps are declared out of scope; their entry
  is `Runtime.Run[T]`.
- **Generator.** `cmd/gimblegraph` is replaced by a `generate-graph` subcommand of
  the `gimble` binary, sibling of `generate-bindings`, invoked as
  `go run github.com/tylergannon/gimble/cmd generate-graph ...`. The name is
  marked as proposed for Tyler to confirm.
- **SprintWorkflow.** The plan now states the type is exported only because
  `-workflow-type` selects by type name; the exported value `Workflow` remains
  what the comment asked for.

## Documents updated

- `ephemeral/issue-201/implementation-plan.md`: design status; section 1
  encoding paragraphs; section 2 SprintWorkflow and generator; section 3
  bindings directive, start mechanism, and interim graph transport; delivery
  step 1; implementation locations; two acceptance rows.
- `ephemeral/issue-201/graph-model.md`: status line; sealed-interface note on
  the interim codec; supervision snippet synced with `graph.go` and placement
  rule; "Encoding decision" section rewritten as settled with the file list and
  the TS/devalue caveat; final acceptance bullet.

## Not done, and why

- `AGENTS.md` L46–47 still says graph-model.md records "outstanding encoding
  choices"; it is outside my file ownership. One line to update.
- The approved command function's name and signature remain Tyler's to choose;
  `Command` is shaped to survive that choice.
- The package sits in the main module, so `go test ./...` runs its tests. It has
  no `//go:generate` directive, so `just build`'s `go generate ./...` is unaffected.
- Nothing committed. Others' modified files (`review-user-decisions.md`, the
  worklog) were not touched.
