# Research document: Claude background continuation contract

Read /Users/tyler/.codex/worktrees/286c/gimbal/ephemeral/research/claude-lifecycle/briefs/common.md
and the local issue and prior-investigation files it names.

Collect and arrange evidence for a Gimbal maintainer deciding how to support
Claude background tasks without mistaking a waiting result for assignment
completion. This lane is documentary research; independent Sol and Terra agents
are performing live probes. Do not launch additional native agent experiments.

Use the workflow's five research groups for these adjacent areas, preferably
one substantial topic per group rather than unnecessary fragmentation:
- Official Agent SDK and CLI contracts for streaming input, results, background
  tasks, process shutdown, persistent sessions, and hosting.
- Current official SDK source, exported types, changelog/history, and relevant
  Go SDK code: origins, task states, prompt/result correlation, queued work,
  structured-output enforcement, hooks, and any reconnectable transport.
- Structured result semantics across automatic wakeups, schema changes, task
  failure, and explicit task-completion declarations.
- Connection/iterator/process lifetime and recovery: available events, replay,
  resumption, orphan notifications, output files, and data-loss boundaries.
- Relevant public issues and firsthand posts: reproduceable waiting/early-result,
  empty-result, lost-continuation, and shutdown cases, including proposed fixes.

Consult official docs and primary GitHub source for technical conclusions.
Online posts are requested too: use them to locate concrete cases, and clearly
separate user reports from confirmed contracts. Follow source links and release
versions; mirrors and multiple copies of one page are not independent evidence.
Retain fetched originals locally, with retrieval/source metadata and precise
passage references. Cite the original URL as well as local paths. Search using
URLs learned from the local starting notes and sources; all assignments and
initial context have already been supplied locally.

The report must account for the complete event/lifetime sequence, assess
candidate deterministic completion rules and their counterexamples, state what
is recoverable after a closure, and explain how an eventual structured response
can represent whole-task completion. Distinguish protocol guarantees from
source-derived inference and application choices. If native waiting intent is
not observable, say so and identify the minimum extra contract to investigate.
Do not manufacture a definitive solution where evidence is absent.

Keep all downloaded sources, indexes, and clips under:
/private/tmp/gimbal-317-literature/corpus/
The only repository file this workflow may create or revise is:
/Users/tyler/.codex/worktrees/286c/gimbal/ephemeral/research/claude-lifecycle/literature.md
No application changes, git operations, additional worklogs, PRs, or issue posts.
Other investigators are active: do not change their files. Produce a concise
report within 6500 tokens; detailed source passages belong in the local corpus.

## Source audit during collection

Before curation, authoring, or editorial acceptance, read
/private/tmp/gimbal-317-literature/verification/corrections.md and the fetched
issue originals beside it. Correct outdated status and unsupported causal
claims in the corpus; do not carry them into the report as current guarantees.
