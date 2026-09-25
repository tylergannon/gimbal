# Native Go coding harness research

Start with the coordinator's [implementation plan](planning/PLAN.md), [module assignments](planning/MODULES.md), and [dedicated workflow design](planning/WORKFLOW.md). Planning is complete for review; implementation has not started. [MERGE-NOTES.md](planning/MERGE-NOTES.md) records which draft proposals were rejected.

The earlier [RECOMMENDATION.md](RECOMMENDATION.md), [REVIEW-STATUS.md](REVIEW-STATUS.md), [BRIEF.md](BRIEF.md), and [SOURCES.md](SOURCES.md) contain the research context and pinned local sources.

Three detached Gimbal research-document workflows produce independent reports and semantic indexes:

| Study | Assignment | Report destination | Semantic index destination |
|---|---|---|---|
| Go candidates and correctness | [candidates-goal.md](candidates-goal.md) | candidates/report.md | candidates/corpus/INDEX.md |
| Upstream Pi semantics | [upstream-goal.md](upstream-goal.md) | upstream/report.md | upstream/corpus/INDEX.md |
| Ownership and parallel port plan | [port-plan-goal.md](port-plan-goal.md) | port-plan/report.md | port-plan/corpus/INDEX.md |

All three reports completed. They contain known errors and are not accepted implementation contracts; use the coordinator's source-checked recommendation and plan. Their local semantic indexes are retrieval aids, not factual authority.

Runtime state and launch receipts are local to /Users/tyler/.local/share/gimbal-pi-research. Source repositories are in its sources directory; experiments belong in scratch. The Gimbal worktree is /Users/tyler/src/gimbal-pi-research on codex/pi-go-research. Research defaults are Gemini 3.8 Flash medium for planning/research/indexing/supervision and Gemini 3.1 Pro high for authorship/editorial review.

Tyler initially requested detached execution, then asked the coordinator to poll every few minutes. All research and planning draft processes have now completed. A completed report is not evidence of candidate qualification.
