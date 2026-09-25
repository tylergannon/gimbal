# Diffusion Router research

Start with the [research synthesis](RESEARCH.md). It records the 2026-09-25 provider and transport probes. Those code-level comparisons were made against the old `39462ce` checkout. The branch was subsequently rebased onto `origin/main` at `81dc3c8`, where Gimble has a current root Go application and an OpenCode adapter. Use [current-code handoff](CURRENT-CODE.md) for implementation against the latest code; treat references to `ephemeral/legacy/` in the original synthesis as historical.

- [Portal documentation archive](portal/README.md): all 12 Connect guides, grouped by coding tools, agent tools, and API surfaces, plus a 22-model catalog snapshot.
- [Pi findings](pi-findings.md): RPC adapter, configuration, mock turn/resume proof, and schema gap.
- [Current-code handoff](CURRENT-CODE.md): current five-method harness contract, routing seams, and Pi implementation gaps.
- [Codex findings](codex-findings.md): per-thread provider selection, app-server process options, and CLI fallback.
- [OpenCode findings](opencode-findings.md): v1/v2 config mismatch, CLI/server turn paths, and mock resume proof.
- [Claude Code findings](claude-findings.md): beta gateway route and Anthropic's non-Claude support limit.

No production Router credentials or paid inference calls were used in this research.
