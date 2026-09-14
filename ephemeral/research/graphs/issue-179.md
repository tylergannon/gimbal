# Arc: operator control of live runs, and the runtime work beside the usage tree

URL: https://github.com/tylergannon/gimble/issues/179
State: closed
Updated: 2026-09-14T14:28:59Z

The usage tree (#173) is rewriting `internal/observation/`, `web/src/lib/observation/`, and the run route, and states that it leaves the runtime, the adapters, the reducers, and the sprint workflow alone. This arc is everything in the Beta milestone that lives on the other side of that line, ordered so that each piece lands as its own small squash, so #173 rebases onto tidy commits rather than one wide one.

The arc has a spine. Beta's proof is Tyler watching a Gimble run build a Gimble feature and steering any agent from the page. Steering from the page needs the runtime to reach a live session by id, and killing an agent that has gone off the rails needs the runtime to reach a live scope or turn by id. That is the trade for `Session.Interrupt`: not a method on the pointer, but cancellation by id with a cause, built on the child contexts the library already creates. The rest of the arc is the hygiene that lets many runs and many agents coexist, and the sprint workflow and skill that make the proof run possible.

## Phases

**1. Shrink the other branch's rebase.** Land first, each touching files #173 names as unchanged.
- PR #158, Codex cache write. #173 calls it edit 1.
- #175, cancel by id with a cause, and #165 in the same PR.
- #159, infallible `Set`/`SetJSON` with the panic boundary.

**2. Operator control.**
- #107, steer reports landed or dropped.
- #113, Claude steer live.
- #176, the live run registry: `Runtime.Steer`, `KillScope`, `KillTurn` by id.
- #163, the Codex daemon descriptor limit, so a wide `Group` can be killed rather than starved.

**3. Many runs, many agents.**
- #118, one writer for `project.jsonl`.
- #111, run ids that do not collide in one second.
- #106, `just attest`, `vet`, `test`. The attest's interrupt is a ctx cancel, not `Session.Interrupt`.
- #112, `go vet` in a fresh clone.

**4. The workflow, its skill, and practice.**
- #168, the sprint workflow: a validator that cannot reshape the goal, plain local prompts, prompts visible before they run.
- #177, the workflow-writing skill and shape examples.
- #178, loop practice runs.

## Held until #173 merges

The runs index at `/`, steer and kill buttons on the page, #135 and #117 event coverage. All three edit what #173 is replacing or regenerate the skgo bindings that #173 also regenerates.

## Rules for every PR in the arc

- Nothing under `internal/observation/`, `web/src/`, or the generated skgo files. If a change forces regeneration, it waits.
- One issue per PR, squash-merged, PR body written as the commit message.
- Cheap tier for every live check. Logs under `ephemeral/attest/<issue>/`.

## Done when

Every child issue is closed, and a Go program using `web.NewRuntime` can, against a live run on the cheap tier, steer a session by id and kill a turn and a scope by id, with the log showing who did it and the Loop carrying on. That is the state from which the page half (steer and kill buttons, the runs index) is one route edit once #173 lands.

