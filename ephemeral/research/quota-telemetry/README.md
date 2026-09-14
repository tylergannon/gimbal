# Quota telemetry: reading remaining subscription quota from Antigravity and Claude Code

Collected 2026-09-13 at Tyler's request. Background for the "how much of my
subscription did this run use" question, which is not in the Beta milestone
(quota windows have no consumer yet; see `ephemeral/research/beta-milestone/README.md`
on branch `claude/gimble-beta-milestone-a16ccf`, and issue #13, closed).

## Files

- `chatgpt-research.md`: ChatGPT deep-research answer. Copied from
  `~/src/inbox/ChatGPT Research.md`. Claims an official `agy -p "/quota"
  --output-format json` probe and an official `rate_limits` object in Claude
  Code's status-line JSON (v2.1.251+).
- `claude-research.md`: Claude deep-research answer. Copied from
  `~/src/inbox/Claude Research.md`. Claims neither tool has a documented
  non-interactive path; Antigravity's data is on a local Connect endpoint
  (`GetUserStatus` / `RetrieveUserQuotaSummary`), Claude's in undocumented
  `anthropic-ratelimit-unified-5h-*` / `-7d-*` headers and
  `GET /api/oauth/usage`.
- Gemini share `https://share.gemini.google/R7BJOGLehroE` (redirects to
  `https://gemini.google.com/share/a50d6d400fdd`): **not captured**. The page
  is rendered by the browser from a follow-up request; the served HTML holds
  no conversation text under any user agent tried, and no browser was
  reachable from the session that collected these. Export it (share menu,
  or copy the text) into `~/src/inbox/` and it goes in here as
  `gemini-research.md`.

## Where the two disagree

The two answers contradict each other on the official surfaces, so neither
is a design input until one claim is checked against the installed CLIs:

| Claim | ChatGPT | Claude |
|---|---|---|
| `agy -p "/quota" --output-format json` works headless | yes, since CLI 1.1.11 | no, slash commands do not render in `-p` |
| Claude Code status line carries `rate_limits` | yes, documented for Pro/Max | not shipped; open feature requests |

Both agree: the per-turn token usage Gimble already records is separate from
quota consumption, and quota is a utilization fraction per window (5-hour and
7-day), not a token count.
