<div align="center">

<img src="docs/assets/gimble-mascot.png" width="480" alt="Gimbal, a friendly gyroscopic guide, points along a trail toward a goal">

# Gimbal

**Workflows for coding agents, written as ordinary Go.**

[Direction](docs/direction.md) · [Workflows as programs](docs/workflows-as-programs.md) ·
[The five arts](docs/five-arts.md) · [Semantic index](docs/semantic-index/programmatic-workflows/README.md)

</div>

---

Gimbal is being rebuilt. The graph-definition engine, its CLI, MCP server,
plugins, and editor were removed on 2026-09-10. What remains is the bare
metal a Go-authored workflow needs:

| Package | What it is |
| --- | --- |
| [`harness/`](harness) | One five-method adapter interface, `CreateSession`, `RunTurn`, `Steer`, `Interrupt`, `Compact`, implemented for Claude Code, Codex, and Gemini CLI. `RunTurn` takes a context; cancelling it interrupts the native turn |
| [`checklist/`](checklist) | The Markdown/YAML ledger format and its `done` bookkeeping |
| [`program/`](program) | `Loop` over a checklist, `Codergen[T]`, `Command`, `Validate` |
| [`program/workflows/`](program/workflows) | `SprintExecute`, `ChapterLoop`, `DeliveryLoop` in ordinary Go control flow |
| [`internal/modelalias/`](internal/modelalias) | Model aliases resolved to provider, native model ID, and effort |
| [`cmd/gimbal`](cmd/gimbal) | `gimbal ls`, `gimbal run <workflow>`, and `gimbal run-prompt` |

The workflow shape is the one recorded in
[POC-WORKFLOWS.md](ephemeral/projects/gimbal/programmatic-workflows/POC-WORKFLOWS.md):

```go
for iteration, err := range program.Loop(ctx, ledger, program.LoopOptions{
    Validate:      runtime.Validate,
    MaxIterations: 8,
}) {
    if err != nil {
        return err
    }
    // Ordinary Go: call agents, inspect typed replies, branch, call helpers.
    // The iterator validates the workspace when the body returns.
}
```

The iterator owns validation and `done`. The body owns implementation, review,
and repair. A chapter loop is a loop whose body runs a sprint loop.

## Run a workflow

```sh
gimbal ls
gimbal run sprint-execute --help
gimbal run sprint-execute --goal "Finish the planned sprints." \
  --checklist docs/sprints/ledger.md --workdir . --logs .gimbal/run
```

A workflow's flags are its input struct's fields. The run directory holds
`operations.jsonl`, one line per command or agent call, and the native agent
logs under `agents/`.

This is a proof of concept. Interfaces change without compatibility.

## Run one prompt

```sh
gimbal run-prompt --workdir . "Explain the failing test."
```

`--output-schema` accepts exact JSON Schema and changes stdout to validated
JSON. Invoked from Codex without a model it selects Fable; from Claude Code it
selects GPT.

## License

[MIT](LICENSE).
