# Implementation Plan: Python Retry Delay Helper

## Goal
Plan the Python retry delay helper described in [design.md](file:///Users/tyler/.codex/worktrees/7b5c/gimble-planning-index/ephemeral/attest/planning-index/project/design.md), using the established shared policy.

## Acceptance
The plan implements exactly the shared retry policy and names its source evidence. It specifies a small Python module and tests without a scheduler or IO.

## Constraints
- Pure computation: helper computes seconds only; no scheduler, no sleep, no network or disk I/O, no external packages, no CLI.
- No files modified and no mutating commands run during the planning phase.
- Exact module naming and placement resolved: `retry.py` in the project root and `test_retry.py` beside it.
- Preserve exact validation command: `python3 -m unittest discover`.

---

## Source Evidence
1. **Design Specification**:
   - [`design.md:3`](file:///Users/tyler/.codex/worktrees/7b5c/gimble-planning-index/ephemeral/attest/planning-index/project/design.md#L3): Defines requirement for `retry_delay(attempt: int) -> int` computing seconds only, without scheduler, network I/O, external packages, or CLI. Mandated resolving the filename choice prior to finalization and forbade implementation during planning.
2. **Semantic Index Route**:
   - [`index/routes/page-0003.md:5-19`](file:///Users/tyler/.codex/worktrees/7b5c/gimble-planning-index/ephemeral/attest/planning-index/index/routes/page-0003.md#L5-L19): Routes themes "Implement bounded retry backoff" and "Keep delay computation pure and side-effect free" to leaf `sources/627e497d46be482b.md`.
3. **Semantic Index Leaf**:
   - [`index/sources/627e497d46be482b.md:1-24`](file:///Users/tyler/.codex/worktrees/7b5c/gimble-planning-index/ephemeral/attest/planning-index/index/sources/627e497d46be482b.md#L1-L24): Documents policy recipe `7 * min(max(attempt, 0), 4)`, zero-based attempt handling, boundary mappings, and gotchas (pure number only; callers own sleeping and job scheduling).
4. **Shared Engineering Policy Source**:
   - [`cache/engineering/retry-policy.md:3`](file:///Users/tyler/.codex/worktrees/7b5c/gimble-planning-index/ephemeral/attest/planning-index/cache/engineering/retry-policy.md#L3): "retry_delay(attempt) returns seconds. Attempts are zero-based. Return 7 * min(max(attempt, 0), 4). Thus attempt zero has no delay, attempts 1/2/3 wait 7/14/21 seconds, and attempts 4+ wait 28 seconds. Negative attempts have no delay. This helper only computes a number; it must not sleep, schedule jobs, or perform IO."

---

## Use Cases
- Upstream workflows and callers calculate bounded backoff delay durations in seconds via `retry_delay(attempt)`.
- Callers maintain full ownership of sleeping, timing, and job scheduling without side-effects in the calculation module.

---

## Approach & Specification
1. **Module Interface (`retry.py`)**:
   ```python
   def retry_delay(attempt: int) -> int:
       """Return retry delay in seconds according to the shared retry policy."""
       return 7 * min(max(attempt, 0), 4)
   ```
2. **Mapping Contract**:
   - `attempt < 0`: returns `0`
   - `attempt == 0`: returns `0`
   - `attempt == 1`: returns `7`
   - `attempt == 2`: returns `14`
   - `attempt == 3`: returns `21`
   - `attempt >= 4`: returns `28` (capped at 4 attempts)
3. **Purity Boundary**:
   - Zero standard library or third-party imports required (no `time`, `asyncio`, `os`, `sys`, `requests`, etc.).
   - Pure arithmetic computation.

---

## Ordered Tasks & Dependencies

1. **Task 1: Create Python Module `retry.py`** (Dependency: None)
   - Create `/Users/tyler/.codex/worktrees/7b5c/gimble-planning-index/ephemeral/attest/planning-index/project/retry.py`.
   - Implement `retry_delay(attempt: int) -> int` returning `7 * min(max(attempt, 0), 4)`.
   - Verify no imports, scheduling, sleeping, I/O, or CLI execution blocks are added.
2. **Task 2: Create Unit Test Suite `test_retry.py`** (Dependency: Task 1)
   - Create `/Users/tyler/.codex/worktrees/7b5c/gimble-planning-index/ephemeral/attest/planning-index/project/test_retry.py` importing `retry_delay` from `retry`.
   - Implement `unittest.TestCase` covering:
     - Negative attempts (`-10`, `-1`) returning `0`
     - Zero attempt (`0`) returning `0`
     - Step increments: `1 -> 7`, `2 -> 14`, `3 -> 21`
     - Bounded ceiling: `4 -> 28`, `5 -> 28`, `100 -> 28`
3. **Task 3: Source Inspection for Purity** (Dependency: Task 1)
   - Verify by inspection that `retry.py` imports no modules and performs no I/O, sleeping, or scheduling.
4. **Task 4: Execute Validation Suite** (Dependency: Task 2, Task 3)
   - Run test discovery from the project root using standard `unittest`.

---

## Affected Files
- `/Users/tyler/.codex/worktrees/7b5c/gimble-planning-index/ephemeral/attest/planning-index/project/retry.py` (new file)
- `/Users/tyler/.codex/worktrees/7b5c/gimble-planning-index/ephemeral/attest/planning-index/project/test_retry.py` (new file)

---

## Exact Validation Commands
Run from the repository root:
```bash
python3 -m unittest discover
```
*Note on planning proof: This command was not executed during this planning phase to respect the explicit constraint in [design.md:3](file:///Users/tyler/.codex/worktrees/7b5c/gimble-planning-index/ephemeral/attest/planning-index/project/design.md#L3) ("Do not implement during this planning task") and the immutable constraints.*

---

## Risks & Mitigations
- **Naming Mismatch for Discovery**: Standard `unittest discover` requires test files to match `test*.py`. Placing `test_retry.py` in the root guarantees default pattern discovery succeeds immediately.
- **Accidental Side Effects**: Any addition of `time.sleep()` would violate the pure delay computation contract. Task 3 includes source inspection ensuring zero imports and pure arithmetic.

---

## Open Questions
- None. All requirements, shared policy formulas, file names, and locations have been confirmed.