Repository ground truth: [design.md:3](/Users/tyler/.codex/worktrees/7b5c/gimble-planning-index/ephemeral/attest/planning-index/project/design.md:3) requires the filename decision before finalization and forbids implementation during planning. The relevant route is [page-0003.md:5](/Users/tyler/.codex/worktrees/7b5c/gimble-planning-index/ephemeral/attest/planning-index/index/routes/page-0003.md:5), whose leaf states the exact policy and purity boundary at [retry-policy.md:3](/Users/tyler/.codex/worktrees/7b5c/gimble-planning-index/ephemeral/attest/planning-index/index/sources/627e497d46be482b.md:3).

## Plan A

- Scope is mostly correct: one pure helper and unit tests, with no scheduler, I/O, CLI, or dependencies.
- Deferring the filename is correct, but “project root unless specified otherwise” invents a location not established by `design.md`. Module and test location should remain one explicit decision gate.
- The policy citation is too weak for the repository’s retrieval requirement: `engineering/retry-policy.md:3` is not the indexed leaf path and does not identify the route or local source clearly.
- The test cases cover the important numeric boundaries, though “negative attempts” is underspecified; naming concrete cases such as `-1` and a larger negative would make the contract clearer.
- Proof is overstated. The listed tests demonstrate numeric outputs, but do not demonstrate the claim that the helper performs no scheduling, sleeping, or I/O. That should be a source-level inspection criterion alongside the test result.
- The exact validation command is preserved, but the plan should say it runs from the project root and should report the actual exit status. It must not be claimed as passed during planning.

## Plan B

- Scope is stronger and faithful to the request. Its formula and boundary examples match the leaf exactly.
- Ordering is sound: resolve the filename and test location, implement the helper, add tests, then validate. It correctly treats naming as a prerequisite rather than silently choosing one.
- Feasibility is good for the specified integer contract. It appropriately does not invent behavior for non-integer inputs.
- “Verify no-side-effects or external imports” is vague and not realistically established by ordinary unit tests. It should identify source inspection as the proof for the module’s absence of imports, timing calls, scheduling, and I/O; otherwise this becomes an undefined test requirement.
- Its source links are more useful than Plan A’s, but it cites the cache source directly rather than the required indexed route-and-leaf evidence. It should name the indexed leaf explicitly.
- The exact validation command is preserved verbatim and is explicitly rooted at the repository, which is better than Plan A. As with A, passing tests alone does not prove purity.

Overall, Plan B is the stronger plan. Neither fully satisfies the proof requirement as written because both conflate unit-test success with proof of side-effect freedom. The filename/location decision is a legitimate required pause, not a defect, because the original request explicitly leaves it unresolved.

Preserved validation command:

```text
python3 -m unittest discover
```

No files were changed and no mutating commands were run.