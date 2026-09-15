# Critique of Plans A and B

Both plans address the core requirement soundly. Here's the detailed assessment:

## Scope

**Plan A**: ✓ Tight and correct. Step 1 confirms filename as a prerequisite (design.md requirement). Step 2 implements only the formula. Step 3 adds unit tests. Explicitly excludes scheduler, I/O, packages, CLI. Correctly notes that the design does not define behavior for non-integers, so the plan needn't specify it.

**Plan B**: ✓ Equally on-target, with more granular detail. Step 3 lists concrete test cases (negative, zero, 1–3, cap, above-4) rather than abstract coverage areas. This is planning specificity, not scope creep, and aids handoff to implementation.

**Winner**: Tie. Plan A is more minimal; Plan B is more prescriptive.

---

## Ordering

**Plan A**: ✓ Logical sequence.
1. Confirm filename (gates implementation)
2. Implement function (depends on 1)
3. Add tests (depends on 2)
4. Run validation (final proof)

**Plan B**: ✓ Identical logical order: confirm → implement → test → validate.

**Winner**: Tie. Both sequences are sound.

---

## Feasibility

**Plan A**: All steps are pure Python development with no external dependencies, scheduling, or I/O. Feasible as written.

**Plan B**: Same. Step 3 is more prescriptive (e.g., `test_retry.py` or `tests/test_retry.py`) but these are only examples (`e.g.`), not hard requirements. Feasible as written.

**Issue in Plan B**: Uses `file:///` URLs in markdown links (e.g., `[design.md](file:///Users/tyler/...)`). These will not resolve without a running file server and do not match the repository's actual file paths. Plan A uses repository-relative citations that are more portable.

**Winner**: Plan A. Citation format is more usable.

---

## Proof (Exact Validation Command)

**Plan A**:
```
python3 -m unittest discover
```
Expected result: "all retry-policy tests pass, demonstrating the exact shared mapping and bounded pure computation." 
**Critical observation**: Explicitly states "This command was not run because implementation is explicitly out of scope for this planning task." ✓ Acknowledges planning-vs.-implementation boundary correctly.

**Plan B**:
```bash
python3 -m unittest discover
```
States to "Run the exact test command from the repository root" but does not explicitly clarify that **this plan does not run or implement**—only describes what the validation will do.

**Issue in Plan B**: Conflates the plan's intent (describe tests) with implementation (actually run them). The instruction says "Change no files and run no mutating commands." Plan B's phrasing risks being misread as a directive to actually execute.

**Winner**: Plan A. Cleanly separates planning (describing what tests will validate) from implementation (actually building and running them).

---

## Source Evidence

**Plan A**: Cites three anchors:
- `[design.md:3]` 
- `[semantic route (page-0003.md:3-19)]`
- `[shared policy (retry-policy.md:3)]`

All line ranges are accurate and grounded.

**Plan B**: Cites the same sources but uses `file:///` URLs with anchor fragments (`#L3`). While intent is clear, the format is non-portable. Additionally, does not cite the semantic route (page-0003.md), only the terminal sources.

**Winner**: Plan A. Complete source chain with reliable citation format.

---

## Alignment to Acceptance Criteria

| Criterion | Plan A | Plan B |
|-----------|--------|--------|
| "implements exactly the shared retry policy" | ✓ Formula: `7 * min(max(attempt, 0), 4)` | ✓ Same formula, plus example outputs |
| "names its source evidence" | ✓ design.md:3, semantic route, retry-policy.md:3 | ✓ design.md, retry-policy.md (missing semantic route) |
| "specifies a small Python module and tests" | ✓ Defers module naming (as required); test coverage listed abstractly | ✓ Defers module naming; test cases listed concretely (negative, zero, 1–3, cap, >4) |
| "without a scheduler or IO" | ✓ Explicitly excluded | ✓ Explicitly excluded |

---

## Summary

| Dimension | Plan A | Plan B | Notes |
|-----------|--------|--------|-------|
| **Scope** | ✓ Tight | ✓ Tight + detailed | Both correct; B more prescriptive |
| **Ordering** | ✓ Sound | ✓ Sound | Identical sequences |
| **Feasibility** | ✓ Yes | ✓ Yes | B's file:// URLs reduce portability |
| **Proof clarity** | ✓ Excellent | ⚠ Ambiguous | A separates planning from implementation; B conflates them |
| **Source evidence** | ✓ Complete & portable | ⚠ Incomplete & non-portable | A cites semantic route; B uses file:// URLs |

**Recommendation**: **Plan A is the stronger plan.** It cleanly separates planning (describing what to build and test) from implementation (actually building it), cites all three source layers reliably, and removes ambiguity about the planning task's scope. Plan B provides more concrete test case coverage, which is valuable, but its citation format and phrasing around proof execution create unnecessary friction. A hybrid—Plan A's structure with Plan B's test case specificity—would be optimal, but as submitted, Plan A is the tighter plan.