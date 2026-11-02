# Fixed candidate completion: implementation review, round 01

## Target

The implementation on `codex/fixed-release-candidate` at `ea49db7d`, against `ephemeral/plans/fixed-candidate-foundation.md` and the repository `AGENTS.md`. The working tree is clean.

Ignored narrowing: none. Do not dispatch, tag, touch shared machines or publish, and edit only this file. Those are valid operating constraints.

## Evidence inspected

- `git status`, `git log` and the diff against `17deb5ba`:
  - Commit `ea49db7d` adds only `ephemeral/` files (plan, spec reviews, worklog). There is no source change since the reviewed implementation head.
  - The implementation under review is therefore the same as PR commit `c47d900c`, which round-01 review `fixed-release-candidate-round-01.md` reviewed.
- Read in full:
  - `internal/automation/release-assets/candidate.go`
  - `candidate_workflow_test.go`
  - the `release-assets/main.go` dispatch diff
  - the whole `release.yml`
- Read the test function list in `candidate_test.go`.
- Read the surrounding producers that decide the real input layout:
  - the macos-release upload paths
  - `release-cli.sh` (`goreleaser --clean`)
  - `release-macos.sh` and `generate-appcast.sh` outputs
  - the unchanged publish upload globs
  - the Homebrew checksum consumer

**Validation limit:** I tried to run `go test`, `go vet` and `actionlint`, but the permission layer refused those commands. So I did not run any check myself. Test and lint results below are not mine. The only passes on record are the earlier reviewer's: focused tests, vet, lint and actionlint, plus a local demonstration against published v0.20.2 files, recorded in `fixed-release-candidate-round-01.md` and the exchange document.

## Findings

I read the code against each DoD item and found no material defect.

- **C1/C2, the file contract:** `candidate.go` does what the plan describes.
  - `candidateTree` rejects any entry that is not one of the expected regular files, so extras, symlinks and unexpected directories fail.
  - `candidateHash` rejects unsafe paths, nonregular files and empty files, and uses `os.Root`.
  - Assembly refuses to overwrite its output, copies and hashes in one pass, and cleans up only the new output on failure.
  - `verifyCandidate` requires the exact identity (struct equality), schema 1, the exact file set with no duplicates, no unknown JSON fields and no trailing data.
  - It rechecks every hash, compares `checksums.txt` with the regenerated five-archive text, and re-validates the appcast: the single item, version, repository and tag URL, ZIP length, and a present signature.
  - The checksum format matches the old publisher's `sha256sum ./*.tar.gz macos/*.zip macos/*.dmg` ordering and `./` prefix.
- **Input layout in the real workflow:**
  - `goreleaser --clean` empties `dist`, so `release-inputs.tar.gz` does not leak into the `cli-*` uploads.
  - The `macos-release` upload's two path roots keep the `dist/` relative layout, so `macos/` and the darwin archive land where `candidateTree` expects.
  - Both match the exact allowed set.
- **C3, workflow:**
  - The `publish` default is unchanged.
  - `candidate` has no `if:` condition and needs `linux-cli` and `macos-release`.
  - `publish` keeps `inputs.publish` and needs `qualify` and `candidate`.
  - The assemble step checks `HEAD` and the tag commit against `github.sha`.
  - The upload name carries the run and attempt, and the ID, digest, URL and assembly attempt are exposed as outputs.
  - The publisher uses `artifact-ids` with `merge-multiple`, verifies with the saved attempt, and uploads the unchanged glob. `candidate.json` is not published, since the globs omit it.
  - The static workflow test pins each of these.
- **Budgets:** `candidate.go` has 283 nonblank lines against 300. `release.yml` has 419 nonblank lines against 440. No new exported name, workflow or proof program was added.
- **Repository rules:** no run output or proof code is committed. Conventional commit prefixes are used. Tests sit beside the code. There are no wrappers.
- **Not yet proved (expected):**
  - C4 and C5 depend on the root-owned tagged dispatch and have not happened. No actual Actions upload, immutable ID, digest or `qualify` record exists yet, so those claims are unmet today and must not be reported as met.
  - The publisher's ID-based download, verify and retry stay unexercised, as the plan states.

### Nitpicks

1. **`artifact-digest` is unproven.** The `candidate` job reads `steps.upload.outputs.artifact-digest` from `actions/upload-artifact@v4`, a floating tag, and fails the whole job with `test -n "$ARTIFACT_DIGEST"` if it is empty. I could not confirm offline that the pinned action version emits this output.
   - A missing output would fail after the artifact is already uploaded, so the run is red and `publish` is blocked even though the bundle exists.
   - The planned C4 run will show this. If it fails, the cause is the missing output, not the assembly.
2. **The tag check is split across steps.** `git rev-parse HEAD` and the tag-commit check run only in the assemble step. That is sufficient, but the plan's "Source, tag and producing run/attempt must agree" is enforced there and again by `verify-candidate`. The publisher does not repeat the tag-commit check. This is fine today because the verify step binds the identity to `github.sha` and the tag string, and the tag commit equals `github.sha` by the dispatch gate.

## Outcome

only nitpicks remain
