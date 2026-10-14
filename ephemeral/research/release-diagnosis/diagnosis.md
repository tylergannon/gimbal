# Release diagnosis — 2026-10-14

## Conclusion

The v0.13.23 automatic release and publication completed successfully. Its
published packages, updater feed, and Homebrew entries agree with that release.
The Release performance workflow is currently broken: an invalid timestamp on
an old cancelled job aborts its historical scan. This also prevents the later
hourly hook-timing checks from running. It does not gate publication.

This is a diagnosis of release automation and distribution at the inspected
revision, not a claim that every application feature works. No repair is made
by this investigation. Raw API responses, downloads, and run logs remain in
`/tmp/gimbal-release-audit-*`, outside Git.

## Authoritative request

Tyler asked why release workflows were failing, why Python had entered a Go/TS
project, and how this morning's proof missed immediate failures. He then asked
what is broken right now, and instructed: double-check everything, write the
diagnosis, and obtain Claude adversarial review before presenting conclusions.
The requested outcome is a correct, independently challenged diagnosis.

## Snapshot and workflow status

Inspected source: `8fcff5a80994e72d20e37c5b4ca1adb92bb4c10b`, the commit pointed
to by annotated tag v0.13.23. Its release metadata names source commit
`ac958bf01f0bc37ec563e2ff2a729674e8bec77e`. Inspection took place after publication
on October 14, 2026, approximately 20:10-20:19 UTC (14:10-14:19 Costa Rica);
times below use UTC unless marked Costa Rica (UTC-6).

