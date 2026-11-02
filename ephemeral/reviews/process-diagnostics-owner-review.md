# Owner review: process-leak diagnostics

Reviewed runtime source d7ce4722, clean built revision 64ca739d, and Sonnet's
independent final review in process-diagnostics-round-05.md.

Verdict: recommend merging. No blocking findings remain. This implements the
approved temporary observer; it does not establish that the historical process
leak is fixed or identify its cause.

The important boundaries hold: flag-off returns before observer setup; launch
birth identity is captured before Wait can reap; logical completion and reap
are distinct; uncertain shared-daemon candidates are separate from ownership;
the shared cohort closes at observation end. Existing completion, cancellation,
and native execution remain authoritative. Diagnostic errors are gaps, not run
failures. No diagnostic path kills a process or introduces a cleanup lifecycle.

Review and actual use caught material reporting defects, all corrected: a late
first census was presented as pre-work baseline, a worker reap was reported as
the borrowed daemon's reap, and later daemon activity entered an already closed
cohort. Tests now exercise these boundaries with real subprocesses. Darwin
executable names also correctly terminate at the first NUL.

Primary validation included the non-live suite (80 packages passed at a251b230),
current vet/custom lint, focused repeated race checks, current source CI, and
built-app flag-off/on runs using luna. The flag-off cache comparison was
unchanged. Actual native tool turns succeeded; completed workers reaped and
known-owned survivor counts were zero through grace. Some uncertain shared
counts were nonzero immediately during teardown, then zero at 10 and 60 seconds.
Sonnet independently ran a native Luna workflow through grace and reran the
positive surviving-child, long-service, and post-end-child tests under race.

Accepted limits: the 500 ms sampler can miss short ancestry and children born
in its final interval; first-census timing is explicit. Shared counts are
candidates, never proof of task ownership. Worker-local observations can end
before grace when their host exits and overlap the parent's view. Agent run
correlation can be unavailable. Positive leaks were exercised in real subprocess
package tests, not by deliberately leaking the production app. macOS was run
live; Linux was compile/vet checked only, and other platforms report unavailable.
Logs omit arguments, environment and prompt/tool contents, use private rotating
files, and close each write. Removal involves the internal package and explicit
hooks/metadata plumbing; no root public API was added.

Only non-blocking limitations remain. Further accounting infrastructure or
automatic remediation would exceed the approved scope. PR #667 is left
unmerged for Tyler's review of this assessment.
