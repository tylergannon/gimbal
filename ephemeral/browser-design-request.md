# Assignment: minimal implementation design

Read AGENTS.md and ephemeral/browser-evaluator-build.md. Inspect current browser
workflow, scope cleanup, Generate options, command/environment adapters and linter.
You own only ephemeral/browser-implementation-design.md. You are not alone; do
not revert others. Do not implement, commit, launch agents, or invoke Gimbal.

Write a short complete design for the fifteen claims with concrete Go signatures,
file ownership splits, integration points and compatibility/lifetime semantics.
Prefer smallest direct implementation over generic resource/provider registries.
Address environment-bound open/close, recording finalized before ffmpeg, scoped
browser access instructions on Generate, supported linter escape forms, and a
report-only evaluator mode so live validation doesn't publish GitHub issues.
Include configuration for separately injected worker binary and prepared browser
image, but don't implement it. Existing sessions own role/environment bindings.
Do not invent new runtime public API names except NewBrowser, Browser and WithBrowser
and necessary constructor configuration agreed through this design.

Use source/tests to ground details. Report exact edit ownership for independent
browser/linter and backend-packaging work, with evaluator migration integrated
after core API. Keep the design focused enough to implement now.
