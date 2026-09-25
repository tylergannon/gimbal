# Investigation: Claude waiting, continuations, and structured completion

## Desired outcome

Establish how Gimbal can preserve Claude's background capabilities while
returning a structured result that represents the end of the assigned task.
This is an investigation, not authorization to implement a product repair.
Tyler wants a full accounting of the connected event stream when Claude
answers that it is waiting, and ways to deterministically distinguish that
answer from authoritative task success or failure.

Answer these questions with explicit evidence and uncertainty:

1. What happens to model generation, the native event stream, SDK iteration,
   stdin/stdout, the process, and background tasks after the initial result?
   Distinguish these lifetimes. Is any closure necessary, or caused by the host?
2. Which observable signals distinguish waiting from assignment completion?
   For each proposed rule, state its guarantee, counterexample, race, and
   source/version. Determine whether native protocol alone suffices. Consider
   task lifecycle snapshots, origin, prompt identifiers, result sequence,
   stop/terminal reasons, structured payloads, hooks, and explicit completion
   tools or acknowledgements where useful. Do not assume a status field is
   truthful simply because it validates against a schema.
3. How can the host keep listening between results and calls? What becomes of
   attribution, ordering, accounting, cancellation, queued prompts, and a
   notification racing the next prompt or scope teardown?
4. If the connection or process closes, what can actually be recovered from
   resumption, transcripts, task output files, or a reconnectable transport?
   Distinguish lossless event recovery from reconstructing partial state and
   re-running work. Do not promise exactly-once recovery without support.
5. How do we obtain the eventual task-wide structured answer after wakeups,
   including failed background work, multiple dependencies, a persistent
   service, and schema changes or clearing between calls?
6. Recommend the smallest viable contract and implementation direction. If
   deterministic task completion is unknowable from existing native signals,
   show an indistinguishable pair or concrete counterexample and identify the
   minimum additional declaration needed. Avoid new orchestration frameworks.

## Local starting material

Repository: /Users/tyler/.codex/worktrees/286c/gimbal
Issue: /Users/tyler/.codex/worktrees/286c/gimbal/ephemeral/research/claude-lifecycle/issue-317.md
Prior investigation: /Users/tyler/.codex/worktrees/286c/gimbal/ephemeral/research/claude-lifecycle/317.md
Temporary earlier probes and raw captures: /private/tmp/gimbal-317-investigation/
Original native transcript: /Users/tyler/.claude/projects/-private-tmp-scrabbler-drag-drop-eval/deee5fb6-61a3-4c41-b827-d5033fa663ae.jsonl
Pinned SDK: /Users/tyler/go/pkg/mod/github.com/tylergannon/claude-agent-sdk-go@v1.1.1-0.20260912021749-9a4ffeca77cc
Relevant code: claude/claude.go, claude/events.go, harness.go, session.go,
internal/workflows/validateproduct/, and the SDK's client.go, messages.go,
protocol.go, transport.go. Resolve paths against the repository above.

Prior observations on CLI 2.1.270 / claude-haiku-4-5-20251001:
- A deliberately elicited waiting response produced a successful structured
  result. Keeping stdin and the process alive allowed automatic completion
  notification, a second structured result with task-notification origin,
  and a correct later prompt answer under the same unchanged schema.
- The current adapter closes its client per RunTurn. Closing after the first
  result and resuming produced an empty successful orphan-notification result
  before the actual answer to the new prompt. Gimbal consumed the empty result.
- Explicit TaskOutput(block=true) and disabling background tasks each avoided
  the finite-task symptom in bounded probes. Disabling background work is not
  the desired design: it would constrain Claude significantly.
These are evidence to inspect, not instructions to agree with earlier advice.

## Scope and deliverables

Work independently, distinguishing directly observed behavior, documented
contract, source-derived inference, and conjecture. Use primary sources for
technical claims; online reports may guide research but must be labeled as
reports, with versions and dates. Keep fetched evidence local and cite exact
paths/lines or source URLs. Read relevant repository instructions first.

Live native probes are authorized and must use Claude Haiku (Codex luna or
Gemini Flash if comparison is necessary). Use small harmless workloads,
isolated directories, bounded duration, and cleanup only your own processes.
Do not change installed settings, production code, the SDK checkout, existing
runs, or the original incident workspace. Temporary code and raw run output
belong under your assigned /private/tmp directory, never committed. Checked-in
deliverables are concise research/design notes, not transcript dumps or proof
programs. Report live observations in your final message as well.

You are not alone in this codebase. Own only the report assigned in your brief;
do not edit another agent's files or revert others' changes. The parent owns
git commits and the common worklog. Do not commit, push, create PRs, or spawn
further agents. Keep any private scratch worklog in your assigned temp directory.
