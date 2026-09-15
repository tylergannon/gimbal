# Merge decisions

## Overview
Final synthesis incorporates the resolved module placement from the user interview:
- Filename and location: Confirmed as `retry.py` at the project root with `test_retry.py` beside it.
- Consensus: Implementation will follow the formula `7 * min(max(attempt, 0), 4)` with zero-based attempts. The function will be purely computational with no external dependencies or side effects.
- Scope and boundaries: Planning does not implement code or execute tests. Once implemented, proof will be verified via `python3 -m unittest discover` from the repository root.
- No remaining unresolved tradeoffs or open questions.

## Decisions
Accepted findings & interview resolution:
- Confirmed module naming and location based on user answer: the helper module will be `retry.py` located at the project root, and the test file will be `test_retry.py` beside it. This directly satisfies the requirement in `design.md:3` to resolve the filename with the person before finalizing.
- Retained the strict purity boundary: no imports for timing/sleeping, no scheduling, no network/disk I/O, no CLI wrappers, and no external package dependencies. Verification includes static code inspection alongside test discovery.
- Fully preserved the exact semantic route and source evidence chain: `design.md:3`, `index/routes/page-0003.md:5-19`, `index/sources/627e497d46be482b.md:1-24`, and `cache/engineering/retry-policy.md:3`.
- Concrete test cases enumerated for unit testing: negative attempts (-10, -1) -> 0, zero attempt (0) -> 0, incremental step progression (1 -> 7, 2 -> 14, 3 -> 21), and capped attempts (4 -> 28, 5 -> 28, 100 -> 28).

Rejected alternatives:
- Rejected placing tests in a subfolder (e.g. `tests/test_retry.py`) in favor of user's explicit choice of `test_retry.py` directly beside `retry.py` in the project root.
- Rejected adding type coercion or non-integer runtime exception logic, as the policy strictly models integer attempt counts `attempt: int` and adding unrequested behavior would expand scope beyond the specification.
