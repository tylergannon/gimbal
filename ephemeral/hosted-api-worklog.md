# Hosted public compiler API worklog

The hosted API worker read the repository instructions and the Stage 3 assignment.
Shell inspection is currently blocked by `Too many open files (os error 24)`.
The discovered node REPL tool is unavailable to this worker. Coordination with
the parent preserves progress without changing the intended public API.

friction: Shell access recovered using `/bin/sh` with login disabled; apply_patch
remained usable throughout the descriptor exhaustion.

decision: The public web options configure the existing project host before
admission. Existing console cancellation and retained-artifact tests now enter
through the public API, so they cover actual consumer integration rather than
only the previous private host entry.

finding: Stage 2 round 03's material cancellation-attribution defect is already
repaired in the continued baseline: an ended run returns 404 without delivery,
busy/finishing returns its own 409, and delivery failures carry a separate 409.
Preserved those behaviors without another lifecycle redesign.
