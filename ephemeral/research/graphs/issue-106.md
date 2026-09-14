# Add just attest, vet, and test recipes

URL: https://github.com/tylergannon/gimble/issues/106
State: closed
Updated: 2026-09-14T01:01:13Z

SPRINTS.md's Sprint 1 asks for `vet`, `test`, and `attest` recipes in the Justfile. None of them exist yet. `just attest` is one workflow on `gpt-5.6-luna` and Haiku that touches every primitive once:
- a scope with data
- a schema turn and a text turn
- a steer that lands and one that drops
- an interrupt
- a fork
- a group of two
- a supervised turn with an objection
- a loop of two laps

It prints what it saw. It needs `Session.Interrupt` first.

