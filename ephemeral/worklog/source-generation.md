# Source generation

- decision: Reuse the established task worktrees and baselines; this stage implements the saved generation work order, not public API or deployment.
- decision: Typed syntax drives local bindings, captured operation arguments and explicit workflow control flow. Source helper effects outside the bounded admitted surface are diagnosed before emission.
- decision: Remove handwritten outcome structs from generated entry contracts; source entries return error. Use adapter/event observations for correspondence rather than invent source results.
- useful_negative: Independent review found mutable imported globals bypassed expression rejection, typed constants lost their Go type, ignored Scope produced an unused temporary, unnamed callback parameters could panic, and ordinary root-context use changed types. Added diagnostics or faithful emission and regression cases.
- useful_negative: Shared planner glue still hardcoded the loop name and activity workspace. Source-generated code exposed this specimen assumption; pass the authored loop name and use the actual planner session workdir. A renamed-loop source variant now compares prompts and events.
- evidence: Initial seven source-only variants passed against actual source/target execution. An overly broad helper diagnostic assertion failed; fixed the diagnostic guard order rather than weakening it. Final evidence will be bound to the final code revision.
- correction: GIMBAL108 requires Generate prompts to remain compile-time constants. Emit the typed source constant inside each generated activity; reject dynamic prompts rather than exempt generated code from the linter.
- correction: Preserve optional live tests' intermediate result assertions by reading durable turn records after removing handwritten outcome return structs. Exercise those same readers/assertions in deterministic paired execution; do not claim a fresh paid-provider run.
- friction: The first failed-lint commit hook continued running its tests. Reusing its log path for a second hook caused overlapping writes and sparse output. The second commit exited zero and produced 6ea8e8d6; use unique logs and await failed hooks before retrying.
