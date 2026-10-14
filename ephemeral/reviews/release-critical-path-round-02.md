# Adversarial review: release critical path, round 02

Date: 2026-10-14 (local). Branch `codex/release-critical-path` at 9bd7c2dc, clean tree, four commits on top of main (f6234e33). PR #540 open; its "Check release source" run was in progress at review time.

## Target

Same authoritative sources as round 01: `AGENTS.md`, `skills/gimbal/SKILL.md` → `skills/gimbal-release/`, and the objective and decisions in `ephemeral/worklog/release-critical-path.md` (main-to-feed/Homebrew publication under ten minutes aiming for five, with validation and Mac usability preserved; tag reserved before the full suite; one shared source/frontend preparation; concurrent platform packaging; full Ubuntu qualification of the versioned amd64 archive in its build job; publish gated on qualification and every platform; prepared inputs tied to the exact revision including deletions; Mac work bounded to four workers; app bundles the versioned distributed CLI; `skgo native build` prepare-only without frontend/Debug; completion requires a published release with measured timings, all required checks, and observed Mac resource behavior).

Caller constraints honored: read-only except this artifact. No scope narrowing was requested, so none was ignored.

## Evidence inspected

- Full `git diff main...HEAD` (18 files) and the current text of `.github/workflows/release.yml`, `.github/workflows/auto-release.yml`, `Justfile`, `scripts/release-cli.sh`, `scripts/release-macos.sh`, `scripts/release-assets.py`, `scripts/release_assets_test.py`, `scripts/release_timing.py`, `scripts/release_timing_test.py`, `scripts/sign-notarize-macos-cli.sh`, `scripts/public-xcodebuild/xcodebuild`, `cmd/gimbal/release_plugin_archive_test.go`, `cmd/gimbal/main.go` (flags), `.goreleaser.yaml`, `.gitignore`, `go.mod`/`go.sum`, `web/package.json`, both worklogs, the gimbal-release skill and `references/distribution.md`.
- Round 01 findings, each re-verified against the current tree (see below).
- `go tool skgo native build --help` against the pinned v0.33.0: `-jobs`, `-prepare-only`, `-reuse-web-build` present. Diffed skgo v0.32.2 → v0.33.0 in the module cache: `internal/adapter` is byte-identical (so the `ReadManifest` adapter fingerprint check still matches `@skgo/sveltekit-adapter` 0.32.1); `internal/nativebuild/build.go` returns before any `xcodebuild` call under `PrepareOnly`.
- Downloaded the published v0.13.13 archives. `gimbal_0.13.13_linux_amd64.tar.gz` (GoReleaser on Linux) stores bare `gimbal`, `LICENSE`, `README.md`; the signed `darwin_arm64` archive stores `./gimbal`. Extracted `gimbal` by name from both with bsdtar and GNU tar 1.35: all four succeed.
- Ran `GIMBAL_RELEASE_ARCHIVE=<darwin archive> go test ./cmd/gimbal -run '^TestReleaseCLIWeb$'`: PASS in 0.77s, reproducing the worklog's proof.
- Ran `scripts/release_assets_test.py` under `/usr/bin/python3` (3.9.6, the runner LaunchAgent's interpreter): 3 tests OK. Ran the full `scripts` suite under the shell's python: 56 tests OK. `go vet ./cmd/gimbal` clean.
- `gh run list -w release.yml` and `gh release list`: every Release run to date (v0.13.8 … v0.13.13) executed the pre-branch workflow. `gh pr list`: #540 is the only PR for this branch.

## Round 01 status

1. skgo flags: fixed. `go.mod`/`go.sum` pin v0.33.0 with no replace; help output confirms the flags.
2. Linux qualification extraction: fixed. The step now extracts the whole archive into `bin/`, and `TestReleaseCLIWeb`'s `tar ... gimbal` works for both member spellings on both tar implementations (verified above).
3. Python 3.9 `extractall(filter=)`: fixed. Explicit member type/path validation; passes on 3.9.6.
4. Documentation: fixed. SKILL.md, `distribution.md` and the `ensure_issue` body describe the new pipeline.
5. Bundled-CLI precheck: fixed (`release-macos.sh:193-196`).

## Findings

### 1. Issue: the new tagged pipeline has never run; the first execution will be a production release behind a reserved tag

The worklog's completion criterion is "a published release with measured timings, all required checks, and observed Mac resource behavior". No run of the new `release.yml` exists: all six Release runs listed by `gh run list` used the previous workflow (v0.13.13 at 14:22 was published from main before this branch). Nothing in the worklog or PR records even a `workflow_dispatch` of the new workflow against an existing tag (the workflow accepts `inputs.tag`, so a dry run against `v0.13.13` from this branch's ref was available without publishing: `publish` would fail at `gh release create --verify-tag`/`upload --clobber` only if it reached that step, and could have been observed up to the Homebrew gate).

