# Diffusion Router Connect documentation snapshot

Retrieved 2026-09-25 from the signed-in [Router Connect portal](https://connect.diffusion.io/connect). The 12 guides below are the full Connect guide inventory visible at capture time. Each guide has a readable Markdown copy and a JSON copy of the portal's guide object. The JSON retains the original field structure and code snippets; the Markdown is generated from it. No API keys or account details are included.

The portal renders its guides from the static asset `https://connect.diffusion.io/assets/index-BT8dbohE.js` (SHA-256 `156cb2e3e5723a41363a8371213d425f6075fbc131532dc8c247273a0aa365f0`). [extract-portal.mjs](../extract-portal.mjs) extracted the guide array. The OpenCode and Codex pages were also checked in Safari against the extracted text. The catalog file was read from the authenticated portal API in Safari; a plain unauthenticated request returned HTTP 401. Catalog freshness applies only to its `fetchedAt` time.

## Coding harnesses and tools

- [Codex](coding/codex.md) — beta, reviewed 2026-09-23
- [Claude Code](coding/claude-code.md) — beta, reviewed 2026-09-20
- [OpenCode](coding/opencode.md) — beta, reviewed 2026-09-20
- [Pi](coding/pi.md) — beta, reviewed 2026-09-20
- [Crush](coding/crush.md) — beta, reviewed 2026-09-20
- [Cursor](coding/cursor.md) — documented, reviewed 2026-09-22
- [GitHub Copilot CLI](coding/copilot-cli.md) — beta, reviewed 2026-09-20

## Agent tools

- [Hermes](agents/hermes.md) — documented, reviewed 2026-09-20
- [OpenClaw](agents/openclaw.md) — beta, reviewed 2026-09-20

## API surfaces

- [OpenAI Responses](apis/openai-responses.md) — locally tested, reviewed 2026-09-20
- [OpenAI Chat Completions](apis/openai-chat-completions.md) — locally tested, reviewed 2026-09-20
- [Anthropic Messages](apis/anthropic-messages.md) — locally tested, reviewed 2026-09-20

## Model discovery

- [Authenticated catalog snapshot](catalog-2026-09-25.json) — 22 model IDs, `state=fresh`, fetched 2026-09-25T17:52:30Z. It lists IDs only, not model capabilities or harness qualification.

Portal guidance is source material, not an attestation of working Gimbal integration. The research report calls out discrepancies and the proof still needed.
