# Terra: persistent listening, recovery, and final structured output

Read /Users/tyler/.codex/worktrees/286c/gimble/ephemeral/research/claude-lifecycle/briefs/common.md.

Independently evaluate waiting versus authoritative completion, concentrating
on session lifetime, continuation ownership, and recovering a final structured
answer. Investigate keeping one native process and event reader alive across
calls; distinguish closing an SDK response iterator, closing stdin, transport
loss, and process death. Determine what resumption can and cannot recover.
Inspect and, where feasible, probe changing/clearing the output schema within a
live session and how a late notification interacts with those changes.

Evaluate small explicit completion contracts if native signals are insufficient:
what can the host enforce mechanically, what still relies on model declaration,
and how are persistent services exempted without losing awaited work? Include
failure, cancellation, scope teardown, and any current HarnessAdapter event
callback constraints that materially limit the approach. No product edits.

Write research notes to:
/Users/tyler/.codex/worktrees/286c/gimble/ephemeral/research/claude-lifecycle/terra-lifecycle.md
Use /private/tmp/gimble-317-terra/ for scratch programs, downloads, and captures.

Make your final answer state the smallest viable lifetime/completion contract,
the data-loss boundaries, structured-result handling, concrete evidence, and
unresolved assumptions. Send material findings to the parent as they emerge;
do not wait for other investigators.
