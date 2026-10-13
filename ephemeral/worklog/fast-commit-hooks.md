# Fast commit hooks

correction: User requires commit hooks to finish within 5–10 seconds. Whole-repository generation, compilation, tests, typechecking and browser tests are validation work, not staged-file checks. Keep cheap index-based policy and formatting checks; preserve full validation in explicit commands and CI.

friction: File globs only selected the old jobs; their ./... and web commands still processed the full repository. Modernize auto-fixed unrelated and generated files. Staged formatting now checks index blobs without rewriting or restaging partial work.

friction: Golangci analysis existed only in hooks and needed explicit CI ownership. Preserve default analysis through the official prebuilt installer rather than compiling the analyzer in every CI run. Modernize is an explicit opt-in rewrite; making its existing generated-code findings a new fatal CI gate would expand the task.

correction: Existing research is uncommitted in Gimbal View's testing-infrastructure-research worktree, not obtainable from the unchanged branch commit. Later fact checks invalidate naive dependency-load flag removal and unrestricted generator test caching. Preserve meaningful test assertions and fresh generator checks when optimizing validation.
