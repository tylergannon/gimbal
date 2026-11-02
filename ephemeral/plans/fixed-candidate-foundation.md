# Fixed candidate foundation — proposed task and definition of done

Status: approved for execution by Tyler on November 1: "If you don't think it needs to go by me and if you recommend that I approve it as it is, then just go ahead and execute." The owner recommends this bounded task. Implementation, coordinated temporary-tag creation/cleanup and build-only dispatch are authorized. Merge and publication remain excluded.

## Goal and existing work

Finish existing draft PR #664 on `codex/fixed-release-candidate`, starting from
`17deb5bab1122a29d0596c4bca2f0ccae6360afd`. Deliver an actually uploaded,
downloadable build-only candidate whose identity and publication bytes can be
verified. Preserve the existing normal publication behavior by source changes
and focused checks; a build-only run does not prove live publication. This foundation
does not qualify the product for release.

The prior task authorized local assembly and source checks only. Its observed
published-file demonstration and independent review remain useful evidence;
neither exercised the new Actions upload. This task proposes closing that gap.

## Cells and contracts

1. **Candidate file assembly and verification.** Existing
   `internal/automation/release-assets/candidate.go` owns the fixed file format
   and rejection behavior. It accepts explicit repository, full source SHA,
   v0.x.y tag, positive producing run ID and attempt. Assembly copies the three
   CLI archives, signed Mac ZIP/DMG and appcast unchanged, creates the existing
   five-archive checksum format, and writes `candidate.json` with schema 1,
   identity and hashes of those seven publication files. Verification requires
   the exact expected identity and file set; changed, missing, extra, unsafe,
   duplicate or nonregular entries and inconsistent checksums/feed fail. The
   existing feed signature must be present; signature trust is not claimed.
   Budget: 300 nonblank production lines in candidate.go (currently 283 nonblank).
2. **Actions transport and consumption.** Existing `.github/workflows/release.yml`
   owns real platform packaging, upload, identity outputs and existing publisher
   wiring. A successful build with `publish=false` produces the bundle after
   platform packaging, exposes its immutable artifact ID, archive digest, URL,
   name and assembly attempt outside the bundle, and skips publication. Source,
   tag and producing run/attempt must agree. Candidate existence alone does not
   establish source qualification. The existing publisher keeps its gates and
   default, downloads the explicit ID, verifies before publishing and retains
   the producing assembly attempt on publisher retry. Budget: 440 nonblank
   workflow lines. No new workflow or exported API is proposed.

These cells change for different reasons: file-format validity versus Actions
packaging and transport. The manifest is their seam. Test code remains beside
the owning implementation; no new proof program is committed.

## Definition of done

- **C1, cell-local, assembly:** real package/appcast inputs assemble successfully;
  copied files remain byte identical and checksums retain the existing format.
- **C2, cell-local, verification:** matching identity/content passes; changed
  content and wrong source/tag/run/attempt fail with nonzero status. Existing
  focused tests cover the remaining documented rejection contract.
- **C3, cell-local, workflow:** source/platform gates, publish default and
  explicit-ID consumer with saved assembly attempt remain intact. Source checks
  and inspection cover this contract; no claim of a live publisher retry is made.
- **C4, integration, both cells and GitHub Actions:** an approved exact revision
  runs the actual tagged Release workflow with `publish=false`. All required
  build/source jobs succeed, candidate upload succeeds, publication is skipped,
  and no release or Homebrew update results. Retain the public run and immutable
  artifact links and observed identities in the PR report.
  Observe the publisher job as skipped, absence of a GitHub Release for the test
  tag, and unchanged Homebrew tap revision across the bounded run. If unrelated
  tap activity occurs, identify its ownership before attributing it to this run.
