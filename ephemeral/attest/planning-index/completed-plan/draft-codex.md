Plan

1. Confirm the unresolved Python module filename and test location with the requester before finalizing the implementation plan, as required by `design.md:3`. Keep the layout to one small module and unittest-discoverable tests; add no package metadata, dependencies, CLI, scheduler, or I/O.

2. Implement `retry_delay(attempt: int) -> int` as pure arithmetic using exactly:

   `7 * min(max(attempt, 0), 4)`

   Preserve zero-based attempts: negative/zero → `0`; `1/2/3` → `7/14/21`; `4+` → `28`. Source evidence: [`design.md:3`](</Users/tyler/.codex/worktrees/7b5c/gimble-planning-index/ephemeral/attest/planning-index/project/design.md:3>), [semantic route](</Users/tyler/.codex/worktrees/7b5c/gimble-planning-index/ephemeral/attest/planning-index/index/routes/page-0003.md:3-19>), and [shared policy](</Users/tyler/.codex/worktrees/7b5c/gimble-planning-index/ephemeral/attest/planning-index/cache/engineering/retry-policy.md:3>).

3. Add focused `unittest` coverage for negative, zero, attempts `1–3`, the cap at `4`, and values above `4`. Do not specify behavior for non-integers because the design does not define it.

4. From the repository root, run this exact proof command:

   ```text
   python3 -m unittest discover
   ```

   Expected result: all retry-policy tests pass, demonstrating the exact shared mapping and bounded pure computation. This command was not run because implementation is explicitly out of scope for this planning task.

Original request preserved

Goal:
Plan the Python retry delay helper described in design.md, using the established shared policy.

Acceptance:
The plan implements exactly the shared retry policy and names its source evidence. It specifies a small Python module and tests without a scheduler or IO.

Constraints:


Repository: /Users/tyler/.codex/worktrees/7b5c/gimble-planning-index/ephemeral/attest/planning-index/project
Read these local context files:
- /Users/tyler/.codex/worktrees/7b5c/gimble-planning-index/ephemeral/attest/planning-index/project/design.md
Use these exact validation commands when describing proof:
- python3 -m unittest discover