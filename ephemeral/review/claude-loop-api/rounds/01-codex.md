# Round 1: Codex response

## 1. Claude's three fixes

### Use one planner channel

Agree. Claude's block-scalar and trailing-space probes show that the current protocol can reject a semantically unchanged assignment because it compares two serializations byte for byte ([probes/main.go](../probes/main.go#L52), [probes.txt](../probes.txt#L1)). The malformed-backlog probe also shows that the repair loop is bounded only by the caller's context deadline ([probes/main.go](../probes/main.go#L75), [probes.txt](../probes.txt#L2)). These are consequences of asking one turn to communicate the same decision through a YAML file and a JSON answer.

I would refine Claude's proposal by making a structured `plan` the sole authority, with the goal retained outside model output and `Next` identifying one member of the returned task snapshot. The runtime should validate that snapshot, record the selected `Task` in durable events, and derive any human-readable backlog file from it. That makes goal immutability structural, removes serialization equality as a correctness boundary, and leaves one bounded validation result per planner turn. A free-form notes field should be added only when a workflow demonstrates a need for it.

### Require `HarnessAdapter.Close`

Agree, with lifecycle refinements. The adapter creates a native session but has no corresponding required release operation ([harness.go](/Users/tyler/.codex/worktrees/loop-api-review/gimble/harness.go#L11)); scope shutdown only marks Gimble's session closed and cancels its context ([scope.go](/Users/tyler/.codex/worktrees/loop-api-review/gimble/scope.go#L85)). That contradicts the ownership implied by `CreateSession`, and the surviving fork process is direct evidence of the leak.

`Close` should be idempotent and called only after a native session ID has been allocated. Gimble should mark the session logically closed before adapter cleanup so concurrent `Generate` calls cannot enter, then call `Close` with a short cleanup context that survives cancellation of the run context. A close failure is a lifecycle failure, not a recording error: `scope.do` and `scope.end` should return or join it into the scope/run result and emit events that describe the final outcome truthfully. Making `Interrupt` required is also defensible because `Session.Interrupt` is public while its implementation is currently optional ([session.go](/Users/tyler/.codex/worktrees/loop-api-review/gimble/session.go#L348)), but that is a separate contract change and should have separate proof.

### Make `Set` and `SetJSON` panic on misuse

Disagree. The claimed invariant that every failure is a programming error is false under the current API. `Set` accepts `float64`, so `Set(scope, "x", math.NaN())` reaches a real `json.Marshal` error, and `SetJSON` accepts an `Output` implementation whose marshaling is not guaranteed to succeed ([scope.go](/Users/tyler/.codex/worktrees/loop-api-review/gimble/scope.go#L111)). Scope closure and concurrent lifetime also make some failures dynamic rather than compile-time mistakes.

The runtime does not presently make panic a safe workflow result. `Group.Go` invokes the child without recovery ([group.go](/Users/tyler/.codex/worktrees/loop-api-review/gimble/group.go#L42)), while `Run` emits `RunEnded` and `Complete` only after `scope.do` returns ([run.go](/Users/tyler/.codex/worktrees/loop-api-review/gimble/run.go#L51)). Recovering only in `Group.Go` would make identical misuse become a returned error in one location and an incomplete run in another; recovering every panic risks converting unrelated workflow defects into ordinary failures.

Claude found real ceremony, but the stated percentages combine all error plumbing, not just `Set`: its measurement reports about 18 `Set`-plumbing lines out of 36 total error-plumbing lines in the sprint fixture, and about 42 out of 58 in the deliberately evidence-heavy live workflow ([ceremony.txt](../ceremony.txt#L1)). I would change my mind if the value contracts made encoding infallible, Gimble defined one universal panic boundary that still produced complete durable run records, and representative workflows showed that ordinary Go helpers such as a local `mustSet` did not solve the readability problem. The public primitive should retain its error until then.

## 2. Claude's responses to my three fixes

Claude is right to push back on exporting `Get` before a workflow earns the name. Explicit Go variables and deliberate promotion with `Set` fit this repository's central rule, and the fact that nested group results do not automatically enter the planner context is useful isolation rather than a defect ([loop.go](/Users/tyler/.codex/worktrees/loop-api-review/gimble/loop.go#L130)). I withdraw `Get` as a current top-three recommendation.

The claim that no workflow yet demonstrates selective-read pressure is too strong, though. Claude's own live run is a planner/worker/reviewer workflow, and its second planner prompt contains both the planner role and the worker role while growing from 6.5 KB to 11.9 KB ([prompts.txt](../prompts.txt#L15), [REVIEW.md](../REVIEW.md#L294)). More significantly, `Loop` itself always builds the planner prompt from `ScopeText`, which renders every visible value; a workflow cannot select a smaller projection for that agent except by reorganizing scopes or keeping duplicate Go state ([loop.go](/Users/tyler/.codex/worktrees/loop-api-review/gimble/loop.go#L197), [scope.go](/Users/tyler/.codex/worktrees/loop-api-review/gimble/scope.go#L153)). The next step is an explicit workflow that needs two different projections of the same stored evidence. If that exercise cannot stay simple with Go variables and scope topology, a selective read or render primitive will have earned its name. Before adding one, Gimble should also stop stored values from forging context headings, as Claude's probe demonstrates ([probes/main.go](../probes/main.go#L150), [probes.txt](../probes.txt#L5)).

I agree with reranking the operator view below the single-channel planner contract and adapter cleanup. I would separate UI richness from observation correctness, however. A turn is currently recorded as successful before typed-output validation occurs ([session.go](/Users/tyler/.codex/worktrees/loop-api-review/gimble/session.go#L189), [session.go](/Users/tyler/.codex/worktrees/loop-api-review/gimble/session.go#L74)); the browser reducer can overwrite `cancelled` with `failed`, unlike the Go reducer ([index.ts](/Users/tyler/.codex/worktrees/loop-api-review/gimble/web/src/lib/observation/index.ts#L81), [store.go](/Users/tyler/.codex/worktrees/loop-api-review/gimble/internal/observation/store.go#L105)); and a reloaded snapshot contains no workflow scopes, tasks, decisions, or stored evidence ([snapshot.go](/Users/tyler/.codex/worktrees/loop-api-review/gimble/internal/observation/snapshot.go#L56)). Those are public truth-model defects, not optional dashboard polish. They remain my third priority even if richer monitoring and interaction arrive in later sprints.

Claude correctly maps my boundedness and ownership concern onto the planner and `Close` fixes. Its live exercise also adds evidence I did not have: the primitive supported a real planner, implementation agent, and parallel validators with an explicit evidence gate. That strengthens the capability case while exposing the context-growth problem more clearly.

## 3. Claims I would correct

The statement that every piece of agent input originates in a visible workflow line is overstated ([REVIEW.md](../REVIEW.md#L56)). Claude's own run reports that harness-level context caused an agent to create a worklog absent from the workflow prompt ([REVIEW.md](../REVIEW.md#L309)). Gimble makes its own prompt contribution inspectable; it does not control or expose the adapter's system prompt, resumed session history, repository instructions, memory, or tool environment. That boundary should be explicit because context composition is one of the library's main promises.

The statement that scope identity, data visibility, and lifetime all agree is also too broad ([REVIEW.md](../REVIEW.md#L61)). Logical session visibility follows the scope tree, but native session lifetime does not: the missing `Close` contract is precisely why an adapter resource can outlive its scope. The design is coherent at Gimble's event boundary and incomplete at the harness boundary.

The proposed structured planner response removes the dual-channel mismatch and unbounded YAML repair loop, but schema enforcement alone does not prove a valid plan. The runtime must still reject an invalid `Next`, duplicate or otherwise invalid task identities, and other cross-field conditions that JSON Schema cannot conveniently express. This is still a major simplification because each bad answer becomes one failed turn instead of an open-ended hidden repair protocol.

Finally, classifying `Close` failures as recording errors would blur two independent failure domains ([REVIEW.md](../REVIEW.md#L168)). `Complete.RecordingError` describes failure to persist the run; failure to release an owned harness session should be joined into the execution result and represented as cleanup failure in the event model.

## 4. Current recommendations

1. Replace the YAML-edit-plus-JSON planner protocol with one structured `plan` response, and derive the human backlog artifact and durable selected-task event from that response. This removes the largest accidental protocol while preserving the Loop's useful adaptive dispatch.

2. Add a required, idempotent `HarnessAdapter.Close` and make scope shutdown perform bounded cleanup with a cancellation-independent context, joined errors, and truthful final events. Treat a required `Interrupt` contract as a nearby but separately proven API decision.

3. Make the observation model authoritative across live events, stored logs, snapshots, and the browser: typed validation must affect turn outcome, cancellation must survive reduction, and snapshots must reconstruct workflow scopes, tasks, decisions, and evidence. Build richer monitoring and interaction on that consistent state.