- **C5, integration, both cells and GitHub Actions:** download that exact artifact
  by immutable ID, independently match the archive SHA-256 to upload/API identity,
  extract it, verify the manifest against independently obtained source/tag/run/
  assembly attempt, and compare payload bytes to the same run's platform outputs.
  Independently repeat content/identity rejection on a disposable local copy.
  Obtain platform outputs independently from that run's `cli-*` and
  `macos-release` artifacts by immutable ID. Record the `qualify` job ID/link and
  successful conclusion for the same run/attempt; do not infer it from upload.

Completion report names the proved source SHA and run/attempt, artifact ID/digest,
all satisfied/unmet claims and expiry. Report observations in chat/PR; do not
commit run output, dumps, screenshots or executable proof material.
Complete download and independent verification before artifact expiry. Explicitly
report the actual publisher job's ID-based download/verification and retry as
unexercised: C5 demonstrates external exact-ID consumption, not those skipped steps.

## Bounded execution plan after approval

1. C, the Tyler-requested Terra 6.1 implementation agent supervised in this chat,
   inspects the approved definition and existing implementation in this
   worktree, making only necessary repairs with focused affected checks.
2. D, the Tyler-requested independent Sonnet validation agent, reviews the task and later implementation/proof, retaining
   its session and a new review artifact each round. Resolve material findings
   with the same reviewer under consensus; escalate disputed requirements to Tyler.
3. Before consuming a shared release runner or version tag, identify ownership
   and coordinate the selected revision/tag and a bounded build window. The
   current native job only runs when the dispatch ref is that exact version tag.
   `prepare-release` considers all stable-format tags, including unpublished
   ones. Agree a specific version tag, no competing automatic version preparation
   during its lifetime, and cleanup ownership with Tyler and parent chat A
   (`01a1981c-d915-7300-a4eb-0da2f8955827`). This chat B owns deletion of its
   temporary tag. Check
   active automatic-release runs and fresh tags before creating anything. If
   that window cannot be coordinated, stop and resolve the execution route rather
   than silently leaving a version reservation behind. This task owns only its
   newly created tag; approval must cover deleting that tag after verification
   (or after failure cleanup), without touching any pre-existing tag. Record the
   run/SHA/tag association before cleanup and verify the tag is absent afterward.
4. Dispatch the approved tag with `publish=false`, inspect actual results, then
   perform independent ID/download/hash/byte checks. A failed run requires cause
   inspection before a bounded retry; it is not completion.
   A retry means the unchanged approved revision. Necessary source repairs are
   authorized within this task; record the new commit and coordinate a fresh
   tag/revision before dispatch. Ask Tyler only if a repair changes scope or a
   consequential decision cannot be settled from this plan. Never repoint or
   recreate the deleted test tag to make a changed revision look like a retry.
5. Update the existing PR with observed claims, checks and boundaries. Readiness
   for merge is the handback; do not merge in this lane. Merge can activate the
   automatic release path, depending on Conventional Commit version selection
   and other pending changes, and requires a separate publication-aware decision.

## Exclusions

Native GUI/Locked Use acceptance, qualification installations and services,
holdout procedures, scheduler and six-hour policy, candidate selection service,
promotion authority, separate promotion workflow and final promotion are outside
this lane. No weakening of publication gates, new provider, credential change,
shared-service change, speculative framework or orchestrator timing/cancellation
accounting. Do not recreate the existing PR or modify another task's branch.

## Decisions required before dependent work

- Retention settled: retain the existing two days and complete verification
  within that window. Tyler delegated this routine choice on November 1.
- Approved task/DoD and consequential experiment: an unpublished
  version tag at the approved revision and one build-only signed/notarized Release
  dispatch, with cause-based repairs/retries. Exact tag requires a fresh collision
  check and coordination, an exclusive version-preparation window and owned tag
  cleanup as above; do not repoint an existing public tag. The owner selects the
  exact tag after a fresh collision check and coordination, without an additional
  Tyler approval for routine implementation choices.
Handback is the existing PR with verified foundation claims and explicit
operational limits. Merge and publication remain excluded.
