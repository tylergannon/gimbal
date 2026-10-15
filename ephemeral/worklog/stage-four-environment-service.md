# Stage 4 environment service

- decision: Environment setup uses the existing backend Process operation and a maintained desktop-environment CLI helper; it does not add a second remote filesystem protocol. Fixed daemon working directories are reported to that child explicitly and cannot silently override session placement.
- correction: An orchestrated conversation lease must be retained before preparing its checkout. Setup helper scopes are children of that lease, so releasing a probe cannot stop the worker before harness admission.
- friction: SSH tunnel liveness does not establish daemon liveness. Reconnect must honor bootstrap intent even when reusing a still-running tunnel after a previous bootstrap failure.
- decision: Helper mutation receipts preserve request/envelope identity across lost responses; probe cleanup failures and failed-readiness ownership remain persisted and visible instead of being discarded in a deferred cleanup.
- correction: Harness model catalogs are alphabetically grouped and their first entry is not a safe default. Setup metadata retains Haiku, Luna and Flash defaults and labels authentication unknown until there is real evidence.
