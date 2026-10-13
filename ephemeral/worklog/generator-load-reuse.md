# Generator load reuse

2026-10-13. Based on `f819aef8a08c47c96eaea01fbd03b22e6c1db2e5` (easier test-cost fixes); this change owns `internal/generate` only.

friction: Stock independently loaded eight workflow packages, Definition reloaded the same overlay for its inspected and runtime graphs, and x/tools v0.51.0 reparsed dependency source whenever Config.Overlay was set. -> Batch roots and keep one immutable source snapshot per generation; each graph walk still has fresh mutable extractor state.

correction: Removing NeedDeps alone loses the source-import diagnostic for non-inlinable helpers whose exported types do not reference Gimbal. -> Load cheap source-import metadata separately from root syntax/types. Added a regression for the body-only helper, changed dependency source, and a newly added dependency file; deliberately deleting the diagnostic caused the new test to fail.

decision: Borrow SKGO's existing-file Go-command-overlay plus root-parser approach. Missing package-only generated placeholders contain no declarations and can be omitted; arbitrary new-file overlays and external package drivers retain ordinary go/packages handling. No persistent cache, test parallelization, or generalized temporary module staging was introduced.

## Matched measurements

Same task worktree and authored source, shared machine caches. Timed actual `go run ./internal/generate/stockgen .` before/after: **7.54s -> 1.50s**, including go-run compilation. All 19 invocation/page/catalog/index outputs were SHA256-identical between the old and new generator. The baseline produced formatting-only index churn; that was restored and is not in this change.

One uncached generator package run before/after (`go test -count=1 -json ./internal/generate`): **40.086s -> 8.597s**, both passed. Stock recovery **12.06s -> 1.07s**; external consumer generation/changed-source compilation **4.68s -> 1.84s**. Existing graph ownership, stale-output recovery, comments, guide/roles/module, and inspection/runtime projection checks passed. The optimized run also includes the new source-import/freshness regression.

The existing Stock test was then extended to remove all eight generated graphs before extraction; that focused test passed in **1.476s**. Focused `go vet ./internal/generate/...` passed; `golangci-lint run ./internal/generate/...` reported **0 issues**. No repository-wide test suite or application rebuild was run for this generator-only change. Timing logs/profiles remain outside Git.
