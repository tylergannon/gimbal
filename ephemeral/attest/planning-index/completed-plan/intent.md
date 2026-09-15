# Shared intent

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


# Shared intent

## Repository orientation

Repository: /Users/tyler/.codex/worktrees/7b5c/gimble-planning-index/ephemeral/attest/planning-index/project
Goal: Plan the Python retry delay helper described in design.md, using the established shared policy.
Acceptance: The plan implements exactly the shared retry policy and names its source evidence. It specifies a small Python module and tests without a scheduler or IO.
Constraints: 

## Recent planning context
- no sprint documents found


## Semantic prior art
Entrypoint: /Users/tyler/.codex/worktrees/7b5c/gimble-planning-index/ephemeral/attest/planning-index/index/README.md
Token cache: /Users/tyler/.codex/worktrees/7b5c/gimble-planning-index/ephemeral/attest/planning-index/cache
Read the entrypoint, follow bounded relevant routes, and cite leaf files. Do not scan the full cache.

## Fixed validation
- python3 -m unittest discover

## Immutable input
Preserve the original goal, acceptance, constraints, and checks exactly.


## Retrieved findings

Observed facts:

- `design.md:3` requires a Python `retry_delay(attempt: int) -> int` helper returning seconds only, with no scheduler, network I/O, packages, or CLI. It explicitly leaves the module filename unresolved and forbids implementation during planning.
- The configured index route maps bounded backoff and pure delay computation to `engineering/retry-policy.md` ([route](</Users/tyler/.codex/worktrees/7b5c/gimble-planning-index/ephemeral/attest/planning-index/index/routes/page-0003.md:3-13>)).
- The shared policy is: `7 * min(max(attempt, 0), 4)`. Attempts are zero-based; negative and zero return `0`, attempts `1/2/3` return `7/14/21`, and attempts `4+` return `28`. The helper must not sleep, schedule, or perform I/O ([source](</Users/tyler/.codex/worktrees/7b5c/gimble-planning-index/ephemeral/attest/planning-index/cache/engineering/retry-policy.md:3>)).
- No Python module, test file, package metadata, or sprint document was observed in the project tree.
- Required proof command: `python3 -m unittest discover`. It was not run because no implementation exists.

Unresolved questions:

- What filename should the Python module use?
- Where should the module and its tests reside?
- Runtime behavior for values that are not integers is not specified.