# Independent validation of PR 205

Verdict: **YES** for the intended durable-snapshot join/recovery functionality.
Implementation SHA: `6f0ec50959360aba14ec9bed92c70c35e85ff709`.
This validation was run independently of the implementer against the rebased
production build on 2026-09-14. No implementation files were changed.

## Demonstrated behavior

- Existing production Chrome scenario passed: SSR without JavaScript, concurrent
  partial text/reasoning/tool events, reload, authoritative final replacement,
  completion and completed-run server restart.
- Independent production probe withheld hydration JavaScript while two turns
  emitted events using colliding provider-local message/tool IDs. With 40 extra
  text deltas per turn, the browser caught up through 100 transactions without
  a replacement snapshot. With 2,000 extra deltas per turn, retention was
  exceeded and the browser explicitly replaced state with one latest snapshot.
  Both turns displayed the exact expected text including every appended `x`.
- A forced EventSource error exercised the production retry handler. The second
  connection used the last fully applied position, not the initial SSR position.
- Both cases killed the server with a **nonempty** journal suffix after the saved
  checkpoint. Before restart, the probe replaced every journal byte before the
  checkpoint offset with invalid JSON and removed raw run/session logs. Recovery
  still matched the entire uninterrupted public snapshot with deep equality,
  and the production browser rendered the identical transcript after re-entry.
  This directly demonstrates that restoration starts at the saved byte offset
  and does not need historical logs or replay the saved prefix.
- Focused Go tests passed for nonempty suffix restoration, interrupted trailing
  journal writes and retained-cursor versus stale-stream joins.

## Fresh real harness runs

Both used the production browser and the same implementation SHA above:

- Codex `gpt-5.6-luna`: completed run
  `01M2GW17WPCQVD1DNFN3TGV40H.observation-proof`, 28 frames; shell tool and final
  answer markers visible. Artifacts:
  `/var/folders/lt/09rsy64x65s_0fp2b8zq3n7m0000gn/T/gimble-live-codex-J0hDbm`.
- Claude `haiku` (`claude-haiku-4-5-20251001`): completed run
  `01M2GW18PJMT9ZHZZW9FRGZ6WZ.observation-proof`, 39 frames; shell tool and final
  answer markers visible. Artifacts:
  `/var/folders/lt/09rsy64x65s_0fp2b8zq3n7m0000gn/T/gimble-live-claude-P8Qj4T`.

## Repeatable commands and measurements

From the repository root:

```sh
go build -o /tmp/gimble-observation-proof ./ephemeral/research/issue-130/proof
go build -o /tmp/gimble-independent130 ./ephemeral/attest/issue-130/independent-runtime
go test ./internal/observation -run 'TestRestartRestoresSnapshotAndOnlyItsDeltaSuffix|TestInterruptedDeltaTailDoesNotAdvanceRecoveredCursor|TestJoinResumesRetainedSuffixOrReplacesFromSnapshot' -count=1 -v
cd ephemeral/research/issue-130/proof
pnpm --ignore-workspace exec playwright test --config playwright.config.ts
node live.mjs codex
node live.mjs claude
node independent.mjs
```

The independent fixture copies the existing deterministic production-runtime
fixture and adds configurable text deltas; it does not replace the application
handler, observation store, SSR or browser reducer. Chrome is required.
The compact measurements and local screenshot locations are in
`ephemeral/research/issue-130/proof/independent-results.json`.

These are single-machine samples, not latency benchmarks. The longer case has
fewer suffix bytes because it stopped closer to its most recent periodic
checkpoint; this is not evidence that longer histories intrinsically restore
faster. Exact suffix byte lengths come from the saved offset and file size;
the poisoned-prefix/deleted-log experiment independently proves the relevant
historical data is unnecessary. We did not use OS syscall tracing to count
physical I/O. Initial SSR measurements precede the added deltas; restored SSR
measurements include the full accumulated state. Long hydration uses a reset,
so its delta frame array is empty; the short case records incremental payloads.

Screenshots and raw harness artifacts remain local; the runnable probe and
summarized measurements are reviewable in this change. No hardware power-loss
or arbitrary filesystem-failure injection was performed.
