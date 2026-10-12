# Messaging backend delivery

decision: Current task explicitly excludes orchestration, placement/discovery, and steering. Keep runtime-loaded messaging selection and three live transport runs as the bounded goal; older proposals are context, not requirements.
friction: Root checkout has unrelated tracked third_party deletions and untracked research -> root synchronization was skipped, not repaired; task starts at fetched origin/main c9517a72 in its own worktree.
decision: Existing service-check is a useful unchanged real workflow specimen with agent, command, service, scope cleanup and final verification. Do not commit proof programs or run artifacts under Gimbal's repository policy.
decision: Existing wide execution interface is not the new messaging interface; normal action paths must directly use narrow messages without a demo-only runner or compatibility shim.
decision: Claude design review identified missing acceptance bounds, control starvation, unspecified catch-up mechanics and orphan cleanup vocabulary. Revised with explicit acceptance uncertainty, native-admission dispatch plus River control queue, repeat retained reads from original since with per-listener event-ID deduplication, and unconditional idempotent orphan cleanup. No general watchdog or replay machinery.
correction: Do not label preservation of still-used HTTP/Docker paths as a user-accepted permanent tradeoff; this slice does not replace those existing routes.
