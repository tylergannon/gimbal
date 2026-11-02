# Research assignment: externally controlled desktop Codex sessions

User intent: Gimbal's harness should initiate, observe, steer, and cancel agent sessions inside the installed Codex/ChatGPT desktop app on a Mac. Those sessions must retain native Computer Use. The user proposes installing a Gimbal plugin into Codex as the integration boundary. Do not substitute a design where a pre-existing agent polls a queue unless documenting a distinct fallback.

Research whether this is supported and what exact mechanism, if any, provides it. Distinguish a public app-server session, a desktop-owned local session, a cloud session, remote-control relay, a plugin MCP subprocess, and a native desktop tool. No predetermined conclusion. Prior assertions are hypotheses, not authority.

This is read-only investigation plus saved research notes. Do not install plugins, run downloaded programs, invoke private endpoints, create/control sessions, change permissions, inspect secrets, or alter live desktop state. Do not spawn additional agents. Other researchers work concurrently: do not edit their files or revert their work. Do not commit. Write only your assigned lane report and source cache.

Save every visited source relied on, including full fetched documentation and relevant source files. The source cache is outside the Git repo; source code and run logs do not belong under ephemeral. Record source URL or original local path, retrieval time, revision/build, SHA-256, cached absolute path, and line citations. Search snippets alone are not evidence. Return a compact claim table with exact citations, quoted evidence where useful, uncertainty and bounded negative-search scope. Separate observations from inferences; the parent independently validates the claims and owns judgments. Keep access errors as limitations, never invent page contents.

Known local sources: public Codex checkout /tmp/codex-public-439.lDvlJA/codex at 07757f448f2d3cf475d0f328732c1c277bce8aa1; installed desktop /Applications/ChatGPT.app (26.1007.21159 build 20052); extracted main /var/folders/lt/09rsy64x65s_0fp2b8zq3n7m0000gn/T/gimbal-cua-desktop-nydm_tz_/.vite/build/main-39FdJ_vp.js. These need not be the same revision. Existing findings are in ../findings.md but inspect primary evidence independently.

## Your lane

Trace the public Codex app-server/SDK/plugin interfaces. Can an external client connect to an existing desktop-owned session host, create/start/resume/steer/interrupt threads, and retain native computer-use wiring? Examine transport defaults, multi-client behavior, plugin configuration reach, discovery/auth, external client restrictions. Distinguish creating a fresh app-server from attaching to the one desktop uses. Investigate MCP sampling or hooks only if actually implemented/relevant. Focus on source; no runtime tests. Copy cited files into your cache with provenance and produce findings plus smallest unanswered question.

Save raw sources and manifest under /Users/tyler/Documents/Codex/2026-10-09/gimbal-session-control-research/codex. Write your report to /Users/tyler/.codex/worktrees/issue-439-proposal/gimbal/ephemeral/research/issue-439/computer-use/session-control/codex-report.md. State the actual model/harness and session ID if available.
