# Workflow-writing skill and one compiling Example per shape

URL: https://github.com/tylergannon/gimble/issues/177
State: closed
Updated: 2026-09-14T01:09:03Z

The beta bar says an agent should write any requested workflow shape as easily as any Go program. Today the only teaching material is `go doc` and `internal/workflows/sprint/sprints.go`. Agents in this repo load `.claude/skills` and `.agents/skills` (both hold the `df-*` skills now).

## Change

- One skill, `gimble-workflows`, installed under both `.claude/skills/` and `.agents/skills/` (same content, one source). It says: what a run is, the primitives in one table (`Run`, `Scope`, `Group`, `Loop`, `NewSession`, `Generate[T]`, `Steer`, `Fork`, `Set`/`SetJSON`/`ScopeText`, supervisors, kill by id), the shapes, how to run and watch on the page, how to read `run.jsonl`, and the house rules a workflow author meets: the validator's findings never become the goal (#168), the judge's input differs in kind from the actor's output, absolute workdirs in prompts, cheap models for attestation.
- The shapes, each a compiling `Example` in the root package (`example_test.go`), and each named in the skill: one turn; fork and bake-off; critique round; supervised worker; `Loop` with a planner; worktree per candidate; validation command; a scope killed by an operator and the loop that recovers.
- Nothing under `docs/` (needs Tyler's permission). The skill is the doc.

## Done when

- `go vet ./...` and `go test ./...` pass with the examples in place.
- A fresh Sonnet session with only the skill and the repo, asked for "a bake-off of two Codex sessions judged by a Claude session", writes a compiling workflow on the first try. Record the prompt and the output under `ephemeral/attest/skill/`.

