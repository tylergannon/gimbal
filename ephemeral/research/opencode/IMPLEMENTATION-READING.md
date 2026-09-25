# Research entry point for the OpenCode implementation

All research is available locally in this worktree. Do not repeat the broad survey. Use the semantic indexes to navigate to primary sources and use current code for the Gimbal contract. This index is navigation, not an implementation checklist.

Authoritative end state: /Users/tyler/.codex/worktrees/c671/gimbal/ephemeral/requirements/opencode-adapter.md. Latest decisions: legacy API, one shared server, synchronous POST plus shared SSE initially, raw event capture, narrow generated types and handwritten client, explicit opencode model prefix. Older async-only recommendations are superseded.

## Reports and notes

- /Users/tyler/.codex/worktrees/c671/gimbal/ephemeral/research/opencode/harness-adapter.md — original CLI/server comparison and event/API compendium.
- /Users/tyler/.codex/worktrees/c671/gimbal/ephemeral/research/opencode/new-api-and-codegen.md — independently reviewed generator comparisons, measured output and API gaps.
- /Users/tyler/.codex/worktrees/c671/gimbal/ephemeral/research/opencode/legacy-adapter-design.md — latest source audit and decisions; implementation brief supersedes historical advice.
- /Users/tyler/.codex/worktrees/c671/gimbal/ephemeral/worklog/opencode-harness-research.md — user corrections and research pitfalls.
- /Users/tyler/.codex/worktrees/c671/gimbal/ephemeral/research/opencode/ — original and follow-up briefs remain available for context.

## Semantic indexes and complete corpora

- /Users/tyler/.codex/worktrees/c671/gimbal/.gimbal/research/opencode/INDEX.md — original fifteen-topic semantic index, primary sources and clips (initial release1.18.27).
- /Users/tyler/.codex/worktrees/c671/gimbal/.gimbal/research/opencode-rerun/INDEX.md — successful workflow's follow-up semantic routing tree. Topic indexes point to primary sources, clips, generator evidence and audits (release1.18.31).
- /Users/tyler/.codex/worktrees/c671/gimbal/.gimbal/research/opencode-next/INDEX.md — earlier follow-up index and original evidence cache. The entire directory is available, including completed trials and final reviews.
- /Users/tyler/.codex/worktrees/c671/gimbal/.gimbal/research/opencode-legacy-design/events/findings.md — exact legacy event scope, message/status ordering and completion exceptions, with adjacent freshly downloaded pinned source.
- /Users/tyler/.codex/worktrees/c671/gimbal/.gimbal/research/opencode-legacy-design/surface/findings.md — minimal endpoint/type surface, steering and fork/schema behavior, and cross-generation execution trace.
- /Users/tyler/.codex/worktrees/c671/gimbal/.gimbal/research/opencode-legacy-design/server/findings.md — shared process, transport/configuration and existing Gimbal integration.

Historical indexes contain some superseded interpretation; original evidence and final report qualifications win. In particular: all generator agents finished; no pending results remain. Custom generator LOC estimates were not measured. Different session lists never proved separate identity; newer and legacy APIs share session rows but have distinct input/message/execution paths. Newer steering is not an established bridge to a running legacy turn. Actual HarnessAdapter is in harness.go, not a hypothetical TurnInput interface.

## Exact inputs and experiments

- /Users/tyler/.codex/worktrees/c671/gimbal/.gimbal/research/opencode-next/input/opencode-generate.json — real combined OpenAPI from installed OpenCode1.18.31; input/README.md has provenance.
- /Users/tyler/.codex/worktrees/c671/gimbal/.gimbal/research/opencode-legacy-design/surface/ref-closure.json — initial legacy reference closure analysis, not a generated/compiled result.
- /Users/tyler/.codex/worktrees/c671/gimbal/.gimbal/research/opencode-next/trials/oapi/RESULT.md — stable and experimental oapi-codegen results. Nearby tools/configs, actual generated models, compile logs and union probes remain available. Stable v2.8.0 components-only is the demonstrated useful approach; new legacy subset still needs generation.
- /Users/tyler/.codex/worktrees/c671/gimbal/.gimbal/research/opencode-next/trials/ogen/RESULT.md — ogen experiments; TYPES-ONLY.md and reviews explain omitted coverage.
- /Users/tyler/.codex/worktrees/c671/gimbal/.gimbal/research/opencode-next/trials/alternatives/RESULT.md — other generators and libopenapi; EVIDENCE-REVIEW.md, QA-NOTES.md and FINAL-QA.md distinguish actual proof from earlier overclaims.

Every listed directory and its contents is readable by workflow agents. Preserve source evidence; new implementation probes belong under a separate .gimbal implementation cache. Do not commit raw research downloads or logs. The unrelated third_party/opencode reducer oracle has a different pin and purpose.
