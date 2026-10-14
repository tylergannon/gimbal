# Adversarial review: release critical path, round 01

Date: 2026-10-14 (local). Branch `codex/release-critical-path`, uncommitted working tree on top of f6234e33.

## Target

Objective and constraints from `ephemeral/worklog/release-critical-path.md`: main-to-feed/Homebrew publication under ten minutes (aim five), preserving validation and Mac usability. Decisions: tag reserved before the full suite; one shared source/frontend preparation; concurrent platform packaging; full Ubuntu qualification of the versioned amd64 archive in its build job; publish depends on qualification and every platform; prepared inputs tied to the exact revision including deletions; Mac work bounded to four workers; app bundles the versioned distributed CLI; `skgo native build` prepare-only without frontend/Debug.

Caller constraints honored: read-only except this artifact. No scope narrowing was requested, so none was ignored.

## Evidence inspected

- `git diff` of `.github/workflows/auto-release.yml`, `.github/workflows/release.yml`, `Justfile`, `scripts/prepare-release.py`, `scripts/release-cli.sh`, `scripts/release-macos.sh`, `scripts/release_timing.py`, `scripts/release_timing_test.py`; new `scripts/release-assets.py`, `scripts/release_assets_test.py`.
- Full current `release.yml`, `auto-release.yml`, `release-cli.sh`, `release-macos.sh`, `release_timing.py`, `prepare-release.py`, `dispatch-release.py`, `.goreleaser.yaml`, `.gitignore`, `go.mod` (skgo pin), `web/package.json`, `skills/gimbal-release/references/distribution.md`.
- Ran `python3 -B -m unittest discover -s scripts -p '*_test.py'`: 56 tests OK.
- Ran `go tool skgo native build --help` against the pinned module.
- Downloaded the published `gimbal_0.13.12_darwin_arm64.tar.gz` and listed its members; attempted extraction with GNU tar 1.35.
- Inspected this machine, which is the `amritanandamayi` release runner: its LaunchAgent PATH, the `python3` it resolves, and that interpreter's `tarfile.extractall` signature.

## Findings

### 1. Critical: the pinned skgo has no `--prepare-only`, `--reuse-web-build`, or `--jobs` flags

`scripts/release-macos.sh:205` runs `go tool skgo native build --platform macos --prepare-only --reuse-web-build --jobs N`. `go.mod:34` still pins `github.com/tylergannon/skgo v0.32.2` and `go.mod`/`go.sum` are not in the diff. Against that pin:

```
$ go tool skgo native build --help
Usage of native build:
  -platform string
  -preset string
  -root string
```

The worklog says the flags were added upstream and exercised locally, but that change was never pulled into this tree. Every tagged Mac job will die at the prepare step with `flag provided but not defined: -prepare-only`, after signing setup and the CLI build/notarization have already spent their time. Fix: bump the skgo require (and `@skgo/sveltekit-adapter` if the release pairs them) to a tag that has the flags, and commit `go.mod`/`go.sum`/`web/pnpm-lock.yaml`.

### 2. Critical: Linux qualification extracts a member name that is not in the archive

