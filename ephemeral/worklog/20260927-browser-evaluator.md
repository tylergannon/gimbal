# Browser evaluator implementation

direction: Active user goal authorizes all fifteen browser/evaluator claims. User explicitly requires Gimbal and subagents. Retain current task worktree and PR398; use Gimbal-driven design/implementation/review, manager-owned integration and live validation.

finding: Authoritative starting HEAD remains 23c02f6 and tracked tree is clean. Docker Desktop socket initially absent; starting the existing installed Docker application is the next environment action. No runtime claim has yet been established.

- Baseline root/execution/linter/validateproduct package tests passed at 23c02f6.
- Gimbal design run 01M3K0ABQHXKP9HQRQDDMWR4G5 completed on the default
  Claude Fable 5.1; independent Gimbal design review is in progress. The
  installed run-prompt command cannot resume a finished run, so revisions
  must receive the previous artifacts explicitly.
- Docker Desktop and a task-owned Temporal dev server are running. Existing
  compose Postgres was started without resetting its volume. Prepared external
  TodoMVC JavaScript ES6 at ff43b02e59dfa604386bb382034b2cd07c2bcd8a.
- The image agent built the separate consumer image successfully. Browser/media
  live checks are still in progress. A Linux arm64 worker was cross-compiled
  outside Git for later injection. Real Codex authentication is pending the
  user's selection; no host login state or API credential has been copied.

- Independent design review found material image, cancellation, lifetime-lint,
  generator and invocation gaps. Accepted all five findings and also corrected
  a fixed wrapper path that would collide for two browsers in one workdir.
  The revised design is awaiting a complete second review; implementation has
  not started and its claims are not yet established.
- The external consumer image's actual browser/media probe succeeded. I viewed
  its TodoMVC screenshot and decoded its entire MP4 with ffmpeg (exit 0).
  video-stop must precede close or the recording is lost. No host credentials
  were read or copied. Public Docker pulls needed an anonymous client config
  because the desktop credential helper hung; the Docker daemon itself worked.
- A separately mounted Linux worker executed inside the image and reached its
  expected missing-configuration error. This proves executable compatibility,
  not a completed Temporal activity or authenticated provider turn.
- Shared-data config now mounts only the canonical evaluation directory;
  runtime credentials and the worker binary remain separate read-only inputs.

## Design converged

Round 07 reports only nits. Runtime design is accepted for implementation.
Worker recipe executed successfully and produced a static Linux arm64 binary.
Use browserShellQuote to avoid colliding with the existing test helper. The
two-minute video encoding budget remains a documented limitation, with raw
finalized WebM retained. Three disjoint Gimbal implementation assignments start
now; evaluator migration follows the browser API. Authenticated proof still
awaits the worker credential choice.

## Evaluator integration

Manager owns validateproduct and maintained usage skills while the three Gimbal
workers implement runtime/generator, linter and backend. The evaluator now uses
browser scopes, environment encoding, remote-readable input checks and optional
report-only synthesis. Focused tests cover cancellation retaining finalized
video, task/debrief failure, sibling continuity and cleanup failure blocking only
that tester's encoding. Local generator sees browser scopes and encode commands.

A PATH-only fake playwright-cli was bypassed by host zsh startup files; the first
runs accidentally invoked the installed browser CLI. Lifecycle close commands
completed, but test marker assertions exposed the mismatch. Empty ZDOTDIR
isolates the test shell and produces the intended fake lifecycle. This fact was
steered to the browser worker. Do not interpret those accidental browser runs as
Docker/Temporal evaluation proof.

## Implementation convergence

All three Gimbal implementation tracks completed. The manager rejected the
linter's extra generic-parameter restriction: typed generic helpers preserve
the instantiated browser type and are allowed; explicit conversion to any is
still diagnosed. The correction ran through another bounded Gimbal assignment.
Combined Gimbal lint passes. Removing PlaywrightCLI also required updating the
web start-lifetime fixture and validation examples; readiness still blocks that
fixture before any browser or model use.

The backend worker reports real no-model readiness and command cancellation
checks against task Docker/Temporal/Postgres. Confirmed cancellation retained
the environment, and a follow-up command verified the sleep process had ended.
Authenticated supervision is explicitly unrun, pending a worker key file.
Full application build, frontend unit/format checks and six browser regression
cases passed. Runtime tests and backend race tests passed in the workers.
Repository hooks and independent whole-implementation review follow.

## Committed implementation and real entry-point attempt

Implementation c0489014 passed the full commit hooks and was pushed. Controller
and Linux worker were rebuilt from that revision. A real validate-product run
with the prepared TodoMVC suite and all roles set to gpt-5.6-luna failed at worker
readiness: the worker had neither Codex authentication nor an OPENAI_API_KEY
secret. The command returned nonzero before the app service or browser could
start. This confirms the credential blocker, not evaluation success. No credential
was provisioned. Independent whole-implementation review is running via Gimbal.

## Independent review

The complete implementation review at c0489014 reported only nits. It included
scratch-module lint cases, real host wrapper/browser use, Docker recording and
encoding, and independently repeated no-model Temporal integration checks.
Authenticated model execution remains unproved. Fixed the shell nit by making
wrapper writing and chmod separate set-e commands, and added the missing
readability and report-only screenshot-correction cases. Focused checks pass.
The new startup test verifies that an unwritable wrapper cannot start a browser.
