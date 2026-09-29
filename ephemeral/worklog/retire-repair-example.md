# Retire the superseded compiler example

decision: Tyler requested removal of the older repair workflow after the compiler
review. Remove its authored entry, Temporal target, fixture project, graph,
private activities and example-specific tests. Keep continuity, fixed fan-out,
planning, shared resource/context support and independent supervision coverage.

decision: The default example is now continuity. Historical repair observations
and replay belong to b5aeff81; deleting the example does not revalidate those
observations against current code. Do not retain obsolete orchestration as a
compatibility path merely to replay its old history.

finding: prepareFixture was genuine test-project setup, not an intended enterprise
compiler primitive. Its deletion removes that misleading reference example.

finding: The remaining targets can reuse the authored continuity result types;
their schemas match the removed duplicates. Aliasing the external Checks type
requires keyed construction for go vet.

doc_bug: Regeneration found the planner graph's source line numbers one line behind
the existing source. Retain generator-produced corrected source associations.

coordination: Implementation is isolated from the original chat's development
worktree. The app's managed worktree API has no sibling-repository parameter, so
this Gimbal checkout was created through git while Gimbal View reuses the attached
compiler-target-review worktree.

friction: One full-suite hook attempt failed the untouched Antigravity adapter's
successful-result/unsettled-tool test with a closed stdout pipe. An immediate
isolated agy package rerun passed. Do not describe this intermittent observation
as repaired by the compiler-example removal; investigate separately if repeated.
