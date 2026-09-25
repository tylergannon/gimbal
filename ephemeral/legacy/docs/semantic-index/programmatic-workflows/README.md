# Programmatic-workflows semantic index

For the Go API, start with the [recovered POC workflow source](sources/poc-workflows.md):
`program.Loop`, `Codergen[T]`, `Command`, `Validate`. That is the approved shape and it is
ported into `program/`. The later stub sketches were removed on 2026-09-10.

This is a compact routing tree for Gimbal's writings about replacing the JSON
workflow language with ordinary Go, while preserving workflow legibility and
making context engineering a first-class concern.

## Scope and token cache

The token cache is the local Gimbal repository checkout. The indexed corpus is
intentionally narrower: the two edited direction documents, the five-arts
framing, and the six preserved source notes under
`ephemeral/projects/gimbal/programmatic-workflows/`. Citations are repository-
relative `path:line` anchors and should resolve from the repository root.

This index is a navigation aid, not a claim that the proposed runtime exists.
The current graph engine remains the shipped implementation; the Go-program
runtime, unified context, and evolving task index are documented direction.

## Route order

1. Start with [recipes.md](recipes.md) when you have a task-shaped question.
2. Use [topics.md](topics.md) to route by concept or source authority.
3. Use [themes.md](themes.md) for cross-cutting relationships and tradeoffs.
4. Open the linked leaf under `sources/` for the exact source anchors.

The edited syntheses are the best starting points for the current framing.
The verbatim leaves preserve Tyler's original wording and corrections; use
them when the distinction between the source claim and later synthesis matters.

## Layout

- `recipes.md` — task-first routes.
- `topics.md` — concept and authority routes.
- `themes.md` — cross-source synthesis.
- `routes/` — narrow topic nodes used by the task and concept routes.
- `sources/` — one dense leaf per indexed writing.
- `.semantic-index/evals.jsonl` — small retrieval query set.
- `.semantic-index/state.json` — generated housekeeping metrics.

## Housekeeping

- Build mode: scratch build.
- Last indexed source review: 2026-09-09.
- Benchmark: run `python3 <df-semantic-index>/scripts/run_evals.py --index docs/semantic-index/programmatic-workflows --write-results` from the repository root.
- Known debt: citations point to mutable working-tree documents; the preserved
  verbatim notes provide stable wording but not immutable historical snapshots.
