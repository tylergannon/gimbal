# Issue 257 workflow-role reference

decision: `roles.go` is the sole authored role catalog; generated documentation preserves declaration order and fails on absent or non-10-to-20-word descriptions, per issue #257 and Tyler's approved half-page design.
decision: Delegate implementation and completion proof to Sol in a fresh worktree based on current `origin/main`.
decision: Generate the complete Svelte role-reference page from Go syntax and type information, so resolved constant values and comments come only from `roles.go`; authored site code adds navigation, not catalog data.
friction: `go generate ./...` rewrites existing skgo/polytype TypeScript before the repository's established `web` formatting step; two generate-plus-`vp fmt` cycles were clean, and the new role page itself is byte-stable without post-processing.
