# Managed runner migration

- decision: Preserve existing repository capacity and move real jobs to disposable workers managed by the native service. Gimbal Linux retains two workers at 8 CPUs/16 GiB; skgo-project retains four at 3 CPUs/10 GiB and its separate native private evaluator.
- correction: The fleet also includes SKGO and gimbal-view; one project's worker limit is not the fleet limit.
