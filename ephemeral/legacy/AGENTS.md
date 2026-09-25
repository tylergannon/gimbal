# Gimbal repository instructions

## This is a proof of concept

Everything in this repository is a proof of concept. Nothing here has users,
a stable interface, or a release. When you change functionality, change it.
Agents are FORBIDDEN from planning, proposing, or building backwards
compatibility: no deprecation paths, no legacy flags, no shims, no "keep the
old behavior behind an option," no migration notes. Delete what is replaced.
If a change breaks something you were not asked to touch, open a GitHub issue
describing the break and move on.

## What Gimbal is for

`docs/five-arts.md` names the five arts of orchestration this project exists to
serve, and the tensions between them. Read it before proposing a feature.

## The approved workflow shape

Workflows are ordinary Go. The approved API is the recovered POC preserved at
`ephemeral/projects/gimbal/programmatic-workflows/POC-WORKFLOWS.md` and ported
into `program/`: `program.Loop`, `program.Codergen[T]`, `Runtime.Command`,
`Runtime.Validate`. Sprints and chapters are both just `Loop`. Do not invent
wrappers around these. Do not reintroduce a graph language, node types, or a
context subsystem.

The CLI is `gimbal ls` and `gimbal run <workflow> [flags]`. A workflow's flags
are its input struct's fields. There is no JSON input file, schema command, or
catalog.

## Build what was asked

Implement only what Tyler asked for. A reviewer's objection is not a
requirement. Do not add proof frameworks, safety hardening, schema catalogs,
or "more robust" alternatives unless asked. Unit tests that check what you are
building are fine. The proof of a workflow is an agent running it and saying
what it saw.

## Semantic index

Writings on the move from graph definitions to Go programs are routed through
`docs/semantic-index/programmatic-workflows/README.md`. Read its entrypoint
before searching that topic broadly.

## Reference submodules

`reference/` holds upstream projects mounted as submodules for reading only.
Never commit inside one and never push from one. See `reference/README.md`.

## Repository size and generated artifacts

- Never commit run directories, event streams, stage transcripts, prompts,
  responses, checkpoints, or raw tool logs. Keep them local. Commit a small
  human-authored summary only when the result is worth preserving.
- Do not vendor large third-party source trees, research corpora, binaries,
  archives, nested repositories, or generated dependency caches. Record our
  findings and cite the upstream source. If the repository genuinely needs to
  attach a substantial external codebase, use a submodule pinned to a reviewed
  upstream commit.
- `scripts/check-staged-content.sh` enforces the mechanical floor. Its large
  commit bypass is for an explicitly human-approved exception; agents must not
  set `GIMBAL_ALLOW_LARGE_COMMIT=1` on their own. The run-log prohibition is
  not bypassable.
