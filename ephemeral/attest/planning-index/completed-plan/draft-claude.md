## Implementation Plan: Python Retry Delay Helper

**Source Policy Evidence:** `engineering/retry-policy.md:3` — Formula `7 * min(max(attempt, 0), 4)` with zero-based attempts.

---

### Module Design

**Task 1: Resolve module filename**  
Ask the person: should this be `retry.py`, `backoff.py`, or another name? (Defer finalization to them.)

**Task 2: Create Python module**  
File: `<decided_filename>.py` (location: project root unless specified otherwise)

Single function:
- `retry_delay(attempt: int) -> int`
- Implements formula: `7 * min(max(attempt, 0), 4)`
- Returns seconds (int)
- No dependencies, no scheduler/IO/CLI

**Task 3: Create test file**  
File: `test_<decided_filename>.py` (project root)

Test structure using `unittest`:
- Test negative attempts → 0 seconds
- Test attempt 0 → 0 seconds  
- Test attempt 1 → 7 seconds
- Test attempt 2 → 14 seconds
- Test attempt 3 → 21 seconds
- Test attempts 4+ (samples: 4, 5, 10, 100) → 28 seconds each

---

### Validation

Run: `python3 -m unittest discover`  
Expected: All tests pass; function computes exact policy without side effects.

---

### Open Decision

**Filename:** Which of `retry.py`, `backoff.py`, `retry_policy.py`, or another name do you prefer? This must be confirmed before implementation.