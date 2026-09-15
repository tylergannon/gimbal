# Usable builtins

correction: User prioritizes being able to use Gimble before improving its viewer. Build supervised lfg, convert sprint planning, and revise Sprint input/scoped-data/supervision; do not substitute review/bakeoff workflows.
decision: df-sprint-plan, df-sprint-execute, and df-easy-loop-e2e are design references for conversion, not instructions to run their CLI orchestration verbatim. User explicitly requires Gimble's own primitives and simple ordinary Go.
decision: First entry is a CLI accepting a goal, claim, design, or plan, with agent guidance proportional to the task and explicit lfg/plan/sprint routes. Optional preference question is pending; graph work is independent.
decision: #216 records command executions as a Beta task; implementation here concerns usable workflows and does not add command-event API or viewer code.

correction: CLI normalization already produces absolute input paths; Sprint must preserve them rather than joining Repo twice. Native plan-to-execution exposed this before any model call; regression added and rerun passed.
correction: A successful task-local validation command is not a permanent sprint invariant. Only failed commands persist into final checking; explicit request Checks are the global checks.
correction: General repository publication must not stage prior edits or run transcripts. Nonlocal finish requires a clean worktree and ignored runtime artifacts; local finish is the default.
correction: Blocking question-reader goroutines leaked beyond scopes. Workflow reads are synchronous; the CLI owns closing stdin on cancellation and joins its watcher.
correction: Exact -check commands must reach all planning lanes and synthesis, not only execution. Added input/prompt propagation and independent closure review.
decision: Verification commands remain optional for one-session LFG. Without them success means worker completion, not an independent acceptance verdict; requiring a shell check for every task would defeat the user's ease-of-use priority.
evidence: Native LFG, guided work-to-LFG, three-provider planning, and plan-to-Sprint execution passed with durable run records and fixed fixture checks. Native archives, artifacts, limits, and review adjudication are in ephemeral/attest/builtins/PROOF.md and round-02/03 reviews.