Untested in any real run: `prepare-inputs` → `release-assets.py pack` on Ubuntu with `go generate ./...` output; artifact restore on the arm64 Linux runner; `actions/checkout` + restore + `goreleaser --clean` ordering (restore writes into `dist/`, which `--clean` then wipes, intentionally); the qualification step list (`just check-cli-release`, `lint-go`, `go vet`, `./bin/gimbal lint`, `just test`, `fmt-check`, `check-web`) against a restored tree on a Go toolchain chosen with `check-latest: true` rather than the preparation job's; `skgo native build --prepare-only --reuse-web-build --jobs 4` under the runner's LaunchAgent environment; and `GIMBAL_DERIVED_DATA` under `~/Library/Caches`. Because `auto-release.yml` now reserves the tag and changelog commit before any of this runs, a first-run failure leaves a published tag and metadata commit whose recovery is a manual re-dispatch. This is a proof gap, not a found defect; the implementation reads correctly. Impact: the completion criterion is unmet and the merge is the test.

### 2. Issue: `auto-release.yml` still installs Go, pnpm and Node on every main push though nothing on main uses them any more

With the gate removed, the main-push path of `Check release source` is: scope selection, link check, `setup-go` (`if: release == 'true'`, line 44), the Python automation tests, `pnpm/action-setup` and `setup-node` (`if: release == 'true' && (event != 'pull_request' || …)`, lines 56-60), then the `release` job. The only remaining consumers of those toolchains are PR-only steps (lines 53, 66-87). Every main push therefore pays Go, pnpm and Node setup (including the pnpm store restore) on the critical path before the tag is reserved, for nothing. This directly works against the branch's stated goal. Fix: gate `setup-go` on `github.event_name == 'pull_request' && steps.scope.outputs.go_packages != '[]'` and drop the `github.event_name != 'pull_request'` arm from the pnpm/node conditions.

### 3. Nitpick: `scripts/public-xcodebuild/xcodebuild` is now dead

`release-macos.sh:209` no longer prefixes `PATH` with `scripts/public-xcodebuild`, and under `--prepare-only` skgo never invokes `xcodebuild` (build.go returns at the "Prepared native project" line), so the shim has no caller (`git grep public-xcodebuild` outside `ephemeral/` is empty). `AGENTS.md`: "delete what is replaced".

### 4. Nitpick: the timing test models a job the workflow does not have

`release_timing_test.py:132-139` (`test_qualification_overlaps_platform_work_without_double_counting`) feeds a job named `Qualify release candidate`. The real workflow has no such job: qualification runs inside `CLI (linux_amd64)`. The interval-union measurement is name-agnostic, so the number is right, but the fixture documents a topology that does not exist, and `parallel_platform_critical_path_seconds` (still computed from `platform_attempts` by name prefix `CLI (`) now silently includes the amd64 job's qualification minutes as "platform" time. Rename the fixture job to `CLI (linux_amd64)` with the longer duration, and either rename the platform metric or note in `ensure_issue` that amd64 packaging time includes qualification.

## Notes (not findings)

- `release-assets.py`: pack excludes `ephemeral/` and `docs/`, includes `web/build`, tracked AM changes and untracked files, and records deletions; restore refuses a different SHA before touching the tree, rejects non-file/non-dir members and any member or deletion resolving outside the checkout, and works on 3.9.6. On Python ≥3.14 the default `data` filter accepts these members.
- `actions/checkout` cleans ignored files each run, so stale `web/build/*`, `native/Resources/gimbal`, `native/build/macos` and `dist/` cannot leak between Mac runs; only the opted-in `DerivedData` persists.
- `release-cli.sh` on darwin extracts `./gimbal` from the signed archive (which `sign-notarize-macos-cli.sh` repacks from `.`), so the app bundles the exact notarized CLI; `release-macos.sh` verifies it before skgo/Xcode work.
- `linux_arm64` is still built and uploaded, and `publish` `needs: [linux-cli, macos-release]` waits for the whole matrix, so qualification failure on amd64 blocks publication.

## Outcome

material findings remain
