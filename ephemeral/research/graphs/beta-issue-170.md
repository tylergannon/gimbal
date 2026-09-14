# #170: Finish the Gimble constitution

Captured 2026-09-14 from https://github.com/tylergannon/gimble/issues/170. Read beta-implementation-handoff.md for final session decisions and coordination notes.

A 207-line draft of the product promises, written by a Codex session on 2026-09-11 and never committed until today, is on branch `codex/constitution`:

- https://github.com/tylergannon/gimble/blob/codex/constitution/constitution.md
- https://github.com/tylergannon/gimble/blob/codex/constitution/ephemeral/worklog/202609111207-project-constitution.md

The draft states numbered promises, each with a "Verified when" clause: a workflow reads like a small Go program; ordinary Go defines control flow and parallelism; primitives, not tactics; one small vocabulary composes into many shapes; program shape is separate from program input; and so on. It positions itself as lower resolution than the API: `API.md` owns exact behavior, `SPRINTS.md` owns delivery status, `docs/definition-of-done.md` owns acceptance.

The worklog records one contradiction the draft could not resolve: it promises plain `errgroup` parallelism, while main has `gimble.Group`. That is settled on main now (`Group` exists, and the reason is in `API.md` § Concurrency), so the draft has to be brought in line rather than the other way round.

## Done when

- The draft is read against main's Godoc and `AGENTS.md`, and every promise matches what the code and the house rules actually say. The `errgroup` versus `Group` promise is rewritten.
- Every "Verified when" names something that exists or is in the Beta milestone, so a beta release can claim each promise or say plainly that it does not.
- Tyler decides where it lives. `docs/` needs his express permission; until then it stays where it is.
- The branch is merged or closed, and this issue says which.
