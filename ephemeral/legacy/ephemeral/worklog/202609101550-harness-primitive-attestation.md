# Harness collapsed to the five-method adapter: live attestation

Date: 2026-09-10. Agent: Claude Fable 5.1. Branch: `claude/harness-run`.

I deleted `harness/backend.go` and rewrote the Codex, Claude, and agy
`adapter.go` files from blank against the original contract:
`CreateSession`, `RunTurn(ctx, input, onEvent)`, `Steer`, `Interrupt`,
`Compact`. Plain errors, context cancellation, no categories, no routing.
`program.Codergen` now picks the adapter by the resolved model's harness,
creates a session, and runs one turn. Then I ran it. All live runs used
`gpt-5.6-luna` except the first two `run-prompt` calls, which used the `gpt`
alias (sol) before Tyler asked for the cheap tier.

## run-prompt, text (Codex, 14s)

Prompt: create `hello.txt` with one exact line, reply DONE. Output was `DONE`.
The file contained the line. The agent log under `agents/op-000001.jsonl` shows
the user part, an assistant message, the apply-patch tool call and result, an
`od` verification command, token usage, and the final assistant text.

## run-prompt, structured (Codex)

`--output-schema` with `line_count` integer and `first_word` string, prompt to
read `hello.txt`. Output: `{"first_word":"harness","line_count":1}`. Validated
against the exact schema before printing.

## Cancellation, run-prompt

Prompt told the agent to run `sleep 240`. Once `pgrep` showed the `sleep 240`
child, I sent SIGINT to the gimbal process at 15:37:08. By 15:37:12 gimbal had
exited with `gimbal: context canceled`, and neither `codex app-server --stdio`
nor `sleep 240` remained. That is the ctx path: `readTurn` returns on ctx,
the adapter sends `turn/interrupt`, drains briefly, returns `ctx.Err()`.

## Cancellation, inside a workflow

I started `sprint-execute` on the default models, then interrupted it 40s into
the first implement turn to switch to luna. It exited with
`implement "Add the c2f subcommand": context canceled`, the app-server was
gone, and the scratch repo had no partial commit.

## sprint-execute on gpt-5.6-luna (units CLI, two ledger items)

15:39 to 15:50 UTC. Operations in order:

1. Loop validated both items; both commands exit 1.
2. `implement Add the c2f subcommand` (137s): agent added `c2f` and `f2c`,
   committed each separately.
3. `review` (65s): no material defect.
4. Loop re-validated: both exit 0, both `done: true`.
5. `evaluate checklist` (81s): **rejected**. The ledger commands cover only the
   happy paths; nothing mechanically verifies unknown-subcommand exit 2,
   dependency freedom, or `go vet`.
6. Loop reopened the first item. `implement` (181s): added a third ledger item
   "Accept Sprint 1 definition of done" with a verifier command, committed.
7. `review` (73s), re-validate (all three exit 0), `evaluate checklist`
   (101s): **passed**. "I independently reran both item checks and the final
   acceptance command; all passed."

Final scratch repo: four commits on top of the baseline, clean tree, ledger
three items all `done: true`.

## What this establishes

- The three rewritten adapters run real turns: Codex live for all of the
  above; Claude and agy against their fakes in unit tests with the same
  contract.
- Schema present returns validated JSON; absent returns text. Both paths are
  exercised through `Codergen[T]` with `T` a struct, `json.RawMessage`, and
  `string`.
- A cancelled context stops a native turn within seconds and surfaces as
  `context.Canceled`, both from `run-prompt` and from inside `Loop`.
- `Loop`, `Validate`, and the sprint workflow behave exactly as they did on
  the old backend yesterday, so nothing above the harness changed in meaning.

Run directories were kept locally and not committed.
