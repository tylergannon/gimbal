# Remove avoidable test costs

correction: The user rejected periodic authentication-revocation cancellation of active responses. Remove that feature and its polling test; retain authentication on incoming requests.
decision: Cleanup policy belongs to each run, captured at Run startup. No mutable global test timeout. The ownership test uses 250 ms; production defaults remain unchanged.
decision: Executable lint/vettool/argument-routing proof belongs after the release build and must reuse that binary. Ordinary Go tests must not build the whole CLI.
correction: Third-party tokenizer performance is outside Gimbal's test responsibilities. Use ordinary spaced fixtures that still exercise Gimbal's budget, spill, completeness, and exact-output behavior; consolidate duplicate assertions.
friction: Whole-suite wall time is governed by the unchanged generator package. Improvements to overlapping packages cannot be summed or presented as an equivalent full-suite speedup.
decision: Next generator slice should reuse immutable typed source within an invocation, batch workflow roots, and preserve dependency compiler exports with overlays. SKGO already contains these patterns; removing NeedDeps alone is insufficient and previously lost a diagnostic.
decision: Separate SKGO current-cost issue https://github.com/tylergannon/skgo/issues/307 records one warm consumer generation at 6.68 s; it does not claim a measured regression or diagnosed cause.
