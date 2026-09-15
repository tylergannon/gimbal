# Functional work before the viewer

Tyler rejected starting #173 before graph recording and rejected the bug-heavy next-five recommendation. That recommendation is withdrawn; only #201 is confirmed in flight.

Accepted sequence: #201 extraction → #215 recording the generated graph with each run → #173 viewer. #215 is a Beta issue and #173 now states this prerequisite explicitly.

Additional functional proposals, not dispatched or marked in flight:

- Record actual command execution: scope, start/end, exit status, and output, so an agent call and a command are both inspectable operations. Ordinary os/exec remains explicit in the workflow. Static discovery alone is not runtime telemetry. Agree the smallest explicit recording mechanism before implementation; do not introduce a high-level execution wrapper by default.
- Write a builtin code-review workflow: inspect a local diff, obtain independent reviews, reconcile findings, and produce an actionable review artifact. Readable ordinary Go, constant context keys, no automatic publication of the review.
- Write a builtin implementation-comparison workflow: run two candidates in isolated worktrees, apply the same checks, and produce a comparison with evidence. Do not automatically merge a winner. Keep the tactic inline, rather than adding an exported BakeOff wrapper.

Command recording and builtin program authoring can progress alongside extraction, with runtime ownership coordinated between command recording and #215. Recording integration depends on #201's generated format; viewer implementation waits for its demonstrated persistence. These are useful workstreams, not five fabricated independent starts.

Existing #150 is a substantial functional proposal (filesystem-backed scoped context), but explicitly deferred and still contains unresolved composition and access decisions. #105 requests read APIs without a current workflow need. Neither is a ready substitute for the concrete graph/command/builtin goals. #211 is explicitly parked and upstream-dependent. Small bugs remain Beta work and can be handled when they block these deliveries or during cleanup.
