# Continue issue 386 on the repaired Claude runtime

The user merged PR 391 and authorized installing it and moving the existing Pi work to a new run. The old run 01M3D2ZMCZ0ZH7Z5W77R52SJV1.implement was intentionally cancelled for the runtime transition, not because its implementation failed validation. The worktree retains its edits, and origin/main through 23934c8a is merged into this branch. The installed host runtime contains that fix.

Preserve and finish the existing pi/ adapter, routing, help and tests; do not rewrite completed work. The first outcome's independent validator ran the real Pi 0.87.1 subprocess against the fake endpoint and verified core turns, usage, schema re-ask, saved-session resume after process replacement, unexpected exit and idempotent close. That was core coverage, not acceptance of the entire harness: Steer/Fork and clear_queue-before-abort cancellation were still missing. The previous planner prematurely moved to integration, so explicitly finish these required behaviors now.

The last coder completed CLI/role-binding and documentation edits and invoked regeneration. Shutdown interrupted the ensuing workflow task check. Treat its validation as unfinished and inspect/re-run applicable checks; do not count cancellation as a passing result or as a demonstrated implementation failure. No real Diffusion Router qualification has happened yet.

The original user request, issue and agreed plan beside this note remain authoritative. Use the existing source and a small number of focused tests. Good enough means requested behavior seen working, without extra proof machinery. Do not require human steering to finish ordinary planner or QA responses under the repaired lifecycle. Run the usual implement workflow; do not modify it or these outcomes.

Before commands needing Pi or Router access, source /Users/tyler/.local/share/gimbal-issue-386/router.env with shell tracing off. It supplies Pi 0.87.1 on PATH and the authorized key; do not print or copy its value. Cached version-matched Pi docs are named in plan.md.
