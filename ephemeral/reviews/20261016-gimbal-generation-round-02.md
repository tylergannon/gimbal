# Gimbal development generation integration review, round 2

Outcome: material findings remain.

## Target and evidence

Re-reviewed the complete working-tree upgrade from `143ccd784ae852d05ec34564483e1c0e519ccd46` in `/Users/tyler/.codex/worktrees/gimbal-skgo-dev/gimbal` against the user's request for the real Gimbal upgrade/edit loop, repository instructions, `docs/definition-of-done.md`, and the accepted `ephemeral/plans/dev-generation-callback.md`. Reviewed surrounding source, package boundaries, generation ordering/publication, the helper configuration/recipe, and the independent tests. No implementation files were edited.

The first-round Stock repair issue is resolved: Stock's fixed owned-output inventory now participates in ancestor-aware directory matching; initialized cached products repair missing generated packages before discovery; publication creates the invocation directory. The tests exercise invocation/design-directory renames, cached repair without re-extracting unrelated workflows, and repair before a later metadata failure.

Ran `go test ./internal/devgenerate ./internal/generate ./internal/generate/devgen`: passed (`internal/devgenerate` 0.474s, `internal/generate` 60.280s). `git diff --check` passed. Production dependency inspection of `./cmd/gimbal ./web` excludes the new Gimbal tooling packages and `skgo/devgen`. The parent additionally reports passing complete Go tests, Svelte checking with zero errors/warnings, CLI releasecheck/hooks, and production build. Those checks do not establish the missing live application proof.

The parent reported that the actual development command on port 8198 had not served after about three minutes, while the coordinator PID 8398 and helper PID 8707 remained alive and the helper repeatedly executed Go commands. Investigation then identified the scratch-file invalidation loop described below. The parent stopped that coordinator. A fix began while this artifact was being finalized; the complete stable fix and actual serving application need the next review round.

## Finding

### Issue: OpenAPI scratch files keep full startup generation permanently obsolete

The reviewed `internal/devgenerate/commands.go` implementation created temporary output with `os.CreateTemp(dir, ".gimbal-types-*.go")` and temporary configuration with `os.CreateTemp(dir, ".gimbal-openapi-*.yaml")`, where `dir` is the watched application `opencode` directory. It removed those files at the end of every OpenAPI generation invocation.

SKGO intentionally tracks raw application events independently of compiler source identity, so these create/write/remove events advance the raw selection epoch while the helper is processing its full startup request. The response cannot acknowledge that now-obsolete selection. Full selection remains pending, so the next request again invokes OpenAPI, again creates/removes scratch files, and again invalidates itself even when the application-owned `types.gen.go` bytes are equal. The observed actual application remained unavailable while the same helper repeated approximately six-second generation cycles. Equality suppression at final publication cannot stop events caused by these intermediate files.

This blocks the primary requirement: the normal `just dev` entrypoint must actually start Gimbal, and successful equal generation must converge. The existing OpenAPI unit fixture validates byte/mtime suppression of the final output but has no real watcher, so it did not expose the failure.

Keep private scratch outside the watched application module while preserving the tool's application working directory and targeted final owned output. Then demonstrate the complete real SKGO startup and a subsequent warm equal/repair edit with no self-invalidating loop. Do not discard legitimate raw non-Go events globally to mask application tooling writes. The author began this specific correction during review; its presence alone does not complete the outstanding running proof.
