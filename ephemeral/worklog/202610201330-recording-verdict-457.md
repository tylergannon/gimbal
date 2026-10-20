# #457 recording failure vs Run verdict

decision: Run returns the execution verdict only; the run store's recording failure (first one) rides in Complete.RecordingError and the new RunRow.recording_error. Source: definition-of-done "Execution and observation" + issue #457.
discovery: At HEAD (v0.18.0) the issue's literal repro (observation-deltas.jsonl replaced by a directory) already returns nil, because #616 split observationFailure (not joined) from recordFailure (run store, joined). The live contradiction was the run-store path: Run returned the record error, and eventResult returned before writing run.jsonl/observing, so the snapshot stayed "running" and no run_ended/complete was logged.
decision: eventResult now still writes the run log and observes when the run-store append fails, and returns the store error to callers that need it.
doc_bug: Run's doc comment said it joins the first recording failure; updated in run.go. docs/ untouched.
friction: `go tool skgo generate` needs web/node_modules (vp install) and `-web web` when the worktree dir is not named gimbal; types.ts RunRow.recording_error was hand-edited to the generator's form, not regenerated.
discovery: A >=512KiB turn result with a broken blobs dir makes Generate itself return the record error (store payload). Left alone: not finalization.
discovery: web/ and cmd/gimbal tests fail here without a built frontend (skgo.manifest.json missing); unrelated.
