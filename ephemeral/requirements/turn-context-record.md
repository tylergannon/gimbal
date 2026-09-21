# Record a turn's prompt and its scope context separately

## Why

`Session.Generate` sends an agent `prompt + "\n\n" + context`, where context
is the rendering of every `Set` / `SetJSON` value visible from the ctx's
scope (`scopedPrompt` and `appendScopeText` in `session.go`, `scopeText` and
`visibleValues` in `scope.go`). `TurnStarted.Prompt` records that glued
string. The run page therefore cannot show "the prompt the workflow wrote"
apart from "the data placed with Set": it shows one wall of text. The detail
view redesign (issue 325, `ephemeral/design/issue-325-detail-view-proposal.md`)
needs the two apart.

What the agent is sent must not change at all. This is a change to what is
recorded and served, not to what is dispatched.

## Definition of done

1. For a turn started through `Session.Generate`, the recorded
   `TurnStarted.Prompt` is the prompt exactly as the workflow passed it,
   without the scope context appended.
2. `TurnStarted` also records which scope values were visible to that turn:
   one entry per key, in the order they were rendered, each naming the key,
   the key of the scope that owns the value (the nearest scope that set it),
   and whether the agent was sent the complete value inline or something
   shorter (an excerpt or a pointer to an artifact, as `renderVisible` and
   the scope index already do for oversized context). Entries do not copy
   value bodies: the values already live in the scope table. Use plain
   names (`Context`, `ContextEntry` or similar); do not coin nouns.
3. A `Generate` call that uses `WithScopeTemplate` records the same list of
   visible values. It need not record the rendered template text.
4. Internal callers that build their own prompt and call `dispatch` directly
   (the PromiseLoop planner turn, a supervisor's look, the interview) keep
   recording what they record today. An empty context list is fine for them.
5. The observation store's turn row (`internal/observation`) carries the new
   list, it is persisted in the turns table file, and it is rebuilt correctly
   from a run's log. A saved run written before this change still opens: its
   turns simply have no context entries and keep their glued prompt.
6. The generated TypeScript types the web app reads
   (`web/src/lib/skgo/observation/types.ts` and the `TurnRow` type in
   `web/src/lib/observation/index.ts`) include the new field, produced by the
   repository's generators rather than by hand where a generator owns the
   file. JSON schema output (`jsonschema_gen.go`) is regenerated if it covers
   the event.
7. Tests beside the code show: (a) the prompt recorded for a `Generate` call
   in a scope with values is the bare prompt; (b) the context entries name
   the right owner scope when a child scope shadows a parent's key; (c) the
   text sent to the adapter is byte-for-byte what it was before this change;
   (d) the store round-trips the entries through the table file and through
   a rebuild from the log.
8. `just vet` and `just test` pass.

## Not part of this work

- No change to any Svelte component or page. The UI work is happening
  separately on another branch; do not touch `web/src/lib/run/` or the route
  components beyond what a generator rewrites.
- No new API for workflows. `Set`, `SetJSON`, `Generate` and their
  signatures stay as they are.
- No migration of saved runs.
- Do not commit, push, or open a pull request.
