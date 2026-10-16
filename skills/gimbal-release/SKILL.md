---
name: gimbal-release
description: >
  Develop and deliver changes to Gimbal's CLI/server and macOS app. Use for
  application implementation, release recovery, and published installation.
  For authoring or operating workflows, use the other Gimbal sub-skills.
---

# Develop and release Gimbal

Follow the repository's `AGENTS.md` and `docs/definition-of-done.md` for scope,
review and completion. The `Justfile` and `.github/workflows/` are authoritative
for executable checks and publication. This skill explains which path to use.

## Fast feedback

Use `just dev` for the edit loop. Run affected tests and observe the requested
behavior before proposing a merge. Commit hooks run staged-file checks and a
small, uncached unit-test pack; their target is one second and budget is three.
An overrun prints a visible warning through Lefthook to the agent making the
commit, with instructions to fix it, and records unfinished checks. It permits
the commit. Actual correctness failures still block. `just warm-hook`
prepares compiler caches outside the commit path. Generators, installation,
repository-wide analysis and browser builds stay outside hooks.

Source commit checks require Node >=24.13.0 to run the TypeScript coordinator
directly. The Bash launcher keeps documentation-only mandatory index checks
independent of Go and Node. `just check-hooks` exercises both paths; the launcher
rejects unsupported Node versions for source changes rather than skipping checks.

PR CI selects checks from changed files: instruction/link checks for docs,
script/hook checks for automation, and affected unit tests for application
changes. Main has one complete application gate before distribution. Do not
repeat an unchanged check merely because a commit was made, a PR merged, or a
tag appeared. Existing evidence must name its source and cover the claim; a new
failure or changed input calls for the relevant check again.

For native UI changes, observe the actual app and macOS system menubar using a
disposable catalog, never `~/.gimbal/desktop`. Upload requested screenshots with
`gimbal upload-artifact`. A build or HTTP response alone does not prove pixels.
Read [development.md](references/development.md) for source builds and app
lifecycle; read [model-catalog.md](references/model-catalog.md) only for catalog
refreshes.

## Automated delivery

Use Conventional Commit titles for commits and PRs; squash merges preserve the
PR title and body. Release preparation reads all first-parent commits since the
previous tag: `feat:` selects a minor version, `fix:` or `perf:` a patch, and
other types do not release by themselves. The largest bump wins. Gimbal is
pre-release and has no users; major increments are forbidden. The commit-msg
hook, PR check, and release preparation reject `!`, `BREAKING CHANGE:`, and
`BREAKING-CHANGE:` declarations. Use `feat:` or `fix:` for pre-release API changes.

Merge within the user's authorization, then follow **Automatic release** from
main to its **Release** run. After quick preflight, main reserves a candidate
tag. Release prepares generated source/frontend once, then builds platforms in
parallel. The Linux amd64 packaging job fully qualifies its versioned candidate;
publication waits for that gate and all platform checks, signing and notarization.
Documentation-only changes do not release an app.
Verify the published version and channel; a merged PR or queued run is not
published delivery. Reuse the pipeline's exact-source checks rather than adding
another local build after merge or tag creation.

The release workflows report their own elapsed time in their logs and job
summaries, including queueing. Automatic release covers main through the tagged
publication wait; Release covers the tagged build through feed/Homebrew updates.
Five minutes is the goal and ten minutes the maximum. The workflow emits a
warning when either is exceeded; timing never blocks publication. Inspect that
run for the slow stage. There is no separate timing workflow or issue scanner.

For failed publication, rerun the failed tagged **Release** job, or dispatch
**Automatic release** on main to reuse its existing tag. Inspect the failed stage
before retrying; avoid repeating successful application checks. Read
[distribution.md](references/distribution.md) for credentials, rules,
manual minor tags, signing recovery, and plugin compatibility. Routine delivery
needs no new credential or permission changes.

## Requested local installation

Use the public Homebrew casks (`gimbal` and `gimbal-app`); inspect the installed
version and actual executable/app path. A bare `go install` omits the built web
console. Refresh affected published skills with their supporting references and
compare them with maintained source. Local installation and runtime proof are
required when requested; they are not extra steps for every merge.

Report the requested behavior seen, source revision, relevant checks, publication
status, and any remaining proof gap. Distinguish source proof, distributed
artifact proof, and installed UI proof. Keep signing/credential details in the
distribution reference rather than copying them into the normal procedure.
