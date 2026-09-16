# The web app

Every Gimble runtime serves one web application: the project's runs, live
and past, drawn as the graph of agents that did the work, with a person at
the page able to steer any of them. This file is what the design team and
the coding agents share: the UI decisions that are made, the features, the
user stories by name, and what is deliberately not on the page.

The app is the SvelteKit app in `web/`. Go answers its loads, remote
functions and server routes; see `README.md` for how that works.

## UI decisions

Only what Tyler has decided. Add a row when a decision is made; do not
add one for a preference.

| Decision | Choice | Why |
| --- | --- | --- |
| Component kit | shadcn-svelte, copied into `web/src/lib/components/ui/` | The design team and the coding agents read the same primitives. |
| Import alias | `#lib/...`, the Node subpath import in `web/package.json`. SvelteKit 3 removed `$lib`. | One alias in the app. `web/tsconfig.json` repeats it under `paths` only because the shadcn CLI checks for it there. |
| Where decisions live | This file, on `main` | The design team reads the instructions beside the primitives. |
| Data path | SSR snapshot first, then a Server-Sent Events stream; a past run is reduced from its logs (#169) | Already how the run page works. A constraint on the design, not a choice for it. |

### The install, and how to change it

The kit was installed with the CLI's own default look so that something
exists to design against. None of it is a decision.

- Preset `bIkeymG`: style `vega`, base colour `neutral`, theme `neutral`,
  icons Lucide, font Inter, default radius. A preset code is the CLI's
  bit-packed encoding of those fields; `encodePreset` in
  `shadcn-svelte/preset` makes one.
- Installed components: button, badge, card, separator, tabs, input,
  textarea, tooltip, scroll-area, table, dialog, skeleton.
- `web/src/app.css` holds the token layer; `web/src/routes/+layout.svelte`
  imports it. Tailwind v4 through `@tailwindcss/vite`.

The CLI runs non-interactively from `web/`:

```sh
# change the look: re-run init with another preset code
vp dlx shadcn-svelte@latest init --reinstall --preset <code> --base-color neutral \
  --css src/app.css --lib-alias '#lib' --components-alias '#lib/components' \
  --utils-alias '#lib/utils' --hooks-alias '#lib/hooks' --ui-alias '#lib/components/ui'

# add components
vp dlx shadcn-svelte@latest add -y <name>...
```

`--reinstall` is what makes `init` overwrite `app.css` without a prompt.
`add -y` skips its confirmation; `-o` also overwrites existing components.

### Not yet decided

- Style, base colour, theme, icon set, typefaces, radius.
- Light and dark, or one theme.
- Density. A run has hundreds of turns; the page is open for hours beside
  a terminal.
- URL shape for a scope, session, or turn (`rev.link-to-node`).
- Whether status colours (running, ended, failed, cancelled, dropped) stay
  separate from an accent that marks the person's own steers.
- Graph rendering: a node-and-edge drawing, or a timeline tree of rows on
  the wall clock indented by scope key. The second is cheaper to build and
  to read at hundreds of turns.
- Whether a phone layout is in the first pass.

## The people at the page

One person, wearing three hats at different moments.

- **Operator, watching.** Started a workflow or found one running. What
  is happening now, what is stuck, what it is costing.
- **Supervisor, steering.** Reads a transcript mid-turn and objects. Needs
  to know whether a steer will land.
- **Reviewer, reading afterwards.** What the planner decided, what a coder
  was told, what a validator objected to, where the money went.

## Features

The spine of every view is the scope key, `lap.3/bakeoff.1/attempt.2`.
Containment is a prefix; siblings whose intervals overlap ran
concurrently. It is the breadcrumb, the URL, the group-by for usage, and
the join with the source. See Observability in
`ephemeral/research/api/API.md`.

| | Feature | Today on `main` |
| --- | --- | --- |
| F1 | **Runs list.** Every run under `.gimble/runs/`, live and past, pushed live. Name, status, started, duration, total cost. Filter by status and workflow; search by id. | bare |
| F2 | **Start a workflow.** One form per workflow, controls named by its Go input struct. The first is the sprint workflow. | designed, not built |
| F3 | **The run graph.** Scopes as a tree from key prefixes on the run's wall clock. Instances of one node stack under one card. Overlapping siblings side by side. Sessions under the scope that created them; a turn where it ran. Two zoom levels. Connection state always visible. | flat list |
| F4 | **Scope detail.** Name, key, began, ended, how it ended, its task, its values as `Generate` hands them to the agent, each planner decision. | bare |
| F5 | **Session card.** Name, adapter, model, workdir, parent if forked. Turns in order, running usage, whether a turn is running now. Home of the steer box. | bare |
| F6 | **Turn detail.** Prompt as sent, output type and schema, result or error, interrupted, duration, usage per model with cost. A validation failure and its re-ask. | partly |
| F7 | **Transcript.** Messages coalesced by id: user, assistant, thinking, tool call and result, deltas, per-step usage, harness errors, approval requests, nested transcripts. Model and five token cells per row, no dollars (#152). | exists |
| F8 | **Loop backlog and decisions.** The backlog as the planner left it after each dispatch; scrub through decisions. | designed, not built |
| F9 | **Steer, interrupt, cancel.** Steer box on every session; interrupt on a running turn; cancel on the run. The person is a supervisor node; their steers are attributed and marked landed or dropped. | designed, not built |
| F10 | **Edges.** Supervisor to worker turn with instruction and interval; fork to parent; steer from source to target, landed or dropped. | designed, not built |
| F11 | **Usage, time, cost by scope.** Every turn charged to the scope it ran in; sums per model with wall time and priced cost; one tree, live and finished. | in progress, #173 |
| F12 | **Prompts before they run.** Every `Generate` call's prompt and schema as values, from a dry run or a captured cheap-tier run. | asked for, #168 |
| F13 | **The template.** The static pass draws the graph before it runs; a live run lights it up. "Declared, not yet run" is a node state. | later |

## User stories

Names are `hat.verb-object`. The design, the issues, and the browser tests
cite them.

### Operator, watching

| | |
| --- | --- |
| `op.find-live` | See at once which runs are live, finished, or failed, without reading a table. |
| `op.open-running` | Open a live run and land on what is happening now. |
| `op.read-shape` | Tell from the graph alone whether a scope is a loop, a bake-off, or a sequence. |
| `op.spot-stuck` | See a turn running longer than its siblings, or a session with no turn, without opening it. |
| `op.know-cost` | See the run's cost so far and which scope is spending it, live. |
| `op.trust-live` | Know whether the page is current, replaying, or has lost the stream. |
| `op.start-sprint` | Fill in the sprint form, start it, arrive on its run page. |
| `op.see-error` | Read a failure where it happened and see which turn caused it. |

### Supervisor, steering

| | |
| --- | --- |
| `sup.read-mid-turn` | Follow a transcript as it streams, tool output collapsed until wanted. |
| `sup.will-it-land` | Before typing a steer, see whether a turn is running so it will land. |
| `sup.steer` | Send a steer and see it in the transcript, landed or dropped, attributed to me. |
| `sup.interrupt` | Interrupt one turn; the run continues. |
| `sup.cancel-run` | Cancel a run, confirm once, watch every turn end and every scope close. |
| `sup.see-other-supervisors` | See which reviewers watch a turn, what they were told, what they objected to. |
| `sup.see-what-agent-saw` | Read a scope's values exactly as `Generate` handed them to the agent. |

### Reviewer, reading afterwards

| | |
| --- | --- |
| `rev.replay` | Open a finished run and see what a live watcher saw, from the log. |
| `rev.follow-planner` | Step through planner decisions with the backlog and chosen task at each. |
| `rev.read-prompt` | Open any turn and read the prompt, schema, and result as sent and returned. |
| `rev.cost-tree` | Usage, wall time, and cost by scope, per model, run down to turn. |
| `rev.trace-fork` | Follow a fork to its parent and what the parent knew then. |
| `rev.trace-steers` | Every steer, who sent it, whether it landed, what the worker did next. |
| `rev.compare-instances` | Two instances of one node side by side. |
| `rev.preview-prompts` | Before a run, read every prompt and schema the workflow will build. |
| `rev.link-to-node` | Copy a URL to one scope, session, or turn by key and have it open there. |

Proposed first pass: all of Operator; `sup.read-mid-turn`,
`sup.will-it-land`, `sup.steer`; `rev.replay`, `rev.read-prompt`,
`rev.cost-tree`. Edges (F10) and backlog scrubbing (F8) second.

## Deliberately not on the page

- Accounts, login, roles. The listener is loopback only.
- Per-message dollars (#152). Cost is real at the turn and above.
- Cancelling one scope. Runs are cancelled and turns interrupted, nothing
  between (API.md).
- Context-window meters, quota windows, MCP status, raw payload retention
  (declined in #173).
- Editing the workflow. The workflow is Go.
- Multi-project. One runtime, one project directory, one page.

## Vocabulary

The page uses the names in the Go API and the event log, unchanged: run,
scope, group, loop, lap, task, planner, decision, backlog, session, turn,
supervisor, steer (landed or dropped), interrupt, cancel, value, adapter,
model.