| Active workflow | Current evidence | Assessment |
| --- | --- | --- |
| Automatic release | [Application-source run 38628422518](https://github.com/tylergannon/gimbal/actions/runs/38628422518) and [metadata run 38628566906](https://github.com/tylergannon/gimbal/actions/runs/38628566906) both completed successfully | Preparation, tagged dispatch/wait, and metadata handling passed on latest source |
| Release | [v0.13.23 run 38628567422](https://github.com/tylergannon/gimbal/actions/runs/38628567422): all six jobs succeeded | Preparation, both Linux targets, Mac signing/notarization, full source qualification, and publication passed |
| Release performance | [Latest completed run 38629348565](https://github.com/tylergannon/gimbal/actions/runs/38629348565) failed with the historical timestamp error | Broken historical scan and dependent hook observation |
| Redirect old documentation URLs | [Run 38628566847](https://github.com/tylergannon/gimbal/actions/runs/38628566847) succeeded; public Pages HTML fetched and points to Gimbal View | Latest deployment and public redirect document work |
| Refresh model catalog | [Latest run 38542742684](https://github.com/tylergannon/gimbal/actions/runs/38542742684), 09:29:39 UTC, succeeded | Last scheduled execution passed; it was not rerun in this investigation |

GitHub reports the Mac runner and both Bob runners online and idle. This is
runner availability evidence, not an extra guarantee about future jobs.

## Publication and package checks

The [public v0.13.23 release](https://github.com/tylergannon/gimbal/releases/tag/v0.13.23)
is neither draft nor prerelease. It contains all seven expected assets: three
CLI archives, app ZIP, app DMG, checksums, and appcast.

I downloaded all seven assets and independently matched every byte count and
SHA-256 digest against GitHub asset metadata. All five package hashes also
matched the published checksums file. This verifies distribution consistency;
it does not make the checksum file an independent signing authority.

Both public Homebrew cask files specify 0.13.23; their SHA-256 values match the
downloaded Darwin CLI archive and app DMG. The latest-download appcast equals
the versioned appcast. Its enclosure URL and length match the downloaded ZIP.
The app and feed both declare version 0.13.23 / build 27 / macOS 27.0. The prior
v0.13.22 feed declares build 26, so the updater build number increases.

I verified the appcast's Ed25519 signature over the actual downloaded ZIP bytes
using CryptoKit and the public key in the shipped app's Info.plist. The signature
is valid. This is stronger than merely observing an edSignature attribute.

On the downloaded, unpacked app: deep strict codesign verification passed;
stapler validation passed; Gatekeeper accepted it as Notarized Developer ID.
The downloaded DMG's internal integrity, signature, and stapler checks passed.
The standalone Mac CLI passed strict codesign verification, including an online
notarization-ticket check, and reports v0.13.23 / commit 8fcff5a8. Direct spctl
assessment rejected the bare executable because it is not an app; that command
does not establish an application-style Gatekeeper pass for the standalone CLI.
The shipped app did receive Gatekeeper acceptance, as stated above.

The app's bundled CLI reports the same version, commit, and build time, and has
the same code-directory hash as the standalone CLI. Their whole-file hashes
differ because the app script signs the nested CLI again with a later timestamp
(`scripts/release-macos.sh`, sign_code). Whole-file equality is not claimed.
I started each CLI with an isolated home and instance directory, no project,
and an ephemeral port. Each returned HTTP 200 with Gimbal HTML. Both probe
servers terminated cleanly. No production catalog was used.

The release logs show the actual archived-plugin and archived-web package
checks passing on all three native targets; the amd64 executable routing gate
also passed. Full Go lint/vet/tests, frontend formatting and checks succeeded.
Frontend units: 135 tests in 28 files. Svelte check: zero errors and two existing
warnings. I inspected the release commands and log results, rather than
inferring execution solely from a green aggregate status.

I did not install via Homebrew, launch the native GUI, execute a Sparkle update,
or rerun the Linux binaries locally. These are not claimed as fresh proof.
Linux archive runtime/plugin evidence comes from the actual release runners.

## Active defect and its impact

The offending record is publish job 115944920729 in the cancelled retry
[run 38590047563](https://github.com/tylergannon/gimbal/actions/runs/38590047563)
for v0.13.14. The GitHub Jobs API reports status completed, conclusion cancelled,
start 15:36:32 UTC, and completion 15:36:31 UTC.

`scripts/release_timing.py:66-78` excludes queued and skipped jobs, but includes
this cancelled job. `seconds()` at lines 54-58 rejects its negative duration.
`measure()` collects all matching release attempts. The scan calls measure at
line 293 without isolating per-record errors; the top-level handler at 311-316
exits 1. Therefore one old record prevents completion of the whole scan. I
reproduced the same ValueError locally by passing this real API job to the
current stages() function. A fresh invocation of the actual scanner with
`--scan --measure` also failed with exactly this timestamp error; --measure
prevents issue creation. No issue-creation function was invoked by either probe.

The first observed timestamp failure was [run 38590853348](https://github.com/tylergannon/gimbal/actions/runs/38590853348)
at 15:36:42 UTC (09:36:42 Costa Rica). The latest completed monitor still reports
the same timestamps. Historical retries remain in the scan; publishing a newer
version cannot remove that record or correct this code path.

Reports are written only after the full scan, so this failure prevents a complete
release-timing report and prevents handling later sources in that scan. Earlier
sources can already have been processed or had issues created; it is not correct
to say no monitoring work ever occurs. An [hourly run 38605329199](https://github.com/tylergannon/gimbal/actions/runs/38605329199)
also failed on an API 504; its subsequent Go/Node setup, warming, and hook
measurement steps were skipped. Workflow structure makes both scan failures
stop these later hook steps. The actual local commit hook is not thereby broken.
Transient API failure is a distinct cause of individual failed runs: the API
client has no retry, and an HTTP 504 also aborts the scan. The persistent
timestamp failure and transient network failures should not be conflated.

The latest 39 monitor runs sampled include 37 failures, one cancellation, and
one success before the timestamp failure began. These are not 37 consecutive
failures: the cancellation interrupts the sequence. Five-minute scans are
configured, but the observed triggers also include workflow events and the
hourly schedule; the sample does not prove execution every five minutes.

`auto-release.yml` and `release.yml` neither need nor wait for Release performance.
The actual v0.13.23 run confirms publication succeeds while the observer fails.
The broken capability is performance observation and advisory issue/report
generation, with repeated failed Actions runs; no current publication blockage
has been found in the inspected release.

The 19 current timing unit tests pass. The reversed-timestamp unit test explicitly
expects rejection; no test demonstrates continuing a real historical scan past
such a cancelled record. Passing these tests did not establish observer liveness.

## Why the morning's validation missed failures

These are resolved release failures, not evidence of current publication failure:

- [#540](https://github.com/tylergannon/gimbal/pull/540), 09:21 Costa Rica, removed
  Mac dependency installation while introducing build reuse. Local native proof
  retained node_modules. The clean Mac needed installed SvelteKit metadata for
  Swift generation and failed. The worklog explicitly acknowledges this mismatch.
  [#541](https://github.com/tylergannon/gimbal/pull/541) restored installation and
  corrected an errcheck failure in the new archive HTTP test.
- [#544](https://github.com/tylergannon/gimbal/pull/544), 10:15 Costa Rica, moved
  Linux jobs to Bob. Its pre-merge proof covered runner image construction,
  registration, mount access, lint, and focused tests, with actual release timing
  explicitly pending. Running the new environment exposed missing Node during
  pnpm installation, then a root-owned Go cache parent preventing private sumdb
  state. [#545](https://github.com/tylergannon/gimbal/pull/545) and
  [#548](https://github.com/tylergannon/gimbal/pull/548) address those failures.
  The latest real releases exercise the repaired paths successfully.
- The observer's timestamp bug existed before the later successful v0.13.21
  publication report. That report proved publication and timings for that release;
  it did not verify that the independent monitor had recovered.

The evidence supports mismatched environment coverage and incomplete system-wide
verification. It does not support claiming that earlier demonstrated publication
successes were fabricated, or that everything became broken after the last proof.

## Python provenance and correction to the chat

My earlier statement that Python entered yesterday was incomplete. Inline Python
in `scripts/generate-appcast.sh` was added with [#485](https://github.com/tylergannon/gimbal/pull/485)
on October 12. Standalone release preparation/dispatch scripts came with
[#514](https://github.com/tylergannon/gimbal/pull/514) October 13. Hook automation
expanded in #520; scope selection and timing observation in #530; artifact
transfer in #540. These are implementation choices; the inspected changes show
no requirement to use Python. Python's presence itself is not the timestamp bug:
unhandled historical-data failure is the behavioral defect in any language.

My prior answer correctly separated publication from monitoring, but understated
the monitor's impact by omitting dependent hourly hook checks. This diagnosis
corrects that omission and states the exact limits of fresh runtime proof.

## Independent review

Claude Code / Sonnet 5.5 independently reviewed the diagnosis in session
`7f3f231b-300d-4fec-b7ae-ea52f9441bce`. Its
[round-01 review](../../reviews/202610141415-release-diagnosis-round-01.md)
reported only nitpicks, with no material errors in the central diagnosis.
It verified source, live API/log failures, hourly skipped steps, tag identity,
publication statuses, and historical merge times. It explicitly did not
independently repeat my downloaded-artifact integrity/signature checks.

Adjudication: accepted the request to distinguish transient API failures and
quantify failed runs. Corrected its wording that the 37 failures were consecutive
or observed at a five-minute cadence. Did not adopt its assertion that existing
timing issues cannot be updated or closed due to this bug: ensure_issue only
looks up and creates issues, and never implemented updating or closing them.
The broken function is completion of scans and subsequent report/alert work.
A follow-up review of the revised diagnosis and artifact claims is pending.
