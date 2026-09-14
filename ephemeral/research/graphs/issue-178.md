# Loop practice runs: four shapes on the cheap tier, with management notes

URL: https://github.com/tylergannon/gimble/issues/178
State: closed
Updated: 2026-09-14T01:26:21Z

The beta bar asks for practice writing and observing loops and managing them. Nobody has yet sat through several Loop runs with different shapes and written down what steering and killing did.

## Change

Three or four small loop workflows on the cheap tier (`gpt-5.6-luna`, `claude-haiku-4-5-20251001`, `gemini-3.8-flash-low`), each a program under `ephemeral/attest/loop-practice/<shape>/main.go` run through `web.NewRuntime` so the page is watched:

1. planner ends the loop on its own;
2. a validation command decides;
3. a supervisor objects mid-turn (steer lands);
4. a `Group` inside a task, one attempt killed by id.

Each gets `notes.md`: what the loop did, what went wrong, which steer or kill was sent, what the log shows, what the page failed to show. The notes feed the skill and are the first loop-management notes for beta.

## Done when

- Four run directories and four notes files are committed under `ephemeral/attest/loop-practice/`.
- Every defect found is filed as its own issue, with the run id, and listed at the bottom of the notes.

