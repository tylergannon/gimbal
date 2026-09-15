# Implementation Plan: Python Retry Delay Helper

## Goal
Plan the Python retry delay helper described in [design.md](file:///Users/tyler/.codex/worktrees/7b5c/gimble-planning-index/ephemeral/attest/planning-index/project/design.md), using the established shared policy.

## Acceptance
The plan implements exactly the shared retry policy and names its source evidence. It specifies a small Python module and tests without a scheduler or IO.

## Constraints
- Pure computation: helper computes seconds only; no scheduler, no sleep, no network or disk I/O, no external packages, no CLI.
- Change no files and run no mutating commands during this planning stage.
- The module filename has not yet been selected; finalize naming with the user prior to creating the file.

---

## Source Evidence
- **Design Specification**: [design.md](file:///Users/tyler/.codex/worktrees/7b5c/gimble-planning-index/ephemeral/attest/planning-index/project/design.md#L3) dictates `retry_delay(attempt: int) -> int` computing seconds without scheduler/IO/packages/CLI.
- **Shared Policy**: [engineering/retry-policy.md](file:///Users/tyler/.codex/worktrees/7b5c/gimble-planning-index/ephemeral/attest/planning-index/cache/engineering/retry-policy.md#L3) defines:
  - Formula: `7 * min(max(attempt, 0), 4)`
  - Zero-based attempts:
    - `attempt <= 0` returns `0`
    - `attempt = 1` returns `7`
    - `attempt = 2` returns `14`
    - `attempt = 3` returns `21`
    - `attempt >= 4` returns `28`

---

## Ordered Implementation Tasks

1. **Confirm Module Filename**:
   - Confirm target module filename (e.g., `retry.py` or `retry_delay.py`) and test path (e.g., `test_retry.py` in the root or `tests/test_retry.py`) with the user as required by `design.md`.

2. **Implement Retry Delay Helper**:
   - Create the chosen Python module defining:
     ```python
     def retry_delay(attempt: int) -> int:
         return 7 * min(max(attempt, 0), 4)
     ```
   - Ensure the module has no external dependencies, performs no scheduling or sleeping, and executes no I/O.

3. **Implement Unit Tests (`unittest`)**:
   - Create test suite (e.g., `test_retry.py`) discovering via `unittest.TestCase`.
   - Add test cases covering:
     - Negative attempts (e.g., `-10`, `-1`) returning `0`
     - Zero attempt (`0`) returning `0`
     - Discrete scaling steps: `1 -> 7`, `2 -> 14`, `3 -> 21`
     - Upper cap: `4 -> 28`, `5 -> 28`, `100 -> 28`
     - Verify no side-effects or external imports.

4. **Validate**:
   - Execute the discovery test runner to ensure complete coverage of the shared retry policy contract.

---

## Concrete Validation Commands

Run the exact test command from the repository root:
```bash
python3 -m unittest discover
```
