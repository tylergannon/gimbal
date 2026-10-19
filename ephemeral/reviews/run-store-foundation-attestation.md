# Attestation: run store foundation acceptance claims

Date: 2026-10-18. Attester: Claude, independent reviewer (rounds 01 to 03 on this branch). Read-only apart from this file.

## Subject

- Branch `codex/run-store-foundation` at e28e7352, clean tree, eight commits over `main` (merge-base c5c9b142, the baseline the request names).
- Claims assessed: the explicit acceptance statements in `ephemeral/run-store-request.md`, as evidenced by `/Users/tyler/.codex/visualizations/2026/10/19/01a1522a-1e4c-7942-bdce-772e9ed181e2/recording-foundation/review-evidence.md` and the artifacts beside it, checked against `run-store-design.md`, `run-store-session.md`, `AGENTS.md`, `docs/definition-of-done.md`, and the code.

## Claim by claim

1. **Five milestones, in order, a commit after each.** Holds. The commit sequence is a5c4bfb2 (storage), 78e02672 (transparent identity), 478d46b4 (executor/transport binding), df87da7e (observation and cleanup ownership), then e3f87d2a, 924ea3bf, 00e901f3, e28e7352 as the integrated-demonstration and qualification fixes. The milestone-to-commit mapping is one-to-one for the first four; the fifth is spread over four fix commits, which the request does not forbid.

2. **Unit tests where they protect meaningful contracts and failure modes.** Holds. Tests cover store ownership, conflicting writers, torn tails, corruption and blob digest verification, 1 MiB stdout/stderr readback from a separate process after spool removal, in-memory store substitution, HTTP and messaging identity acceptance, old-daemon rejection, live-writer cleanup exclusion, and housekeeping retry distinctness. Full uncached suite: 77 packages pass at e28e7352, run by me.

3. **Running application proof.** Holds for the paths demonstrated, with one caveat. The evidence names three live runs at e28e7352 (native observe-host with two Codex `luna` turns, remote verify-discount on a Linux/aarch64 Docker daemon, UI cancellation). I did not re-drive the application. I did reopen those three stores from a fresh process through only the public `runstore/directory` contract and found the same record, node, event, outcome and blob counts the proof reports, parent references to earlier-created nodes, blob SHA-256 verification, `cleanup_status: released` on all three, no `executor-spool` directories left, no proof container in `docker ps -a`, and `vcs.revision=e28e7352`, `vcs.modified=false` on the task binary. The evidence text now matches the artifacts.

4. **Independent adversarial assessment: seam well designed, completely implemented, no application path circumvents it.** Holds, with the limits below.
   - Design: the writer owns the segment log with a single flock-held handle; cleanup and termination take the store writer then `runner.lock` in the same order as runner startup, so nothing steals a lock before its owner flocks it. Blobs are content-addressed and verified on read. This matches the design document.
   - Completeness: every acceptance scenario in the session document that names a behavior has either a package test or a live observation. The exceptions are the edges listed under limits.
   - Circumvention: I searched the non-test tree for writers of run facts outside `runstore/`. The only truncating writer of run records is the legacy observation path in `internal/observation/terminated.go`, and it refuses new-format runs (`runstate.Recognized` guard) and otherwise only truncates a torn tail of the observation projection, which the design designates as derived, not authoritative. `event_persistence.go` appends lifecycle records to `run.jsonl` and `project.jsonl`, which remain projections; the executors' spool files are inputs that the live proof removed without changing readback. I found no path that appends or truncates authoritative facts around the store.

5. **Honest reporting of findings and limits.** Holds. The evidence names the unproved edges and the deferred slices rather than claiming them.

6. **Baseline, scope, and location.** Holds. Merge-base is c5c9b142. Scope is durable recording and directory reopen; replay, automatic recovery, durable human waits, SQL storage and orphan reconstruction are stated as subsequent. Runtime changes are in this worktree; the Gimbal View worktree is outside my view and I make no claim about it.

## Repository gates

Run by me at e28e7352: `go vet` clean; `golangci-lint` 0 issues under both the local 2.15.0 and the release-pinned 2.14.0 with unbounded reporting; `just fmt-check` clean over 168 files; frontend `vp run check` 0 errors, `vp test --run` 44 files, 201 tests passed. I did not run the full `just build`; the evidence says it last completed at 924ea3bf and that no frontend or generated transport source changed after it, which the diff of the later commits confirms.

## Limits

- I did not execute the live Codex turns, the Docker remote run, or the UI cancel. My independence is in reopening the resulting stores, binaries, and host state, not in reproducing the runs.
- Not separately demonstrated, only consistent with the code read: pending-buffer backpressure at the 4 MiB limit, segment roll at 16 MiB and last-segment-only truncation, the startup lock windows under a racing sweeper, host restart over a mix of successful and failed-body released runs, rejection of a context-only store override, and parallel completion inversion versus readback.
- Two nitpicks from rounds 02 and 03 remain open and are not acceptance blockers: a scope-ended node creation is latched as a recording failure, and session creation binds as a `turn_` operation.
- The lint commit e28e7352 tightens assertions in several unrelated test packages; a future flake there traces to that commit, not to the store.
- Rounds 01 and 02 are not checked into the tree; this attestation relies on their findings only through round 03's status list.

## Verdict

The explicit acceptance claims in the request are met at e28e7352 to the extent stated above. No material finding remains open. The unproved edges are coverage gaps, not observed defects.
