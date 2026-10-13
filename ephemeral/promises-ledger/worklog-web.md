# Worklog — pl-web (host wiring, remotes, UI)

- Stub `internal/promises` committed alone ("STUB promises API (drop on merge)").
  `Run` returns nothing in the stub; host ignores any return value anyway.
- First commit attempt failed the hook: `web/` tests need `web/build/skgo.manifest.json`;
  a fresh worktree must run `just build` before committing.
- Host: model resolution placed right after `promises.Open`, before the project is
  registered, so a bad `GIMBAL_PROMISES_MODEL` fails admission with nothing to unwind
  (plan put it just before `go p.cleanupExitedTasks()`, after registration). Host wraps
  the error with `GIMBAL_PROMISES_MODEL:` so the message does not depend on the package.
- `go build ./...` fails on plugins/* (no main) — pre-existing; use `just vet`.
- No Svelte MCP server in this session; ran the same checker as a CLI:
  `npx @sveltejs/mcp svelte-autofixer <file> --async` (from a scratch dir; inside web/ npm
  refuses because of devEngines pnpm). Zero issues on every touched file; remaining
  suggestions are the "function inside $effect" heuristic on the existing
  `stream.catch(navigateOnUnauthorized)` idiom and on `.set` of remote queries.
  Keyed a pre-existing unkeyed `{#each}` in ProjectWorkspace (upload issues) to clear it.
- Chrome MCP calls timed out in the PreToolUse hook; drove the real binary with the e2e
  package's Playwright (headless chromium) from scratchpad scripts instead.
- Design FEEDBACK.md (final) adopted: count line with segmented Sort on the right; single-tag
  chips with "All"; Tag sort by alphabetically smallest tag; "no quote" in collapsed rows;
  Added/changed stamps; "From the transcript (n)" with recording labels from the sidebar's
  loaded history (prop, no new remote) and "Earlier recording" fallback; voice-withdraw hint;
  ring-style error lines with server text verbatim; inline "Could not withdraw #n: …";
  disconnected banner; two empty states; recording-view helper text and list glyph.
- Sidebar pager mirrors the unused RecordingHistory.svelte ("Older recordings"/"Newest",
  page replaced not appended) per coordinator; plan said append into local $state.
- `setPromiseWithdrawn` wraps store errors in `skgo.Errorf(500, …)`: a plain Go error
  reaches the page as "Internal Error", which made the inline withdraw error useless.
- Screens are in ephemeral/promises-ledger/screens/ but uncommitted (AGENTS.md: no
  screenshots or run output committed).
