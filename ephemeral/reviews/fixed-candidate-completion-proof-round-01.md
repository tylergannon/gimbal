# Fixed candidate completion: proof review, round 01

## Target

The actual proof for the approved plan `ephemeral/plans/fixed-candidate-foundation.md`:

- Release run 41378822759, attempt 1, on `tylergannon/gimbal`, dispatched on ref `v0.20.3` with `publish=false`.
- Source `ea49db7d993f6b310968e262dc6f772dcea4c2b1`.
- Candidate artifact 13112408419.
- The downloaded originals under `/tmp/gimbal-fixed-candidate-41378822759`, located through `context.md`.

Ignored narrowing: none. The no-tag, no-dispatch, no-merge and no-publication rule is an operating constraint. I changed no original artifact. Negative checks ran on a disposable copy in `/tmp/rev-rDT3`.

## Evidence and independent observations

I derived every observation below from GitHub API reads and local checks. I did not take any of it from the narrative in `context.md`.

**Run identity and jobs** (API, attempt 1)
- The run is a `workflow_dispatch` on branch `v0.20.3`, `head_sha` `ea49db7d…`, status completed, conclusion success.
- Jobs: Prepare inputs, CLI linux_arm64, CLI linux_amd64, Qualify release source (125065239670), the macOS release job, and Assemble fixed release candidate (125066562272) all succeeded. Publish (125066715393) was **skipped** with zero steps run.
- The `candidate` job's steps (checkout, both downloads, assemble, upload, record identity) all succeeded.
- Its log shows each platform artifact downloaded by ID with an expected digest that matched the digest computed on download.

**Tag and source**
- The remote `v0.20.3` is an annotated tag, and it peels to `ea49db7d993f6b310968e262dc6f772dcea4c2b1`, the run's source SHA and the current branch head.
- That tag still exists as of this review. The plan assigns its cleanup to root.

**Artifact identity**
- API artifact `release-candidate-41378822759-1` has ID 13112408419, digest `sha256:c50eff9b…33faa`, `expired: false` and `expires_at` 2026-11-04T02:45:25Z. Its `head_sha` is the source SHA.
- The local `candidate.zip` hashes to the same `c50eff9b14cc…`.
- The four sibling artifacts (`cli-linux_amd64` 13112935186, `cli-linux_arm64` 13112134474, `macos-release` 13112169829, `release-inputs` 13111454857) have digests that match the local `linux-amd64.zip`, `linux-arm64.zip` and `macos.zip`, and, for the first three, the digests logged in the job.

**Manifest and content**
- `candidate.json` is schema 1, repository `tylergannon/gimbal`, tag `v0.20.3`, run 41378822759, attempt 1, with the seven expected files.
- I rebuilt `release-assets` from this worktree into my own temporary path. `verify-candidate` with the exact expected identity exited 0.
- `shasum -a 256 -c checksums.txt` returned OK for all five archives. The `checksums.txt` matches the old publisher's format: a `./` prefix for root files and `macos/` for app files.
- `cmp` matched all six copied payloads (darwin, linux amd64 and arm64 archives, ZIP, DMG, appcast) byte for byte against the independently downloaded platform artifacts.
- `appcast.xml` has a single item with version 0.20.3 and a `v0.20.3` enclosure URL. The enclosure length is 26686962 and an `edSignature` is present. Signature trust is not claimed.

**Rejection behavior** (disposable copy, explicit flags)
- Good copy: exit 0.
- One byte appended to the amd64 archive: exit 1, "candidate SHA256 mismatch".
- Restored: exit 0.
- Extra file: exit 1, "unexpected candidate entry".
- Missing `appcast.xml`: exit 1.
- Fake line appended to `checksums.txt`: exit 1.
- Wrong run attempt (2), wrong run ID, wrong source SHA and wrong tag: each exited 1. The tag case failed on the file set, not on a tag-specific check, so it is a weaker test than the others.

**Publication did not happen**
- `releases/tags/v0.20.3` returns 404. The latest releases are still `v0.20.2` and `v0.19.4`.
- The Homebrew tap `main` is still `72844b11b2b9f4dfa8cc1ed9b4fc6252e40d361c`, matching the recorded baseline before and after the run.

## Findings

No material findings. The proof supports C4 and C5 as written.

- **C4:**
  - The tagged `publish=false` run succeeded.
  - The candidate upload succeeded.
  - The publisher was skipped.
  - No release exists for the test tag.
  - The tap is unchanged.
- **C5:**
  - The exact ID was downloaded and the archive digest matched the API digest.
  - The manifest verified against independently obtained identity.
  - The payloads matched the same run's platform outputs by ID.
  - Rejection was repeated on a copy.
  - The `qualify` job (125065239670) succeeded on the same run and attempt.

### Nitpicks

1. **Verified against an unpinned base.** The last-released tag check is the review's own, but the plan's "record the run/SHA/tag association before cleanup" needs root to write the `v0.20.3` to `ea49db7d` association into the report. The tag exists now and the plan requires it to be deleted afterwards and verified absent. That final step was not yet done at review time.
2. **Wrong-tag rejection is not a clean identity test.** A wrong tag fails on the expected file names before the manifest comparison. That is still a rejection, but it does not exercise the manifest tag field alone.
3. **Reporting must keep the stated limits.** The report must say the actual publisher job's ID-based download, verification and retry are unexercised, and must state the expiry (2026-11-04T02:45:25Z for the candidate; the platform artifacts expire a few minutes earlier) so that C5 is repeated by nobody after the window. Native acceptance, signature trust, merge and publication remain outside this claim.

## Outcome

only nitpicks remain