`.github/workflows/release.yml` step "Prepare full qualification prerequisites" runs `tar -xzf dist/gimbal_*_linux_amd64.tar.gz -C bin gimbal`. GoReleaser stores members with a `./` prefix; the real v0.13.12 archive lists `./`, `./gimbal`, `./LICENSE`, `./README.md`. GNU tar (Ubuntu's default, 1.35) matches anchored names exactly:

```
$ gtar -xzf gimbal_0.13.12_darwin_arm64.tar.gz -C out gimbal
gtar: gimbal: Not found in archive
gtar: Exiting with failure status due to previous errors
```

The step exits non-zero, `CLI (linux_amd64)` fails, and `publish` (which `needs` it) never runs: no release can be published by this workflow. `scripts/release-cli.sh:56` uses `./gimbal` with bsdtar on the Mac, which is why that form works there. Use `./gimbal` (or `--strip-components=1`) on Linux as well.

### 3. Critical: the Mac runner's `python3` cannot run `release-assets.py restore`

`scripts/release-assets.py:47` calls `source.extractall(filter='data')`. The `filter` keyword exists only in Python 3.8.17+/3.9.17+/3.10.12+/3.11.4+/3.12+. The release runner is this machine; its LaunchAgent (`~/Library/LaunchAgents/actions.runner.tylergannon-gimbal.amritanandamayi.plist`) exports `PATH=/opt/homebrew/bin:~/.local/bin:~/.local/share/mise/shims:/usr/bin:...`. Under that PATH:

```
$ which python3 && python3 --version
/usr/bin/python3
Python 3.9.6
$ python3 -c "import tarfile,inspect; print('filter' in inspect.signature(tarfile.TarFile.extractall).parameters)"
False
```

There is no Homebrew or mise python3 on that PATH, and the restore step runs before `jdx/mise-action` anyway. The step raises `TypeError: extractall() got an unexpected keyword argument 'filter'` and the macOS job fails before any build. The 3.11.6 at `/usr/local/bin` is not on the runner's PATH. Either drop `filter=` (the archive is produced by this same workflow, so the hardening buys little) with a guard for older interpreters, or put a modern Python on the runner PATH and pin it. Also note `release_assets_test.py` passes here only because the shell's `python3` is 3.11.6.

### 4. Issue: release documentation no longer describes the pipeline

`AGENTS.md` names `skills/gimbal/SKILL.md` (routing to `skills/gimbal-release/`) as the authority on building and releasing. `skills/gimbal-release/references/distribution.md:98-100` still says non-documentation main changes "run one complete build, analysis, test, formatting and Svelte gate before release"; the diff removes that gate from `auto-release.yml` and moves qualification after the tag into `CLI (linux_amd64)`. Line 178 still says `scripts/release-cli.sh` "runs the full frontend build", which is now skipped under `RELEASE_PREPARED_INPUTS=1`, and nothing documents the shared `release-inputs` artifact, `just build-assets`, `RELEASE_PREPARED_INPUTS`, `GIMBAL_DERIVED_DATA`, or `GIMBAL_BUILD_JOBS`. The failure-issue body in `scripts/release_timing.py` (`ensure_issue`) still says "Active critical-path time counts parallel platform maximums", which the new interval-union computation no longer does. Update these alongside the behavior change; the recovery procedure in the skill now depends on them.

### 5. Nitpick: prepared-input mode trusts the bundled CLI blindly until after the Xcode build

With `RELEASE_PREPARED_INPUTS=1`, `scripts/release-macos.sh:177` skips building the bundled CLI and also skips the `--version` check that followed it. If `native/Resources/gimbal` is missing or stale (for example a manual run that did not execute `release-cli.sh darwin_arm64` first), the error surfaces only after the full Release build, at the `app is missing its bundled Gimbal CLI` check. A `test -x "$BUNDLED_CLI"` plus the existing version grep before the `skgo`/`xcodebuild` block would fail in seconds instead of minutes.

## Notes (not findings)

- Timing change (`release_timing.py` interval union) and its new test are correct for the overlapping-job topology; `platform_attempts` is still used for `parallel_platform_critical_path_seconds`.
- `release-assets.py pack`/`restore` revision check, deleted-file carry-over, and `web/build` inclusion behave as the worklog decided; `goreleaser --clean` removes `dist/release-inputs.tar.gz` before artifact upload, so it does not leak into the GitHub release.
- `pnpm install --frozen-lockfile` in the qualification job runs `prepare: svelte-kit sync`, so `pnpm check`/`pnpm test` have `.svelte-kit` despite the build happening in another job.
- Tagging before qualification is the worklog's explicit decision; on a failed qualification the tag and changelog commit remain and `dispatch-release.py` re-dispatches the same tag on rerun, which is the documented recovery path.

## Outcome

material findings remain
