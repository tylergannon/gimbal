# Gimbal warm development

- decision: Appgen must not compile observed application packages merely to identify types. Polytype's source declaration uses the same codegen/cache path, while Gimbal's generator retains shared catalog contracts in the lightweight workflow package.
- finding: internal/builtin/workflow_pages.go embeds generated pages but has no callers. Shipped definitions already contain SourcePage JSON; removing the unused embed prevents output changes from rebuilding the next generator process.
- boundary: Stock projections consume workflow bodies, comments, and source positions. SKGO declaration-only reuse cannot be applied indiscriminately to Stock.
- boundary: Existing web/dev/run.mjs launches Vite and the Go app once; it does not rebuild backend edits. Integration will use SKGO's shared supervisor and retained generation Worker, preserving Vite's workflow preview plugin.
