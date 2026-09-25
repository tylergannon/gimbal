# Pi and Diffusion Router research — 2026-09-25

doc_bug: The Portal Pi guide labels the integration beta and provides only a minimal model ID; Pi 0.87.1 silently defaults metadata to 128000 context, 16384 output, no reasoning or images, and zero costs -> verify Router capabilities and populate explicit model metadata before production routing.

decision: Current official Pi is `@earendil-works/pi-coding-agent` 0.87.1; old `@mariozechner/pi-coding-agent` 0.73.1 is deprecated -> pin a version and test that version's RPC protocol. In particular, current `agent_settled` is the session-level idle boundary; old docs only described `agent_end`.

decision: Prototype a per-session Pi RPC child for Gimbal because it supplies prompt, steer, abort, compact, model control, and durable resume with explicit `PI_CODING_AGENT_DIR` isolation. A localhost mock proved provider parsing, bearer interpolation, Chat Completions request construction, events, and cross-process session resume; no real Router call was made.

decision: Pi core RPC has no documented per-prompt output schema -> preserve Gimbal's exact validator and plan bounded JSON repair or a verified trusted extension. Treat schema conformance as a material parity gap versus existing harnesses.

friction: An unrestricted `rg` over bundled Pi JavaScript printed megabytes of minified code -> restrict searches to docs and TypeScript declarations when researching current package internals.

Detailed citations and live qualification gates: `ephemeral/projects/gimble/diffusion-router-research/pi-findings.md`.
