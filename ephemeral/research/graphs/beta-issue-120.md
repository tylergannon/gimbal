# #120: Validate supervision in a live run

Captured 2026-09-14 from https://github.com/tylergannon/gimble/issues/120. Read beta-implementation-handoff.md for final session decisions and coordination notes.

Supervision has not been validated in a live run. In both Sprint 2 runs (`20260911-000839.sprint` on `gpt-5.6-luna` and `haiku`, and `20260911-003721.sprint` on `gpt-5.6-luna` and `sonnet`), no supervisor looked at anything and no steer happened:

- The sprint workflow attaches a supervisor only to each coder's turn, at the default three-minute interval. Every coder turn finished in under three minutes; the longest took 2m57s.
- The validator's turn ran for 10m40s with no supervisor attached, because supervision is attached per turn.

The last time a supervisor was seen looking and steering live was before `WithSupervisor` and `WithInterval` existed: a Haiku supervisor steered a Codex worker away from writing a file.

Done when a live run on real models shows all of the following:

- A supervisor attached with `WithSupervisor` looks at a running turn at its `WithInterval`.
- An objection is steered into that turn and changes what the worker does next.
- A supervisor of a supervisor, attached through the supervisor's own options, does the same to the supervisor's look.

Do it with the cheap tier and say which models ran. Fix whatever the run shows is broken. Folds in #114.
