# #457 recording failure vs Run verdict

decision: Run returns the execution verdict only; the run store's recording failure (first one) rides in Complete.RecordingError and the new RunRow.recording_error. Source: definition-of-done "Execution and observation" + issue #457.
discovery: At HEAD (v0.18.0) the issue's literal repro (observation-deltas.jsonl replaced by a directory) already returns nil, because #616 split observationFailure (not joined) from recordFailure (run store, joined). The live contradiction was the run-store path: Run returned the record error, and eventResult returned before writing run.jsonl/observing, so the snapshot stayed "running" and no run_ended/complete was logged.
decision: eventResult now still writes the run log and observes when the run-store append fails, and returns the store error to callers that need it.
doc_bug: Run's doc comment said it joins the first recording failure; updated in run.go. docs/ untouched.
friction: `go tool skgo generate` needs web/node_modules (vp install) and `-web web` when the worktree dir is not named gimbal; types.ts RunRow.recording_error was hand-edited to the generator's form, not regenerated.
discovery: A >=512KiB turn result with a broken blobs dir makes Generate itself return the record error (store payload). Left alone: not finalization.
discovery: web/ and cmd/gimbal tests fail here without a built frontend (skgo.manifest.json missing); unrelated.

2026-10-21 completion: generated source rejected the preserved omitempty tag.
RunRow recording_error is now an explicit string; regenerated Go encoder/decoder
and TypeScript match. Active provider event storage and post-native outcome
storage failures remain diagnostics and cannot replace native success/failure.
New 600KiB real-blob-directory failures cover native success and provider error;
the preserved partial source fails this regression. Durable-intent admission
before new native side effects is retained. The existing execution/recording
suites pass; empty-table serialization expectation updated for the new field.
Run transcript and overview expose recording health beside execution status;
a real server-render regression checks successful status and diagnostic text.
No session projection, Stop, provider routing, history, or activity edits.
