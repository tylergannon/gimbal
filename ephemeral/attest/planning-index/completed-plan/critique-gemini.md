### Plan Critique: Scope, Ordering, Feasibility, and Proof

Both plans were evaluated against [design.md](file:///Users/tyler/.codex/worktrees/7b5c/gimble-planning-index/ephemeral/attest/planning-index/project/design.md#L1-L4), the shared policy at [engineering/retry-policy.md](file:///Users/tyler/.codex/worktrees/7b5c/gimble-planning-index/ephemeral/attest/planning-index/cache/engineering/retry-policy.md#L1-L4), and the required validation command (`python3 -m unittest discover`).

---

### Plan A Critique

#### 1. Scope
* **Strengths:** Fully aligned with the acceptance criteria and constraints. Accurately identifies and preserves the shared policy formula `7 * min(max(attempt, 0), 4)`. Explicitly cites all relevant source evidence: [design.md:3](file:///Users/tyler/.codex/worktrees/7b5c/gimble-planning-index/ephemeral/attest/planning-index/project/design.md#L3), semantic routing ([page-0003.md](file:///Users/tyler/.codex/worktrees/7b5c/gimble-planning-index/ephemeral/attest/planning-index/index/routes/page-0003.md)), and [retry-policy.md:3](file:///Users/tyler/.codex/worktrees/7b5c/gimble-planning-index/ephemeral/attest/planning-index/cache/engineering/retry-policy.md#L3). It adheres strictly to the prohibition against schedulers, I/O, packages, and CLI.
* **Gaps / Considerations:** Identifies that non-integer runtime behavior is unspecified in the design, choosing not to define it. While technically faithful to the spec, explicitly documenting whether type hints alone suffice or if type checking is out of scope would further clarify the boundary.

#### 2. Ordering
* **Strengths:** Execution sequence is linear and dependency-respecting: Step 1 resolves the open filename/location decision required by [design.md:3](file:///Users/tyler/.codex/worktrees/7b5c/gimble-planning-index/ephemeral/attest/planning-index/project/design.md#L3) prior to creating files, followed by function definition, test coverage, and validation.
* **Gaps / Considerations:** Step 1 states "Confirm the unresolved Python module filename... before finalizing the implementation plan", but does not provide concrete filename candidates to the user to make the decision immediately actionable.

#### 3. Feasibility
* **Strengths:** Directly feasible with pure standard Python arithmetic without external dependencies or side effects.
* **Gaps / Considerations:** Specifies "unittest-discoverable tests" without naming a test pattern (e.g. `test_*.py`), which is necessary for the default discovery pattern in standard `unittest`.

#### 4. Proof
* **Strengths:** Accurately preserves and assesses the exact command: `python3 -m unittest discover`. Explicitly clarifies that the command was intentionally omitted during this planning phase to respect the constraint that no implementation occur during planning.

---

### Plan B Critique

#### 1. Scope
* **Strengths:** Correctly implements the shared policy `7 * min(max(attempt, 0), 4)` and cites `engineering/retry-policy.md:3`. Prohibits dependencies, CLI, scheduler, and I/O.
* **Gaps / Considerations:** Omits citation of the semantic routing page and [design.md:3](file:///Users/tyler/.codex/worktrees/7b5c/gimble-planning-index/ephemeral/attest/planning-index/project/design.md#L3) as source evidence, citing only the leaf cache file.

#### 2. Ordering
* **Strengths:** Tasks are ordered logically from filename confirmation to module creation, test creation, and validation.
* **Gaps / Considerations:** Redundancy between "Task 1" in the main flow and the trailing "Open Decision" section. Combining these or phrasing Task 1 as an actionable prompt clarifies whether the plan is awaiting user response before proceeding.

#### 3. Feasibility
* **Strengths:** Highly feasible. Concrete file naming pattern `test_<decided_filename>.py` ensures that `python3 -m unittest discover` will detect the suite by default without custom discovery arguments. Test cases enumerate explicit boundary inputs (negative, `0`, `1`, `2`, `3`, `4`, `5`, `10`, `100`).
* **Gaps / Considerations:** None. Concrete and directly implementable.

#### 4. Proof
* **Strengths:** Accurately includes the exact validation command: `python3 -m unittest discover`, with explicit expected outputs matching the step mapping.
* **Gaps / Considerations:** Does not explicitly state that the command was not run during planning due to the "do not implement during this planning task" instruction in [design.md:3](file:///Users/tyler/.codex/worktrees/7b5c/gimble-planning-index/ephemeral/attest/planning-index/project/design.md#L3).

---

### Summary Comparison

| Criterion | Plan A | Plan B |
| :--- | :--- | :--- |
| **Scope** | Strong: Comprehensive citations (design, route, leaf policy); respects all constraints. | Good: Follows policy & constraints, but omits intermediate semantic route citation. |
| **Ordering** | Strong: Strict gated sequence resolving filename before file creation. | Good: Sequential, though slightly duplicates the filename question in Task 1 & Open Decisions. |
| **Feasibility** | Strong: Pure computation; slightly abstract test file naming. | Strong: Concrete test file naming (`test_<name>.py`) guarantees unittest discovery works out-of-the-box. |
| **Proof** | Strong: Exact command (`python3 -m unittest discover`) and clarifies why it was not run during planning. | Good: Exact command and expected test behavior; omits explicit note on non-execution during planning. |
