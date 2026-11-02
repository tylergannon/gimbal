# Temporary process-leak diagnostics

Tyler approved implementation of the process-diagnostics proposal in this chat
and assigned Terra implementation, Sonnet validation, and the task owner's
review. This is a new task after merged resource repair #665; its startup-lock
guarantee is not evidence that the historical process leak was fixed.

Implement optional debug observation enabled by GIMBAL_DEBUG_PROCESSES=1.
Keep it concentrated and removable, with a few hooks into existing ownership
boundaries. No new public API or general accounting framework. Existing
workflow and native cancellation/recovery semantics remain authoritative.

## Required behavior

- With the flag absent/off, diagnostics create no sampler, subprocesses or log
  files. Debug failures never change a workflow's authoritative result, block
  cancellation or become a completion gate.
- Identify directly launched Gimbal workers/commands by existing execution
  identity, executable, PID, birth identity, parent and process group where the
  platform supports it. Associate run/session/turn identity where actually
  available; explicitly record missing correlation rather than inventing it.
- Observe descendants during work and remember identity across reparenting.
  PID reuse must not attach a new process to the old owner. Sampling is not a
  complete OS process-creation audit: missed ancestry remains uncertain.
- Record process exit/reap and owner-end as separate observations. Inspect
  survivors at owner end and after observational grace intervals (proposed
  defaults immediate, 10s, 60s). An alive known-owned process after its owner
  ends is a suspect survivor, not automatically a proved leak.
- Borrowed native daemons, legitimate retained human sessions and deliberately
  active services have their own owner lifetimes. Process names alone never
  determine legitimacy. Shared-daemon descendants without established task
  ownership are labelled candidates, distinct from known-owned survivors.
- Reports support inspection of survivor ages and count trends across completed
  owners, without conflating unrelated Desktop clients with Gimbal work.
- Emit bounded, rotating private local diagnostics and warnings with a discoverable
  output location. No full command arguments, environment values, credentials,
  prompts or tool contents in process diagnostics. Bound retained observations,
  cleanly close observer resources, and expose dropped/unavailable observations.
- Observation only: never kill, archive, retry cleanup, restart services, enforce
  global resource caps or retain a cleanup lifecycle. Preserve existing final
  results and native uncertainty. Failures to enumerate or write are diagnostic
  gaps, not execution failures.

## Definition of done and proof

Package-local tests must exercise stable process identity/PID reuse, attribution
and missing ancestry, bounded state/log rotation, cancellation and flag-off/error
behavior where relevant. Real subprocess tests demonstrate that a deliberately
surviving child is reported after owner end, ordinary exited/reaped workers do
not persist in reports, and an intentional live service/separate client is not
mislabelled as an owned leak. Repeatable checks live beside the package they test.

Run the actual built Gimbal with the flag off and on against disposable state.
Observe worker/service lifetimes and a harmless tool turn using unversioned luna
where a provider turn is needed. Report exactly what was observed and any
platform/attribution limit. Do not use a global kill watcher or disturb the
existing Codex/Desktop/qualification services. Sonnet independently inspects
the full change and validates actual behavior; resolve material findings via
the same reviewer session and fresh review artifacts. The task owner reviews
the resulting code and proof before handing it back to Tyler.

Follow repository proof rules: no run logs, dumps, screenshots, result files or
standalone proof programs committed; ephemeral holds plans/reviews/notes only.
Summarize proof in this chat and the PR body. Open a reviewable PR with green
applicable checks; this request does not authorize its merge or publication.
