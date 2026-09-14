# Semantic Index Configuration

## Token Cache

The token cache is the full local materialization of the project context.
Tools like `rg`, `find`, `grep`, and `wc` operate on it directly.

**Remote source**: local only (this repository)
**Remote type**: `local`
**Local path**: `docs/` and `ephemeral/` under the repository root
**Sync command**:
```
(already materialized; it is the checked-in tree, refreshed by git pull)
```
**Token cache scope**: `docs/` is the definition of done and the static
site. `ephemeral/` is the design record (`research/api/API.md`,
`SPRINTS.md`, the three vendor research reports), the legacy codebase the
restart drew on (`legacy/`, inspiration only, not the API), attestation
runs with their logs (`attest/`), worklogs and reviews. About 180 files
and 5 MB, roughly 1.3M tokens, most of it `legacy/` and `research/`.

## Semantic Index

The semantic index is a routing tree built over the token cache.
Use it to retrieve relevant citations without scanning the full token cache.

**Status**: Not yet built
**Local path**: (none)
**Entrypoint**: (none)
**Access**: Search the token cache directly with `rg`. At its current
size one search answers a question in seconds, so an index was judged not
worth its upkeep when Sprint 001 was planned on 2026-09-12. Revisit when
`ephemeral/` grows past what a few `rg` calls can cover.

To build: run `/df-semantic-index <token-cache-local-path> <index-output-path>`
