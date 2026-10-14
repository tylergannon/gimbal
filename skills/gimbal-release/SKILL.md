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
An overrun prints a warning and records unfinished checks rather than blocking
shipping. Actual correctness failures still need attention. `just warm-hook`
prepares compiler caches outside the commit path. Generators, installation,
repository-wide analysis and browser builds stay outside hooks.

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

Merge within the user's authorization, then follow **Automatic release** from
main to its **Release** run. After quick preflight, main reserves a candidate
tag. Release prepares generated source/frontend once, then builds platforms in
parallel. The Linux amd64 packaging job fully qualifies its versioned candidate;
publication waits for that gate and all platform checks, signing and notarization.
Documentation-only changes do not release an app.
Verify the published version and channel; a merged PR or queued run is not
published delivery. Reuse the pipeline's exact-source checks rather than adding
another local build after merge or tag creation.

Timing policy lives in `scripts/release-performance.json`: five minutes is the
main-to-publication goal, ten minutes the maximum. Queue/retry time counts;
active stage timings are also reported without adding parallel jobs together.
**Release performance** observes runs and opens issues for overruns. It does not
fail application tests or stop publication. Use its reports to improve the slow
stage; do not relax budgets just to make a report green.

For failed publication, rerun the failed tagged **Release** job, or dispatch
**Automatic release** on main to reuse its existing tag. Inspect the failed stage
before retrying; avoid repeating successful application checks. Read
[distribution.md](references/distribution.md) for credentials, rules, manual
major/minor tags, signing recovery, and plugin compatibility. Routine delivery
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
