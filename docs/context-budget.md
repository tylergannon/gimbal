# Claude Code context budget

Every session starts with a fixed cost: tool schemas, the skill listing, the
agent listing, MCP instruction blocks, `CLAUDE.md`, and memory. On this
machine that base was **~53.5k tokens before a single word of work**. This
doc records what it is made of, how to measure it, and which levers actually
move it. All numbers come from measurement, not estimation; tokens are
`chars / 4`.

## Measuring it

Claude Code writes a `prompt_snapshot` attachment into every session
transcript containing the exact tool schemas and system prompt blocks it
sent. That is the ground truth:

```bash
f=$(ls -t ~/.claude/projects/<project-slug>/*.jsonl | head -1)
python3 - "$f" <<'EOF'
import json, sys
for line in open(sys.argv[1]):
    d = json.loads(line); a = d.get('attachment') or {}
    if a.get('type') == 'prompt_snapshot' and a.get('tools'):
        tools = a['tools']
        total = sum(len(json.dumps(t)) for t in tools)
        print(f"{len(tools)} tools, {total} chars ~{total//4} tok")
        for t in sorted(tools, key=lambda x: -len(json.dumps(x))):
            print(f"  {len(json.dumps(t)):7d}  {t['name']}")
        break
EOF
```

The other blocks (skills, agents, MCP instructions, CLAUDE.md) arrive as
`attachment` records in the same file; sum their `rendered` content by
`attachment.type` for a full breakdown.

To test a change without touching your real config, run a throwaway headless
session with an override file and re-read its snapshot:

```bash
claude -p "reply with just: ok" --model claude-haiku-4-5-20251001 --settings /tmp/probe.json < /dev/null
```

## Where the tokens went (baseline, 2026-09-15, CLI 2.1.270)

| Block | Chars | ~Tokens |
|---|---:|---:|
| Tool schemas (21 loaded) | 144,824 | 36,200 |
| Skill listing (65 skills) | 25,389 | 6,350 |
| System prompt | 11,498 | 2,875 |
| Agent listing (15 agents) | 10,531 | 2,630 |
| MCP server instructions (4 servers) | 7,575 | 1,895 |
| CLAUDE.md + MEMORY.md | 6,659 | 1,665 |
| Deferred tool names (~126) | 4,320 | 1,080 |
| env / session context / date | 3,158 | 790 |
| **Total** | **213,954** | **~53,500** |

Tool schemas dominate, and one tool dominates them: `Artifact` alone was
**76,497 chars (~19.1k tokens, 36% of the entire base)**. Deferred MCP tools
are already cheap — ~126 names cost 1,080 tokens total.

## How deferral actually works

Tools listed by name only ("deferred") cost ~10 tokens instead of their full
schema; the model loads a schema on demand with `ToolSearch`. The decision,
from the CLI binary:

```js
function shouldDefer(tool) {
  if (tool.alwaysLoad === true) return false;
  if (isHardNonDeferrable(tool)) return false;   // ToolSearch itself, etc.
  if (tool.isMcp === true) return !toolSearchDisabled();  // MCP: deferred by default
  return tool.shouldDefer === true;              // built-ins: only if declared
}
```

So **MCP tools are deferred by default and built-ins are deferred only if
their own definition says so.** `WebFetch`, `WebSearch`, `Monitor`,
`NotebookEdit`, `LSP`, `Cron*` declare it; `Artifact`, `Workflow`,
`ScheduleWakeup`, `SendUserFile` do not. No setting flips a loaded built-in
into the deferred pool. The one remote override
(`tengu_non_deferrable_builtins`) can only take tools *out* of deferral.

That leaves two real moves: make a tool smaller, or remove it.

## Levers

### Applied

```json
{
  "env": {
    "CLAUDE_CODE_ARTIFACT_TOOLSET": "1",
    "CLAUDE_CODE_DISABLE_WORKFLOWS": "1"
  },
  "permissions": { "deny": ["ScheduleWakeup"] }
}
```

