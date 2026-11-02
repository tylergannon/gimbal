# Process-leak diagnostics

decision: Tyler approved the temporary opt-in process observer and explicitly assigned Terra implementation and Sonnet validation. Build from released main in an isolated managed worktree; the earlier merged checkout remains referenced by Desktop tool processes and is preserved.

constraint: The startup-lock exclusion claim, observed bounded stabilization, and historical process-leak causation are separate. Diagnostics must record uncertainty for shared-daemon ancestry and must not become a cleanup ledger, automatic killer or workflow completion gate.

constraint: Root checkout has unrelated user edits and could not fast-forward safely. Do not stash, reset or edit them. Runtime proof output remains outside the repository; repeatable checks belong in package tests.

implementation correction: Logical owner completion and OS reap are distinct diagnostic events. End observations from existing result publication/service stop boundaries so a still-alive worker can be reported; ending only after Wait can detect descendants but can never identify the root worker surviving completion. Anchor birth identity before the wait goroutine may reap, and fail unknown identity as an observation gap.

friction: A new worktree has no generated frontend manifest. Prepare it with just build before broad Go tests; failures from absent assets do not justify product-code changes.

review correction: Shared-daemon first-census children are a timestamped baseline, not evidence of pre-work existence or task ownership; later first-seen children remain uncertain candidates. Record discovery delay so early work hidden in the baseline is visible as an observation limitation. Normal service stop initiation is neutral, with qualified warnings reserved for survivors at the later grace samples.

test correction: Repeated in-process tests cannot reuse a sync.Once logger pointed at a deleted t.TempDir. Run real observer fixtures in isolated subprocesses and verify the complete sampling path, not only classification helpers. Capture tool process exit codes explicitly; absence of FAIL text alone is not an exit-status assertion.

review correction: A parent worker's reap signal must not be logged as the sampled shared daemon's reap. Shared observation end is distinct from daemon ownership end/death; keep the signal only for the missing-result diagnostic boundary and label ages as observation ages.

proof correction: Absence of a sampled shell process is not proof no tool ran. The owned native session record for run 01M6D7ZPETD7489TY54HBD0Z98 contains executed cat/sleep and read-only inspection commands. Assess execution from that native evidence, and ancestry capture separately from the process sampler; the diagnostic log intentionally excludes command arguments and tool contents.

live correction: A shared grace sampler adopted processes born thirty seconds after observation end and labelled them as sixty-second candidate survivors. Freeze the shared cohort at the end boundary; later unrelated native activity is not retroactive evidence for the closed observation. This differs from following causally known owned descendants. Darwin process names are C strings and must stop at the first NUL, not merely trim trailing NULs.
