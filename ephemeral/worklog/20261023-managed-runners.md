# Managed runner migration

- decision: Preserve existing repository capacity and move real jobs to disposable workers managed by the native service. Gimbal Linux retains two workers at 8 CPUs/16 GiB; skgo-project retains four at 3 CPUs/10 GiB and its separate native private evaluator.
- correction: The fleet also includes SKGO and gimbal-view; one project's worker limit is not the fleet limit.
- decision: Fork PR source checks use GitHub-hosted workers; trusted repository builds use the managed Linux scale set.
- correction: Pinned manager enforces a host-wide two-Mac-VM limit. Preserve the third Mac job lane by retaining Gimbal's native release runner alongside SKGO's twoVM slots; do not claim a threeVM fleet from health counters.
