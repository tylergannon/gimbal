# Small deliveries toward the graph and linter

Current decision supersedes earlier permissive text below: #162 must hard-fail nonconstant Set/SetJSON keys, including the existing builtin sprint, and accepted Beta workflows must have completely recoverable static shape. See constant-context-key-rule.md and the current #162/#201 bodies. These are task drafts, not implementation claims.

## First: ship the useful part of #162

Workflow authors get build-time diagnostics for straightforward Set mistakes before a run spends model time. Cover local duplicate constant keys on a known context, reserved `task` on a direct Loop task context, direct Background/TODO writes, and obvious captured-parent/raw-go writes. Handle simple aliases if inexpensive. Avoid guessing on unknown/derived contexts; false positives must not force a valid workflow rewrite. Dynamic keys continue working.

Use the existing standalone go-tool pattern in the repository's vet gate. Demonstrate each included check with one failing and one valid source example; built-in sprint passes unchanged. A fixture with dynamic keys and a valid context.WithCancel-derived child also passes. Document the checks actually implemented. No claim of complete scope or session safety.

## Next: produce and display one useful program graph

go generate emits inspectable JSON for the built-in sprint and one small caller-module example. It contains scope regions, session ownership, agent-call sites, command execution sites, visible supervisor attachments, and branch/repetition structure. Follow the few statically resolved local helpers needed by sprint. Dynamic argv/key expressions are retained as expressions, not evaluated. An indirect target stays explicitly unresolved.

Display that manifest in a simple read-only scope diagram with commands as visible operations and supervision as a distinct edge. Both branch alternatives appear before execution. This delivers a program view without depending on complete lifetime linting or predicting the planner's task count. General interface dispatch, recursive summaries, and arbitrary helper expansion are excluded from this slice.

## Then: improve the run view using facts already available

Build on #173's scope/turn timeline. Show supervisor attachments and landed/dropped steers from their existing lifecycle records. For one real command in the built-in workflow, evaluate a named Scope around ordinary os/exec with explicit command outcome values; show that operation running and finished, and reproduce it from the log. A scope interval is labelled as an operation interval, not exact process timing.

Link unambiguous runtime instances to the manifest. Group indistinguishable sites or label the match unresolved. The view must not invent which branch ran or treat absence as proof that a site can never run. A live example showing one command, a supervisor attachment, and repeated task scopes is sufficient for this slice; exact process telemetry and complete source lighting can follow only if the view needs them.

## Deliberately deferred

- A general Go ownership/escape proof, full interprocedural key analysis, and pointer analysis.
- Requiring constant Set keys or rewriting workflows to please the linter.
- Exact mapping of every unnamed Generate site and every schema retry.
- A default function-call hairball, semantic dependency inference from prompt text, or shell-program analysis.

The separate constant-key study informs an authoring recommendation. It does not block these deliveries.
