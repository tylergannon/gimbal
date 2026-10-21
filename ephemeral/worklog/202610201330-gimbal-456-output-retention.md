# gimbal-456 recoverable output loss (revalidated at be0782e3)

decision: #456 does not reproduce at be0782e3. No `Ack` exists in remote_execution.go, effects, backend, or taskdaemon. `retireLocked` needs `released` (set only by scope cleanup) and 5 minutes since completion. Reading never retires. No source change made.
decision: added internal/taskdaemon/artifact_consumption_test.go (real daemon behind an HTTP proxy that kills the first stdout /tasks/bytes call, public gimbal.RunCommand, full scope cleanup, then reads the artifact and polls). Mutation check: dropping the retention clause in retireLocked fails it with a 404.
friction: the issue body's "not rerun against HEAD" caveat was accurate; the cited line ranges (remote_execution.go#L59-L89, bytes.go#L43-L65) no longer contain ack code. Old repro steps can't be followed literally.
note: the message path (messageProcessResult/effects.Await) only listens to events; nothing consumes output on delivery. Not exercised with a failing sink, only inspected.
note: internal/environments/operations.go helper path returns on a stdout read error without running its cleanup() (mutation receipts), so output stays recoverable there too. Not tested; outside this job's files.
