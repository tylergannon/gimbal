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
During the investigation Tyler added that the remediation plan must replace
Python, using existing Go/TypeScript/shell rather than another language. He
delegated the language choice. This document selects Go for structured release
automation and skill helpers, TypeScript for the timed hook, and existing shell
for command orchestration.

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

Latest delivery timing can be established from the successful runs even though
the observer is broken: the application-source Automatic release was created at
20:00:33 UTC; the successful Homebrew update step completed at 20:06:29 UTC.
The feed was published earlier, at 20:06:26 UTC. Main-workflow-to-feed/tap
delivery therefore took 356 seconds (5m56s): within the ten-minute maximum,
above the five-minute goal. This is a missed goal, not a publication failure.

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

The same Claude session performed a
[round-02 review](../../reviews/202610141420-release-diagnosis-round-02.md),
again finding no material issues. It independently matched all downloaded
assets to live digests and checksums, fetched the live feed and Homebrew casks,
repeated app/DMG signature, stapler, Gatekeeper and integrity checks, and
verified the updater signature using OpenSSL Ed25519 rather than my CryptoKit
check. It did not repeat the CLI startup probes or Linux runtime checks.
Its accepted nitpick was to replace the pending-review text with this account.
Asset digests and Homebrew are distinct published records that agree with the
bytes; this is consistency evidence, not independent provenance of source.

## Remediation plan, including removal of Python

This is planned work, not an implemented repair. Use Go's standard library for
structured automation and package-local Go tests, reusing the existing yaml.v3
dependency for skill helpers that need YAML. Use directly executable TypeScript
and Node standard-library APIs for the hook; keep the existing shell
build/signing/orchestration scripts. Do not introduce a new language, runtime,
general workflow framework, compatibility shim, or Python fallback.

Release/CI Go replacement programs live in package-main directories under
`internal/automation/`, with tests beside the code. They do not extend the public
gimbal API. Each command retains a concrete existing purpose; this is not a
new workflow abstraction. Restore the observer first by replacing its Python
entry point with Go and fixing the diagnosed scan isolation and API failure
paths, plus separating hourly hook measurement from scan success. Verify that
live recovery before continuing the remaining migration. Keep behavior-preserving
ports and additional fixes in separate reviewable commits where practical;
do not wait for every skill helper to be ported before restoring monitoring.

The desired state is:

- Release and hook observation complete despite one unusable historical job
  record. Preserve the original timestamps and identify unavailable timing
  explicitly; do not manufacture a duration or mark an unmeasured check passed.
  Separate source-record failures so later sources are still examined. Handle
  transient read-API errors with bounded retry, and let the scheduled hook check
  run independently of unrelated release-scan failure.
- All six maintained Python entry points are replaced: CI scope/link selection,
  release preparation, tagged dispatch/wait, revision-bound asset pack/restore,
  release/hook timing observation, and bounded pre-commit checks. Their six
  Python tests become package-local Go tests or Node tests beside the TypeScript
  hook, as applicable. Preserve actual behavior
  that is still required, including exact-revision checks, safe archive restore,
  atomic tags, rerun reuse, and publication ordering.
- The twelve tracked Python helpers under `.agents/skills/` are also in scope:
  seven model-resolution copies, chapter, promise, sprint ledger, and two
  semantic-index/evaluation helpers. Replace their supported behavior in Go and
  update their maintained skill instructions/references. These are checked-in
  executable helpers, not frozen legacy or a reason to retain Python. Preserve
  any applicable provenance records when maintaining borrowed content. This is
  not a change to unrelated globally installed skills or upstream projects.
- Inline Python is removed from both appcast verification and macOS notary JSON
  parsing. Update workflows, Justfile, lefthook, and maintained release references
  to use the replacements; delete the old Python files and runtime requirements.
  This explicitly includes the Mac python3 preflight, Python unittest gates,
  hook test-input globs/startup accounting, and runnable documentation examples
  such as docs/context-budget.md and skill Python/venv setup instructions. Replace
  the Python test gate with the affected Go package tests; do not remove coverage.
  Delete generated Python caches; no Python cache-ignore entry currently exists
  in .gitignore, so there is no ignore rule to remove there.
  Frozen legacy code and historical research notes are not rewritten.
- The three-second hook budget and mandatory staged-content/index/format checks
  retain their existing semantics. Compilation or module downloads must not
  consume the hook coordinator's startup. Existing Go smoke tests still use
  their compiler cache; cold checks retain the existing advisory budget behavior.
  Documentation-only commits must retain their lightweight path. No broad build
  is moved into a push hook, and cold/unfinished smoke checks remain reported as
  unfinished rather than green or a performance-only blocker.

Hook startup decision: use a TypeScript .mts coordinator under `scripts/`, executed
directly by the existing Node toolchain with erasable TypeScript and only Node
standard-library APIs. There is no transpiler, package install, or separate
compiled helper to bootstrap. A small Bash lefthook launcher keeps the
documentation-only mandatory-check path independent of Go/Node; source changes
invoke the TypeScript coordinator, which performs the mandatory checks, input
agreement, parallel smoke execution, shared deadline, descendant cleanup, and
timing report. Fresh worktrees can immediately run it using the existing Node
prerequisite. Explicit warm-hook warms the existing Go smoke compilation, not a
new hook executable. Cold tests can time out as today, but smoke is never skipped
because a new coordinator binary was not prepared.