- `CLAUDE_CODE_ARTIFACT_TOOLSET=1` splits the Artifact monolith into
  `Artifact` + `ArtifactComments` + `ArtifactData` + `ArtifactCheck`, keeps
  only the core loaded and **defers the rest**. Artifact's schema drops
  **76,497 → 30,328 chars (−11.5k tokens)** with publishing, reading and
  updating intact; comments and the artifact DB cost one `ToolSearch` call on
  the rare turn they are needed.
- `CLAUDE_CODE_DISABLE_WORKFLOWS=1` removes the `Workflow` tool (−2.3k).
- Denying `ScheduleWakeup` removes it (−2.0k); `/loop <interval>` still works,
  only the self-paced mode is lost.

Measured end to end in a headless probe: **152,101 → 88,698 chars of tool
schema (−15.9k tokens)**.

Then, in the same pass:

- Deleted `~/.claude/agents/` (9 unused project agents, each carrying verbose
  `<example>` frontmatter): agent listing 10,531 → 2,947 chars (−1.9k).
- Removed the tractor skills — three symlinks under `~/.claude/skills/` plus
  the `tractor@tractor` plugin, which also drops its MCP instruction block and
  tool names (−0.8k). Plugin-sourced skills are immune to `skillOverrides`
  (the resolver returns `"on"` unconditionally for `source === "plugin"`), so
  disabling the plugin is the only lever for those.
- `skillOverrides`: 13 skills `off`, 10 `user-invocable-only` (−2.6k).

Cumulative: **~53.5k → ~32.3k tokens of base context.**

### Available

| Lever | ~Tokens | Cost |
|---|---:|---|
| `CLAUDE_CODE_ARTIFACT_{DB,ASSETS,MULTI_FILE,COMMENTS,PIN,PRESENCE,DELETE}=0` | −2,300 | Artifact 30,328 → 21,151 chars; loses those features |
| `skillOverrides` set to `user-invocable-only` for slash-only skills | −3,000 to −6,300 | Skill vanishes from the model's listing, `/name` still works |
| Trim or delete unused `~/.claude/agents/*.md` | −1,900 | Their verbose `<example>` frontmatter is pure listing cost |
| Disable unused claude.ai account skills (docx/pptx/xlsx/pdf/…) | −1,400 | Toggle at claude.ai → Settings → Skills |
| Disable unused MCP connectors (computer-use, claude-in-chrome, visualize) | −3,000 | Instruction blocks + loaded schemas |
| Deny `SendUserFile`, `AskUserQuestion`, `SuggestSkills`, `ListAgents` | −3,900 | Each removes that capability outright |
| Deny `Artifact` entirely | −7,600 | No artifact publishing at all |
| `CLAUDE_CODE_DISABLE_BUNDLED_SKILLS=1` | −2,065 | Blunt: also kills `/code-review`, `/simplify`, `/run`, `/init` |

### skillOverrides

Lives in `settings.json` at user, project or local scope:

```json
{ "skillOverrides": { "dataviz": "off", "df-promise": "user-invocable-only" } }
```

- `"off"` — gone entirely, not listed and not invocable.
- `"user-invocable-only"` — **removed from the model's skill listing, still
  runnable by the user as `/name`.** This is the right setting for any skill
  you always invoke deliberately; it costs nothing and loses nothing.
- `"on"` — default.

Verified: with `dataviz: off` and `design`/`claude-api` set to
`user-invocable-only`, all three disappeared from the session's skill listing.

### Permission deny removes tool schemas

`permissions.deny` in `settings.json` does not merely block a call — the tool
is dropped from the request entirely. A probe denying six tools cut the tool
list from 152,101 to 58,370 chars. `--disallowedTools` behaves identically for
one-off runs.

## Caveats

`CLAUDE_CODE_ARTIFACT_TOOLSET` and the `CLAUDE_CODE_ARTIFACT_*` feature flags
are internal experiment gates, not documented settings. They can be renamed or
retired by a CLI upgrade; the failure mode is benign (the fat schema returns).
Re-run the measurement script after a Claude Code upgrade to confirm the
savings still hold.
