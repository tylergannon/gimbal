# Gate releases on live proof, using Tractor's own checklist validation

URL: https://github.com/tylergannon/gimble/issues/69
State: closed
Milestone: None
Updated: 2026-09-13T16:47:13Z

## The problem

v0.9.0 shipped with a green test suite and nothing run end to end. The gap was noticed and written down as a footnote instead of stopping the release.

The `release-tractor` skill already covers this:

> Also run focused tests and live entry-point proof appropriate to the changed behavior. A passing test suite is not a substitute for exercising the released interface.

That is correct and it did not help, for two reasons. It sits in prose below a fenced block of commands, so the commands read as the gate and the sentence reads as advice. And it is only reached by loading the skill, which is exactly the step that gets skipped when a release is one item inside a larger request.

Prose that has to be remembered is not a gate.

## What live proof actually looks like here

Verified by hand after the fact. A scratch git repository with a deliberately broken HTTP server, a `.tractor/run`, and a checklist whose item curls the running service:

- `tractor run <pipeline>` — agent fixed the source, engine started the service on an allocated port, item command captured `status=200 body=ok`, judge passed, engine wrote `done: true`, run COMPLETED.
- Same repository with `.tractor/run` set to `exit 1` — item recorded `passed: false`, `infer: null`, `service never became ready`, no model consulted.
- `tractor run sprint-execute --goal ...` — the built-in resolved by name from the binary, walked `sprints → implement → review → sprints`, service on an allocated port, engine wrote `done`.

That is roughly twenty minutes and it caught a real defect the test suite could not (#67).

## Proposal

Gate the release on evidence rather than on remembering to gather it, using Tractor.

- A release checklist in the repository whose items carry the commands that produce this evidence: a scratch repository is built, a pipeline is run for real, and the item passes only when the engine wrote `done` and the run completed. The engine owns that, so a release cannot close on an assertion that it works.
- The `release-tractor` skill's publish step then depends on that checklist being closed, rather than on a sentence being remembered.
- The verification block in the skill should name the proof as a required artifact with a path, not as a closing paragraph.

The pleasing part is that this is what Tractor is for. If a checklist the engine validates is the right way to know a feature is done, it is the right way to know a release is releasable, and dogfooding it here costs one ledger.

## Open questions

- Which built-ins must be exercised per release: the ones whose files changed, or one representative every time? All four is expensive; `chapter-loop`, `sprint-plan` and `delivery-loop` are still unverified today.
- Whether the scratch repository is a committed fixture or built by the item's command each time. A fixture is faster and can go stale; building it each time is slower and always honest.

## Related

Discovered while closing out #65 and #67. Unverified as of now: `chapter-loop`, `sprint-plan`, `delivery-loop`.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

