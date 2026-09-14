# Beta workflow authoring: statically legible shape

Tyler's proposed rule of thumb, 2026-09-14: “Workflow shapes should be comprehensible by static analysis, as a proxy for degree of complexity.”

Suggested wording: **Write workflows so their possible shape is visible in source: what work can run, in which scopes, with what concurrency and supervision. Runtime data may choose branches, repetitions, and inputs; it should not conceal the structure of the workflow.**

This is a simplicity preference, not a requirement to predict the run or prove arbitrary Go safe. Each authoring diagnostic needs a recognizable forbidden construct, a simpler permitted example, and a bounded implementation. For now, lint-clean workflow structure must be completely statically recoverable. An unsupported structural construct fails the gate until supported or deliberately permitted; runtime data and branch choices need not be predictable. Lint errors fail the linter/gate; Go compilation and direct execution remain available.

## Rule candidates

Dynamic-worker dispatch and nonconstant Set/SetJSON keys are now approved hard diagnostics. The remaining rows are recommendations for review, grounded in current API/design rules where noted; they are not new acceptance requirements merely because they appear here.

| Rule | Why it helps | Straightforward Beta detection | Status |
| --- | --- | --- | --- |
| Constant context keys | Recorded fields are explicit at each write site | go/types constant-value check on every Set/SetJSON key | Approved; current builtin sprint must fail |
| No dynamic worker dispatch | Possible workflow functions are visible at their call sites | Function collection lookup invoked directly or through simple local aliases | Approved; added to #162 |
| Constant structural names | A source site has a stable readable name without predicting runtime data | Go constant expressions at Scope, Group, Group.Go, Loop, NewSession, and Fork naming arguments | Existing repository stance; recommend adding to lint |
| Structured workflow concurrency | Child work has an explicit scope and join | Recognizable workflow operations launched by raw `go`; known Group paths that exit without Wait | Group ownership is existing API policy; recommend broadening the Set-only raw-go check |
| No recursive workflow orchestration | Repetition is visible as a loop region, rather than hidden in recursive helper expansion | Direct/mutual recursion among statically resolved workflow helpers where a cycle itself performs workflow operations | New proposal; review before enforcing |

The names for new diagnostic identifiers should be assigned once the rule set is accepted. Do not invent a catalog of empty analyzer features.

### Constant structural names

Prefer `Group.Go("check", ...)` inside a loop. The template is “check”; observed instances receive runtime identity. A runtime repository name, task description, command argument, or model choice is data. Keep it in the instance's data or description rather than encoding it into the structural name. Named Go constants and constant expressions are fine; duplicate labels in different scopes are fine. Set/SetJSON keys must now also be compile-time constants; see constant-context-key-rule.md.

### Structured concurrency and joins

Use Group.Go for concurrent workflow work and Wait for its join. This lets the map show branches and the continuation. Restrict the check to workflow orchestration and recognized helper bodies; do not ban ordinary goroutines throughout adapters or imported libraries. A nested raw goroutine performing workflow work is still raw concurrency even if its enclosing callback was passed to Group.Go.

Flag known missing joins on ordinary analyzed return paths. Resolve visible joins through supported direct helpers; an unresolved workflow join is incomplete shape and fails the structural gate. A resolved join does not claim an exhaustive lifetime proof. General panic/recover, escape, and every-path lifetime proofs are outside the initial check. Existing wrong-context checks must also accept ordinary cancellation/deadline derivations of the correct scope context.

### Workflow recursion

The proposed ban concerns cycles that perform workflow operations, not recursive data processing. A recursive JSON/tree traversal used to construct a prompt need not fail. An ordinary Go `for` loop can visibly repeat a direct workflow operation without making the number of iterations statically known; planner-directed Loop.Tasks is also valid. Neither requires inventing a new workflow wrapper. Keep this a proposal until a representative workflow demonstrates the boundary cleanly.

## Rules not to add simply to satisfy the analyzer

- A blanket ban on function parameters, interfaces, maps, loops, dynamic command arguments, model/provider choice, or dynamic values under constant context keys.
- A blanket ban on ordinary helper functions or inherited sessions used in child scopes.
- “Every runtime event must have an exact source match” as a workflow-authoring check. That is an instrumentation requirement.
- “No context or session can escape” as an exhaustive proof obligation. Obvious global storage or post-scope use can become bounded diagnostics, but proving all closure/storage cases would take us into the hard analysis the user wants to avoid.
- “Every supervisor option must be written inline.” Fixed helpers and visible conditional attachments may be perfectly legible. Unsupported option construction needs a concrete case before a new restriction is justified.

## Review test for any proposed rule

Can a reader see the possible workflow shape in the permitted rewrite, and can the analyzer recognize the violation without speculative whole-program reasoning? If a clear construct is unsupported, either implement that support or keep it outside the lint-clean subset for now; reconsider restrictions against real workflows rather than adding speculative analysis. If the rewrite clarifies orchestration, the rule is a useful Beta candidate.

Related: [dynamic worker rule](dynamic-worker-lint-rule.md), [Beta linter #162](https://github.com/tylergannon/gimble/issues/162), [Beta graph extraction #201](https://github.com/tylergannon/gimble/issues/201).
