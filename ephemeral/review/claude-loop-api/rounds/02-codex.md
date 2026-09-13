# Round 2: Codex response

## 1. Held points

### Where a `Close` failure lands

Agree. A scope's returned error is an input to workflow control: `Group` cancels siblings on it ([group.go](/Users/tyler/.codex/worktrees/loop-api-review/gimble/group.go#L48)), and `Loop` stops when its task scope returns one ([loop.go](/Users/tyler/.codex/worktrees/loop-api-review/gimble/loop.go#L130)). Adapter cleanup happens after the body has established that result, so it should not retroactively turn successful work into a task failure or change sibling and commit decisions.

`SessionClosed{Error}` should record each release failure, and `Run` should join a typed cleanup error into its final returned error separately from both the body error and `RecordingError`. I would aggregate all close failures in that typed value rather than retain only the first; the events preserve per-session detail, while the returned value lets a caller distinguish failed work, incomplete cleanup, and failed recording. `RunEnded` must also reflect the complete run verdict or the event sequence must define a later terminal verdict that does, rather than reporting success immediately before `Complete` reveals cleanup failure.

### Skip shadowing keys in previous-task feedback

Disagree. Shadowing is the public mechanism for revising a value in a child scope ([scope.go](/Users/tyler/.codex/worktrees/loop-api-review/gimble/scope.go#L111)). A task may intentionally change a visible parent value such as `status`, `assessment`, or `plan`; silently omitting it from feedback because its key already exists would hide exactly the revision the planner needs. The runtime cannot infer from key collision that `role` is task-only instruction while `status` is task output.

The double-role evidence is real, but it should remain part of the two-projection exercise. That exercise should determine how a workflow explicitly separates task input from planner feedback; until then, preserving all task-local values is less surprising than an undocumented semantic exception inside `Loop`. I would accept the shadow-skipping rule if representative workflows established that child shadows are uniformly execution-local and never intended as feedback, and the contract stated that clearly.

## 2. Invalid structured plans

End `Loop` on the first structurally invalid plan. A repair turn is a hidden retry policy, makes cost and termination less obvious, and recreates a smaller version of the protocol we are removing. A workflow that wants retries can express a bound in ordinary Go and create the planner session in the enclosing scope so its conversation survives the failed `Loop`; the validation error should be returned with enough detail for that next attempt.

## 3. Draft recommendations

1. **Agree — one planner channel.** Make the structured plan authoritative, validate it once, derive `backlog.md` and events from it, and state explicitly that an invalid plan ends `Loop`.

2. **Agree — required, idempotent `HarnessAdapter.Close`.** Keep cleanup out of scope control flow, record every failure on `SessionClosed`, and join an aggregate typed cleanup error into the final run result.

3. **Agree — observation tells the truth.** Validation, cancellation, and reload state are correctness properties of the runtime's public account, independent of later UI work.

4. **Dissent — feedback projection skips shadowing keys.** Key collision does not distinguish execution-only context from an intentional child revision; remove this immediate fix and merge the double-role problem into the deferred two-projection work.

5. **Agree — rendered values cannot forge sections.** Use an unambiguous escaped or length-delimited representation in both renderers so value bytes cannot become peer headings.

6. **Dissent — proposed Godoc wording.** Document that `ScopeText` renders workflow-stored scope values and excludes harness instructions, session history, memory, and tools; saying all Gimble prompt contributions are workflow lines is still false because `Loop` adds its own planner instructions ([loop.go](/Users/tyler/.codex/worktrees/loop-api-review/gimble/loop.go#L197)).

7. **Agree — put `Interrupt` in the interface.** The public operation should be a capability every adapter implements, with its own contract and proof.

8. **Dissent — deferred-item wording.** Agree to defer `Get`, selective projection, and changes to `Set`'s error return, but record the measured `Set`-specific ceremony accurately: 18 of 210 lines in the sprint fixture and 42 of 193 in the live workflow, about 9% and 22%, rather than 17–30% ([ceremony.txt](../ceremony.txt#L1)).

Keep recommendations 1–3 as the priority order. Remove draft item 4 as an immediate runtime change and fold it into item 8 as the concrete two-projection experiment; then renumber section-forging, required `Interrupt`, and the corrected Godoc ahead of the deferred work. Add the fail-on-invalid-plan decision to item 1 and the typed aggregate cleanup result to item 2.