Declare Node >=24.13.0 in the maintained runtime requirements and the Bash
launcher; current CI's Node 24 satisfies it. Before invoking TypeScript for a
commit that changes source files, reject an unsupported Node runtime with a
clear prerequisite error, preserving
the existing rule that actual execution/prerequisite failures block. It is not
a passed or merely slow smoke check. The documentation-only shell path still
works without Node. Use explicit .mts modules and imports, with no TypeScript
syntax that needs transpilation. A local probe on the exact installed Node
24.13.0 at `/Users/tyler/.local/share/mise/installs/node/24.13.0/bin/node`
executed type annotations and a Node standard-library import successfully
with empty stderr; this is syntax/runtime compatibility evidence, not a benchmark
of the future full coordinator.

Test-gate decision: replace Python unittest discovery with explicit automation
Go tests, `node --test scripts/*.test.mts` for the hook tests, and `go -C <skill>
test ./...` for each affected skill module. Nested modules are not covered by
root `go test ./...`. Include their normal affected vet/lint checks explicitly
as well. Justfile's existing check-hooks target runs the hook tests; its automation check
and CI gate cover the Go commands and skill modules. Warm-hook continues to run
the fixed fast smoke pack and does not acquire the full helper test suite.
Keep model-resolver copies self-contained with the existing per-skill ownership;
do not add a shared installed dependency or a new synchronization framework.

Proof required before calling the replacement complete:

1. Package tests reproduce the real cancelled-job timestamps, a failing API
   read, subsequent-source continuation, and independent hourly hook execution;
   existing required release and hook behavior remains covered.
2. The maintained automation has no executable Python references or .py files.
   Run on the actual supported runners with Python unavailable to this path;
   exercise a clean checkout and the container cache layout, not just a warm
   developer tree.
3. The real observer completes its historical scan with the known gap exposed,
   and the hourly hook measurement completes. This must be seen in actual runs.
4. A real tagged release passes qualification, archived CLI/plugin/web tests,
   signing/notarization, and publication. Download and verify the resulting
   packages, feed signature/version, and Homebrew entries. Preserve the
   ten-minute delivery maximum and three-second hook budget; measure the
   five-minute delivery goal without presenting a goal miss as build failure.
5. Exercise each replacement skill-helper command on controlled fixtures:
   model resolution keeps configured tier/binding behavior; chapter and sprint
   ledgers keep IDs, links, and update semantics; promise operations preserve
   file locking and atomic state changes under concurrent processes; semantic
   evaluation/metrics retain their input/output and evidence-link behavior.
   Invoke the Go commands via go run from their documented source context,
   relying on the normal Go build cache outside the hook budget. Update the
   skill-local invocation paths so installed copies remain self-contained:
   helper source and its small Go module context stay beside the loaded
   SKILL.md, with the existing yaml.v3 version pinned where needed, and commands
   run with explicit -C pointing there. They must not depend on an absolute
   path to this Gimbal checkout or the target project's module dependencies.
   Borrowed-skill refreshes must preserve these Go entry points and pass the
   same no-Python acceptance checks; record the local ports in existing
   provenance where available, rather than blindly restoring upstream Python.

The [round-03 review](../../reviews/202610141425-release-diagnosis-round-03.md)
found no material defects in the diagnosis, but identified plan gaps in skill
helper scope, Python dependency inventory, and Go hook startup. All three were
accepted and resolved above from the tracked inventory and existing hook rules.
Its request for separate port/fix commits was accepted. Its request to inventory
.gitignore was checked: no Python cache entry is present.

The [round-04 review](../../reviews/202610141423-release-diagnosis-round-04.md)
again found the diagnosis accurate, but identified delayed observer repair,
helper-binary startup that could skip fresh-worktree smoke checks, and missing
behavior/sync proof for the skill ports. Accepted: observer recovery now comes
first; the hook uses directly executable existing TypeScript/Node instead of
a compiled helper; skill behavior and future sync constraints are explicit.
No further permission question is needed: Tyler requested Python replacement
and delegated language choice, and the plan now includes the review's alternative
of explicit skill-port proof. Skill ports are sequenced last and can be delivered
separately from observer recovery; the complete no-Python outcome still includes
them.

The [round-05 review](../../reviews/202610141427-release-diagnosis-round-05.md)
found the diagnosis accurate and earlier plan issues resolved, but identified
missing explicit test gates for nested skill modules/Node hook tests and an
undeclared TypeScript runtime prerequisite. Both are now explicit above. Its
proposal to treat an unsupported runtime as an advisory skip was not adopted:
the current Python hook blocks on prerequisite/execution errors, and AGENTS.md
requires actual failures to block; only elapsed-budget failures are advisory.
The [round-06 review](../../reviews/202610141431-release-diagnosis-round-06.md)
concluded only nitpicks remain: no material diagnosis error or unresolved plan
gap. Corrected the Justfile target name and the wording about Node prerequisites.
Its claim that no Node 24.13.0 exists on this machine is incorrect: I repeated
the probe using the explicit mise path above, which reports v24.13.0, returns
the expected output, and produces empty stderr. That version is the tested
compatibility floor, not a claim about the earliest Node release to support the
syntax. Claude separately verified the same behavior on Node 24.21.0. The full
coordinator and migration still require the behavioral proof described here.
