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


## Paired proof follow-up

decision: Tyler authorized the 18-claim continuity/planning comparison and reuse of focused existing evidence. Reuse this task worktree so the retired workflow stays absent; no compiler/public API work added.
correction: An unconditional union of feedback keys is insufficient for conditional shadowing. The new paired test failed when the skipped review write selected the inherited parent value. The target now adds review only after that branch writes successfully.
correction: The source Check clones arguments using append to nil; the target retained an empty non-nil slice, producing [] rather than null in the missing-command record. Match source capture/encoding.
friction: Native app artifact inventory hung; existing task worktrees were inspected directly and reused instead of creating another worktree.
friction: Temporal serializes a final errors.Join as an ApplicationError. Check supported infrastructure identity at the authored caller boundary inside the workflow, before external serialization; do not imply arbitrary client-side concrete error preservation.
