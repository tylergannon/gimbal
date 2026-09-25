# Early Jev projects relevant to Gimbal

These projects show possible mechanics, not proof that Jev improves Gimbal outcomes.

- [Foreman](https://github.com/thruwire/foreman) runs a separate, debounced Jev observation loop beside Codex/OpenCode. It batches health and progress questions over a bounded snapshot, then ordinary policy code selects a directive. Its author calls it an architectural experiment rather than an established improvement over a conventional coding-agent harness.
- [Abide](https://github.com/coldteadotai/abide) checks agent edits or completed turns against project rules. Its author-reported replay found 11 confirmed turn alerts among 15 flags versus 10 confirmed edit alerts among 39 flags. This suggests that Gimbal should compare stable turn boundaries with frequent raw edit checks, not assume that more polling always improves precision.
- [jev-reranker](https://github.com/hotchpotch/jev-reranker) scores candidate passages while retaining their original indices and metadata. This is a direct model for filtering a Gimbal research corpus while keeping untouched excerpts and citations available to the downstream researcher.

All three are young, project-specific implementations. Their thresholds and reported outcomes are not Gimbal acceptance criteria.
