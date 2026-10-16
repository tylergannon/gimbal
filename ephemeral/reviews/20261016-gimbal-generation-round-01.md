# Gimbal development generation integration review, round 1

Outcome: material findings remain.

## Target and evidence

Reviewed the complete working-tree upgrade from `143ccd784ae852d05ec34564483e1c0e519ccd46` in `/Users/tyler/.codex/worktrees/gimbal-skgo-dev/gimbal`, including dependency/tool-environment changes, the new helper/configuration and producer selection, generation publication/cache changes, surrounding existing generators, and the tests. Authoritative scope is the user's request to retry the Gimbal upgrade and demonstrate the real application using the released SKGO/Polytype interfaces, together with `AGENTS.md`, `docs/definition-of-done.md`, and `ephemeral/plans/dev-generation-callback.md`.

Inspected the Gimbal release skill, existing generation declarations and `schema.go` selections, Stock's built-in registry, the production appgen command, and released SKGO v0.35.0's helper refresh/start behavior. Ran `go test ./internal/devgenerate ./internal/generate ./internal/generate/devgen`: passed; the generator package took 29.731 seconds. `git diff --check` passed. Production dependency inspection of `./cmd/gimbal ./web` found none of the new `internal/devgenerate`, helper-main, or `skgo/devgen` packages.

The caller is conducting the live browser/application proof and production build in parallel; no live application result was supplied before this review. Tests demonstrate focused publication, selection, partial-error, no-op, and repair behavior, but they do not alone establish the complete running edit loop or restored project locks. The preexisting `NewSession.svelte` change was preserved. No implementation files were edited by this review.

## Finding

### Issue: Stock-owned output directory removals do not select repair

`internal/devgenerate/generate.go:106-119` recognizes Stock output events only when they are within `web/src/lib/workflow-design/generated`, or exactly match an invocation/legacy-command filename. It does not recognize a directory event for an ancestor such as `internal/invocation`. This differs from the ordinary producer output handling at lines 92-95, which checks both directions for directory events.

A concrete trigger is moving the `internal/invocation` directory out of the watched tree in one filesystem rename after helper initialization. The watcher can report the directory rename rather than one event for every descendant. `isDirectoryEvent` sets `sourceChange`, but no graph producer owns that directory, and none of the Stock filename comparisons match it; `stockSelected` remains false. Rediscovering source memberships cannot reconstruct Stock ownership because its products are outside the declared producer outputs. Consequently cached Stock products never republish the missing invocation/registry files, and the application rebuild remains broken until the developer manually restores them or restarts the helper. A rename/removal of an ancestor of the generated workflow-design directory has the same selection gap.

The accepted plan explicitly requires directory events to reconcile previous/current membership and missing owned outputs to select their owner for repair. Apply the same ancestor-aware directory matching to Stock's owned outputs, and add a focused test using only a directory event, including `internal/invocation`. An initialized Stock cache should repair its output directory without re-extracting unaffected workflows. This is a selection/repair defect; equality-aware Stock publication itself is already covered by the new tests.
