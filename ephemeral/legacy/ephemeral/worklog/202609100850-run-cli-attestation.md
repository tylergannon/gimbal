# gimbal run / gimbal ls: live attestation

Date: 2026-09-10. Agent: Claude Fable 5.1. Branch: `claude/run-cli`.

I built `gimbal ls` and `gimbal run <workflow> [flags]` over the ported POC
`program/` package, then ran `sprint-execute` and `chapter-loop` for real
against two scratch Go repositories with default models (implement
gpt-5.6-terra, review gpt-5.6-sol, evaluate gpt-5.6-luna). Both exited 0 and
printed `completed`. This is what I saw.

## sprint-execute (greet CLI, two ledger items)

Elapsed 14:34 to 14:50 UTC. Operations, in order:

1. Loop validated both items: both commands exit 1 (no subcommands yet).
2. `implement Add the hello subcommand` (130s). The agent added both `hello`
   and `shout`, committed each separately.
3. `review Add the hello subcommand` (133s): no material defect.
4. Loop re-validated: both commands exit 0, both items marked `done: true`.
5. `evaluate checklist` (85s): **rejected**. The evaluator objected that
   `test "$(go run . hello)" = ...` ignores the command's exit status, and
   that `go vet`, unknown-subcommand behavior, and dependency-freedom in the
   definition of done had no mechanical check.
6. Loop reopened the first item with that feedback. `implement` again (383s):
   the agent rewrote both item commands to check exit status, added a third
   ledger item "Verify the sprint definition" with a verifier script covering
   `go vet`, no external modules, unknown-subcommand exit 2, and a clean
   worktree, and committed.
7. `review` (no material defect), Loop validated all three items (exit 0),
   `evaluate checklist`: **passed**. Final ledger: three items, all `done: true`.

The agent editing the ledger between laps is behavior the POC's `Loop`
explicitly allows (it re-reads the file every lap). Nothing in the workflow
asked it to add an item; it did so to satisfy the evaluator.

## chapter-loop (count CLI, one chapter, two sprint items)

Elapsed 14:34 to 14:45 UTC. Operations, in order:

1. Chapter loop validated the chapter command (exit 1), then the nested
   sprint loop validated both sprint items (exit 1).
2. `implement Add the words subcommand` (92s): agent added `words` and `chars`
   in one commit.
3. `review` (96s): no material defect.
4. Sprint loop re-validated: both exit 0, `done: true`.
5. `evaluate checklist` (56s): **rejected**. `chars` counted bytes, not runes,
   and the sprint doc said commit per item but there was one combined commit.
6. Sprint loop reopened the first item. `implement` (149s): switched to
   `utf8.RuneCountInString`, added a test, committed separately.
7. `review`, re-validate (exit 0), `evaluate checklist`: **passed**.
8. Sprint loop exhausted. Chapter loop validated the chapter command (exit 0)
   and ran `evaluate chapter ledger` (91s): **passed**. Chapter `done: true`.

## What this establishes

- `Loop` re-reads the ledger every lap, validates every item by running its
  command, owns `done`, and reopens the first item when the evaluator fails
  the ledger. Exactly the POC-WORKFLOWS.md description.
- `ChapterLoop` is a `Loop` whose body runs a `Loop`. The nested sprint loop
  finished before the chapter-level validation ran.
- `Codergen[T]` decoded the structured review, evaluate, and turn results
  through the real Codex harness.
- `operations.jsonl` and `agents/op-NNNNNN.jsonl` in the `--logs` directory
  are enough to reconstruct both runs after the fact; that is how this note
  was written.
- The evaluator on gpt-5.6-luna is strict. Both runs spent a second lap
  satisfying it. That is a prompt and ledger-authoring observation, not a
  loop defect.

Run directories were kept locally and not committed.
