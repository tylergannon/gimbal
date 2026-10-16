# SKGO development upgrade

decision: Replace Gimbal's retired SKGO dev API with ordinary skgo dev and the explicit application development factory; retain the instance across handler reconstruction.
correction: The rollout changed from a pinned development commit to the published 0.34 minor series. The local module cache provides v0.34.4 at fd180adbcb53084ff1cf524841a82b166c6c47cc.
friction: The managed worktree tool cannot create a repository worktree from a projectless chat; git worktree add created the isolated checkout instead.

friction: Published adapter 0.34.4 accepts Kit ^3.0.0 as peer but dev rejects installed 3.0.1; filed SKGO #339. Pin Kit 3.0.0 and preserve Gimbal's existing query-return repair alongside the supplied queue patch. SKGO kit-patch --check enforces its own exact patch bytes and package-manager pin, so it cannot verify this consumer's combined patch; application checks must cover its actual behavior.

correction: The released v0.34.7 Go module pairs with npm adapter v0.34.5. Adapter publication is content-based; do not assume every Go tag creates an adapter at the same version. The registry has no adapter 0.34.7, and unchanged released adapter bytes are accepted by the v0.34.7 generator fingerprint check.
decision: Keep Kit 3.0.0 for this qualification because it is the exact released adapter/dev support pin. A successful earlier production build on 3.0.1 does not qualify its development behavior or the consumer's combined query-return/queue patch.
friction: The staged whitespace check treats unified-patch context prefixes as source whitespace. Re-emit the combined Kit patch with zero context, reinstall through vp to update its lockfile hash, and compare the installed patched files byte-for-byte; the queue and query-return changes remain identical.
