# Issue 325: the detail view

Design proposal, 2026-09-20. Paths are relative to the repository root.

Mock screens for section 5 (pane at 480px, pane maximized, session page,
finished session on Result): https://claude.ai/artifact/XmyNSD827E8ZhzqLaWcHbN

Trims recommended on review, to keep the build small: drop the snap points,
magnet and double-click cycling from the handle (drag, maximize and a
remembered width are the whole feature), and drop the 200 KB payload tier
(one `Show all` threshold is enough).

## 1. Diagnosis

The three reported symptoms are real, and each has a one-line cause. Under
them sit four structural faults that a symptom patch would leave in place.

**The three symptoms.**

- *Assignment fills the body.* `DetailPane.svelte:403-405` renders the
  scope task first, unclamped, with `section p { white-space: pre-wrap }`
  (`:515`). Worse, `scope.task` belongs to the **scope**, not the node: in
  the issue's screenshots `coding`, `task check` and `qa-orchestration` all
  show the identical wall of text, because all three live in `task.1`. The
  block that dominates the first screen is not even about the thing
  selected.
- *Usage clips at the right edge.* `:454` is a `<details>` whose body is
  empty; the entire six-field usage string is crammed into the summary's
  `<span>`, and `:529` gives that span `max-width: 230px; overflow: hidden;
  text-overflow: ellipsis; white-space: nowrap`. `usageText`
  (`web/src/lib/observation/index.ts:418-421`) emits ~70 characters on one
  line. Opening the disclosure reveals nothing. The truncation is the
  design, not a bug.
- *Escaped payloads and body-wide sideways scroll.* `MessageRow.svelte:6`
  defines `text = JSON.stringify(v, null, 2)` and `:45` / `:49` apply it to
  the whole tool input object and to each output content part. Any string
  inside becomes `"…\n\t…"` — the literal `\n` in the screenshots — and the
  `{"type": "text"` wrapper leaks into view (visible in shot 4). The
  sideways drag is separate: `SessionTimeline.svelte:54-55` uses
  `display: grid` with no `min-width: 0`, so a grid item stretches to its
  widest `pre` (`MessageRow.svelte:85` has `pre-wrap` but no
  `overflow-wrap`, so an unbreakable token wins), the section grows past
  the pane, and `.detail-body { overflow: auto }` (`:513`) scrolls the
  whole body — section titles included.

**The four structural faults.**

1. **The outcome is absent, not buried.** The turn branch
   (`DetailPane.svelte:402-472`) never renders `turn.result`, `turn.error`,
   `turn.output_type`, `turn.interrupted` or `turn.duration`
   (`observation/index.ts:86-100`). The only `.result` in the file is a
   *watcher's* (`:436`). A finished session's answer is reachable only by
   reading its transcript to the end. Issue 325 says the result is pushed
   out of view; it was never in view.
2. **Everything is one scroller, ordered worst-first.** Header stack,
   assignment, activity, transcript and five disclosures all live in
   `.detail-body`. The disclosures (`:420-472`) — prompt, watchers,
   session, usage, definition — sit *below* an unbounded transcript, so on
   a live run they retreat as fast as the agent works.
3. **No live affordance.** Nothing pins the transcript to its tail and
   nothing windows it (`SessionTimeline.svelte:31` renders every message).
   A person watching a live coding turn is hand-scrolling a growing
   document. `:29` also prints the raw provider session id as a heading,
   duplicating `DetailPane.svelte:289`.
4. **The pane is a constant.** `.detail { width: 400px; min-width: 340px;
   flex-shrink: 0 }` (`:506`), 340px under 960px (`:555`), and the
   workspace scrolls sideways below 760px (`+page.svelte:381-385`). There
   is no resize, no maximize, no persistence. Selection also lives only in
   page state (`+page.svelte:41`), so a selected node cannot be linked or
   survive a reload — which is why there is nowhere bigger to go.

Two smaller things worth fixing in passing: `MessageRow.svelte:68` emits a
literal backslash-n in a shell row (`<pre>$ {row.command}\n{row.output}</pre>`
is markup text, not a newline); and `scope.decisions`
(`observation/index.ts:57`) and `scope.error` have no home anywhere in the
pane (`DetailPane.svelte:473-482` shows only assignment, values, sessions),
though planner decisions are a named requirement in
`ephemeral/design/run-interface-handoff.md`.

Type sizes under the 13px floor: `MessageRow.svelte:86` (12px),
`SessionTimeline.svelte:63` (12.5px), `DetailPane.svelte:521` and `:542`
(12px).

## 2. Detail view content

**The governing rule.** The first screen answers *what is it doing* or
*how did it end*. It never answers *what was it told*. Context is one
click away, never the lead.

**The instrument: stable tabs.** Borrowed from Temporal's execution detail
and Chrome DevTools. Five plain names, one home per fact, no block
competing with another for height. The tab bar sits under a fixed header
stack; the tab body is the only scroller; the footer is fixed.

### (a) Running agent session

Tabs: **Activity · Result · Prompt · Usage · Source**. Default **Activity**.

| Block | Shows | Default | Rule |
| --- | --- | --- | --- |
| Header (fixed) | icon, session name, kind badge, status pip, `⤢` maximize, `↗ Open` | always | Second line: scope path · session id, mono, single line, truncating, full value in `title`. |
| Status line (fixed) | `running · 2m 24s · claude-sonnet-5:high · 9 model calls · $0.31` | always | One line, wraps to two under 420px. This replaces the model-call list at `DetailPane.svelte:406-411`. |
| Activity | transcript, oldest at top | selected | Owns all remaining height. Pins to bottom while within 48px of it; otherwise stops and shows `↓ Jump to latest (3 new)`. Renders the last 50 messages with `Load earlier (203)` above. |
| Result | `still running`, turn ordinal, start time, last assistant text (3 lines), watcher verdicts | — | Exists while running so the tab set does not change shape at completion. |
| Prompt | `turn.prompt`, verbatim, in one payload block; below it a link `assignment · task.1 →` | — | The *scope's* assignment is not shown here. It lives in the scope's detail. |
| Usage | the six counts as a labeled two-column table, then per-model rows from `Totals.sessions` | — | Never a single line. `usageText` is retired from the pane. |
| Source | role, session name, adapter, model, `file:line`, declared prompt | — | Static definition only. |
| Watchers | — | — | Not a tab. Watcher name, pip and one-line verdict go in **Result**; the full watcher result is a payload block there. |
| Steer (fixed footer) | textarea, help line, `⏹ Stop turn`, `Steer` | always | `Stop turn` targets **this** turn. Today the topbar's stop targets the globally most recent unfinished turn (`+page.svelte:65-69, 171-186`), which is not necessarily the one selected. |

### (b) Finished agent session

Same tabs, default **Result**. Result content in order:

1. Outcome banner — `ended` / `failed` / `interrupted`, duration, finish
   time. Failed and interrupted use the destructive border treatment
   already at `DetailPane.svelte:532`.
2. `turn.result` as a payload block. If `output_type` names a JSON shape,
   pretty-print it; otherwise render it as prose with real newlines.
3. `turn.error` as a payload block, if present.
4. Watchers: one row each — name, pip, verdict line, expandable result.
5. `Activity · 47 messages, 12 tool calls →` as a link to the Activity tab.

Footer becomes the recorded-run note (`DetailPane.svelte:500-502`), not a
steer box.

### (c) Command node

Tabs: **Stdout · Stderr · Details**. Default **Stderr when `exit_code != 0`
or `error` is set, otherwise Stdout** — borrowed from GitHub Actions, which
opens the failing step. Status line: `exit 0 · 3.2s · go test ./...`.
Stdout and Stderr are payload blocks anchored to the *bottom* on open,
because the end is the news; `stdout_file` / `stderr_file` render as a
copyable mono line beneath. Details holds command, args, workdir, times.
A failure banner stays above the tabs, always visible.

### (d) Scope / loop

Tabs: **Overview · Values · Decisions** (Decisions only when `scope.loop`).

- **Overview** — this is the assignment's home: `scope.task` in full, in a
  scrollable payload block, first. Then status, elapsed, `scope.error`,
  the sessions declared here, and for a loop an iteration list (`task 1 of
  3` rows with pip, duration, outcome) where a row selects that instance.
- **Values** — one collapsed row per key, `key · 1.4 KB`, expanding to a
  payload block. Today every value is expanded at once
  (`DetailPane.svelte:478-480`).
- **Decisions** — `scope.decisions` newest-first, each `seq` plus its body
  as a payload block. Currently rendered nowhere.

Footer for a live loop keeps the planner message box and `Wrap up`.

### Tool-call rendering rules

These are exact and are the heart of slice 1.

1. One article per tool part. **Open** while `status` is `streaming`,
   `running` or `error`; **collapsed** once `completed`, unless the person
   opened it by hand (a manual open is sticky for that part).
2. Collapsed summary row: tool name (13px mono, 600), a one-line argument
   digest, status, duration. The digest takes the first present of
   `file_path`, `path`, `command`, `pattern`, `url`, `query`,
   `description`; failing that, the first string-valued key. Truncate from
   the **left** at 60 characters so filenames survive. Never JSON.
3. Input renders as **key/value rows**, one per top-level key. A string
   value renders as text with real newlines. A non-string value renders as
   `JSON.stringify(v, null, 2)`. The object as a whole is never
   stringified — that is what produces `\n`.
4. Output: `state.content` is an array of parts. A `{type: 'text', text}`
   part renders `text` alone. Any other part pretty-prints. The wrapper
   object never appears.
5. Every payload is its own scroller: `max-height: 260px` in the pane,
   `420px` on the page and when maximized; `overflow: auto`;
   `white-space: pre-wrap`; `overflow-wrap: anywhere`; `min-width: 0` on it
   and on every flex/grid ancestor up to `.detail-body`, which itself gets
   `overflow-x: hidden`. **Nothing but a payload ever scrolls sideways, and
   a payload wraps before it scrolls.**
6. Payloads over 4 000 characters render the first 2 000 with
   `Show all (18 KB)`. Over 200 KB, the first 2 000 plus `Copy` only.
7. Each payload carries a strip: label (`input` / `output` / `error`), size,
   `⧉ Copy`, and `⇄` wrap toggle. Wrap is on by default; off gives true
   `pre` with horizontal scroll **inside the block**, for diffs and tables.
8. Nesting: the tool article sits on tint B inside a message article on
   tint A, inset 8px, 1px border clearing 3:1. A subagent transcript nests
   one level further, back on tint A, collapsed with `12 events`.
9. Nothing in a payload is under 13px.

## 3. View control

**Chosen: a right dock that resizes, with a maximize that hides the map.**
Bottom dock is rejected in one sentence: the map is a vertical spine
(`docs/design/README.md`, "steps stack on one centered spine"), and a
bottom dock steals exactly the axis the map needs.

- **Sizes.** Default 480px (up from 400). Drag range 380px to
  `viewport − 360px`, so the map always keeps 360px. Snap points at 480,
  720 and 60% of the viewport, with a 12px magnet.
- **Handle.** A 6px hit target on the pane's left border, 2px visible,
  `col-resize`, highlighting to `--map-line-strong` on hover. Double-click
  cycles the snaps. Keyboard-reachable as a `separator` role with
  `aria-valuenow`; left/right arrows move 24px.
- **Maximize.** `⤢` in the pane header. The pane takes the whole workspace
  body; the map is `display: none`, keeping its pan and zoom. `⤡ Restore`
  returns. **The map never re-lays out on resize** — it loses viewport, not
  layout — which keeps a drag cheap and keeps the spine where the eye left
  it.
- **Wide behavior.** At pane width ≥ 900px the **Result** and **Usage**
  tabs leave the tab bar and become a fixed 320px right rail inside the
  pane; the bar becomes `Activity · Prompt · Source`. One home per fact —
  the home simply moves with the size, it is never in two places.
- **Persistence.** Width and maximized flag in `localStorage` under
  `gimble.detail.width` and `gimble.detail.max`, one setting for the whole
  app, not per run. A stored width outside the current range clamps.
- **Shortcuts.** `\` maximize/restore. `Esc` restores from maximized (and
  only then; it must not clear selection). `o` opens the session page. All
  three are ignored while focus is in a textarea or input. `/` stays
  search (`Topbar.svelte`).
- **1280px.** Map 800px, pane 480px. This is the design target and both
  the wireframes below assume it.
- **Narrow.** Below 1024px the pane stops sharing the row and becomes an
  overlay sheet: full height, slide in from the right, `min(92vw, 560px)`,
  scrim over the map, `Esc` or scrim-click closes it back to the map. The
  `overflow-x: auto` at `+page.svelte:381-385` is deleted — the workspace
  never scrolls sideways.

## 4. Session page

**URL.** `/runs/[runID]/sessions/[sessionID]`, optional `?turn=<turnID>` to
anchor a particular turn. The session id is the stable unit
(`SessionRow.id`) and it is the right one: a session outlives a scope, so
the page can show every turn the session ran across loop iterations —
something the pane, which is turn-scoped, structurally cannot.

**Entry.** `↗ Open` in the pane header, `o`, or double-clicking a graph
node (single click still selects into the pane — Linear's peek-then-full
pattern). **Exit.** Breadcrumb `Runs › implement › task.1 › coding`, a
`✕ Close` that returns to `/runs/[runID]?sel=<key>` with the node still
selected, and browser back. Slice 4 puts selection in the run URL so that
return is exact.

**What it adds over the pane.**

1. **Every turn of the session**, in a left rail — the pane shows only the
   latest (`DetailPane.svelte:189-200`). Each row: ordinal, scope, pip,
   duration, tokens. This is the only place a repeated call's history is
   legible.
2. **The full usage table plus per-model breakdown** from
   `Totals.sessions[id]` — the fact that is truncated to 230px today.
3. **Full-width payloads**, wide enough for diffs and command output
   without wrapping.
4. **Steer and stop targeted at this session**, with room to write more
   than two lines.

**Relation to the pane.** Same leaf components at a different size —
`MessageRow`, the new `ToolCall`, `Payload`, `UsageTable`, `Pip` — and a
different composition above them: three columns instead of one, and the
turn rail, which has no counterpart in the pane.

**Live behavior.** The page opens its own SSE subscription with the same
effect as `+page.svelte:205-254`; the transcript pins and windows exactly
as in the pane; steering uses the existing `steer` remote; stop uses the
existing `stopTurn({run, turn})` with this session's running turn. A
recorded run shows the recorded-run footer and no controls.

**Non-session graph items get no page.** Commands, scopes, loops and
watchers are served by the pane, and maximize is their page: their data
fits a single column, and a command's large output is already a payload
block with a file reference. Adding three more routes would be three more
things to keep in sync for no new fact.

## 5. Wireframes

Scenario throughout: run `K36AYPHH`, workflow `implement`, scope
`implementation.1/task.1`, session `coding`, the Scrabbler drag-and-drop
task. Viewport 1280×800 for the first two, 1440 for the third.

### 5a. Pane at default width (480px), running session

```
├ map, 800px ────────────────────┤├ pane, 480px ──────────────────────────────┤
                                  ┌────────────────────────────────────────────┐
                                  │ ⌷ coding   [agent call]  ⟳   ⤢   ↗ Open    │ 56
                                  │ implementation.1/task.1 · ses_c3ByaW50…LjE │
                                  ├────────────────────────────────────────────┤
                                  │ running · 2m 24s · claude-sonnet-5:high ·  │ 28
                                  │ 9 model calls · $0.31                      │
                                  ├────────────────────────────────────────────┤
                                  │ ▏Activity │ Result │ Prompt │ Usage │ Src  │ 40
                                  ├────────────────────────────────────────────┤
                                  │ ▸ Load earlier (203)                       │
                                  │                                            │
                                  │ ▼ Assistant                      completed │
                                  │   ▸ Reasoning · completed                  │
                                  │   I'll add pointer-based drag handling to  │
                                  │   the rack tiles first, then the board     │
                                  │   squares.                                 │
                                  │   ┌ Read  …/src/lib/game.ts   ok  0.2s ──┐ │
                                  │   └────────────────────────────────────── ┘ │
                                  │   ┌ Edit  …/routes/+page.svelte ok 0.4s ─┐ │
                                  │   │ input        1.2 KB   ⧉ Copy    ⇄   │ │
                                  │   │ old_string                           │ │
                                  │   │   let selected = $state<string |     │ │
                                  │   │     null>(null);                     │ │
                                  │   │ new_string                           │ │
                                  │   │   let selected = $state<string |     │ │
                                  │   │     null>(null);                     │ │
                                  │   │   let dragTile = $state<string |     │ │
                                  │   │     null>(null);                     │ │
                                  │   ├──────────────────────────────────────┤ │
                                  │   │ output         64 B   ⧉ Copy    ⇄   │ │
                                  │   │   web/src/routes/+page.svelte has    │ │
                                  │   │   been updated successfully.         │ │
                                  │   └──────────────────────────────────────┘ │
                                  │   8 in · 673 out · 190 reasoning · $0.31   │
                                  │                                            │
                                  │ ▼ Assistant                        running │
                                  │   ▸ Bash  vitest run --browser  running 12s│
                                  │                                            │
                                  │            ↓ Jump to latest (3 new)        │
                                  ├────────────────────────────────────────────┤
                                  │ ┌────────────────────────────────────────┐ │ 96
                                  │ │ Steer this turn…                       │ │
                                  │ └────────────────────────────────────────┘ │
                                  │ Lands before the next model call.          │
                                  │                       ⏹ Stop turn   Steer  │
                                  └────────────────────────────────────────────┘
```

Regions: header 56px, status 28px, tab bar 40px, activity scroller flexes
(~560px here), footer 96px. The drag handle is the pane's left border.

### 5b. Pane maximized (1280px, map hidden)

```
┌──────────────────────────────────────────────────────────────────────────────┐
│ Runs › implement › K36AYPHH  ⟳ Running  ● Live  2m 24s │ now task 1 │ [find] │ 56
├──────────────────────────────────────────────────────────────────────────────┤
│ ⌷ coding  [agent call]  implementation.1/task.1        ⤡ Restore    ↗ Open   │ 56
│ running · 2m 24s · claude-sonnet-5:high · 9 model calls · $0.31              │ 28
├──────────────────────────────────────────────┬───────────────────────────────┤
│ ▏Activity │ Prompt │ Source                  │ Result                        │ 40
├──────────────────────────────────────────────┤ still running · turn 2 of 2   │
│ ▼ Assistant                        completed │ started 10:36:04              │
│   ▸ Reasoning · completed                    │ last text 10:37:02            │
│   I'll add pointer-based drag handling to    │ "I'll add pointer-based drag  │
│   the rack tiles first, then the squares.    │  handling to the rack tiles…" │
│                                              │                               │
│   ┌ Read  web/src/lib/game.ts     ok  0.2s ┐ │ Watchers                      │
│   └──────────────────────────────────────────┘ │ ● architectural-critique    │
│   ┌ Edit  web/src/routes/+page.svelte ok 0.4s┐│   no objection · 10:36:41    │
│   │ input             1.2 KB    ⧉ Copy   ⇄  ││                               │
│   │ old_string                               ││ Usage · this turn            │
│   │   let selected = $state<string|null>(null);│ input                     8 │
│   │ new_string                               ││ output                  673 │
│   │   let selected = $state<string|null>(null);│ reasoning               190 │
│   │   let dragTile = $state<string|null>(null);│ cache read           48 218 │
│   ├──────────────────────────────────────────┤│ cache write           1 109 │
│   │ output              64 B    ⧉ Copy   ⇄  ││ cost                  $0.31 │
│   │   web/src/routes/+page.svelte has been   ││ ───────────────────────────  │
│   │   updated successfully.                  ││ claude-sonnet-5   673 out    │
│   └──────────────────────────────────────────┘│                              │
│   ▸ Bash  vitest run --browser    running 12s │                              │
│                      ↓ Jump to latest (3 new) │                              │
├──────────────────────────────────────────────┴───────────────────────────────┤
│ ┌──────────────────────────────────────────────────────┐                     │ 96
│ │ Steer this turn…                                     │   ⏹ Stop turn  Steer│
│ └──────────────────────────────────────────────────────┘                     │
└──────────────────────────────────────────────────────────────────────────────┘
      left column ~940px                          right rail 320px
```

Note the tab bar has lost **Result** and **Usage**: at ≥900px they are the
right rail instead.

### 5c. Session page at 1440px

```
┌────────────────────────────────────────────────────────────────────────────────────┐
│ Runs › implement › K36AYPHH › task.1 › coding    ⟳ Running  ● Live  2m 24s  ✕ Close│ 56
├──────────────────┬──────────────────────────────────────────────┬──────────────────┤
│ Turns            │ ▏Activity │ Prompt │ Source                   │ Result           │ 40
│ ──────────────── ├──────────────────────────────────────────────┤ ──────────────── │
│ ▸ turn 1  ended  │ ▼ User                              10:34:58 │ still running    │
│   task.1  1m 12s │   Implement Scrabbler GitHub issue #1 in     │ turn 2 of 2      │
│   4 210 tokens   │   this worktree. Add native HTML5 drag-and-  │ started 10:36:04 │
│                  │   drop so a player can drag a rack tile onto │                  │
│ ● turn 2 running │   an empty board square…            [3 lines]│ Watchers         │
│   task.1  2m 24s │                          ▸ Show all (3 668)  │ ● architectural- │
│   1 980 tokens   │                                              │   critique       │
│                  │ ▼ Assistant                        completed │   no objection   │
│ ──────────────── │   ▸ Reasoning · completed                    │   10:36:41       │
│ Session          │   I'll add pointer-based drag handling to     │                  │
│ coding           │   the rack tiles first, then the squares.    │ Usage · session  │
│ claude-sonnet-5  │                                              │ input          8 │
│   :high          │   ┌ Read  web/src/lib/game.ts     ok 0.2s ─┐ │ output     1 486 │
│ adapter claude   │   └──────────────────────────────────────────┘│ reasoning    412 │
│ ses_c3ByaW50…LjE │   ┌ Edit  web/src/routes/+page.svelte ok 0.4s┐│ cache read96 436 │
│                  │   │ input            1.2 KB   ⧉ Copy    ⇄  ││ cache write2 218 │
│ Source           │   │ old_string                              ││ cost       $0.62 │
│ internal/work-   │   │   let selected = $state<string|null>     ││ ──────────────── │
│ flows/implement/ │   │     (null);                             ││ claude-sonnet-5  │
│ implement.go:142 │   │ new_string                              ││   1 486 out      │
│                  │   │   let selected = $state<string|null>     ││   $0.62          │
│ Assignment       │   │     (null);                             ││                  │
│ task.1 →         │   │   let dragTile = $state<string|null>     ││                  │
│                  │   │     (null);                             ││                  │
│                  │   ├─────────────────────────────────────────┤│                  │
│                  │   │ output             64 B   ⧉ Copy    ⇄  ││                  │
│                  │   │   web/src/routes/+page.svelte has been  ││                  │
│                  │   │   updated successfully.                 ││                  │
│                  │   └─────────────────────────────────────────┘│                  │
│                  │   8 in · 673 out · 190 reasoning · $0.31     │                  │
│                  │   ▸ Bash  vitest run --browser  running 12s  │                  │
│                  │                     ↓ Jump to latest (3 new) │                  │
├──────────────────┴──────────────────────────────────────────────┴──────────────────┤
│ ┌────────────────────────────────────────────────────────────┐                     │ 96
│ │ Steer this turn…                                           │  ⏹ Stop turn  Steer │
│ └────────────────────────────────────────────────────────────┘                     │
└────────────────────────────────────────────────────────────────────────────────────┘
   rail 240px                    centre ~840px                      rail 320px
```

The left rail is the page's whole reason to exist: two turns of one
session, in two different moments of `task.1`, side by side.

## 6. Build slices

**Slice 1 — readable payloads, confined overflow, visible outcome.**
Closes issue 325's literal symptoms.
New: `web/src/lib/run/Payload.svelte`, `ToolCall.svelte`,
`UsageTable.svelte`.
Changed: `MessageRow.svelte` (stop stringifying, key/value input rows,
text-part output, delegate to `ToolCall`/`Payload`, fix the literal `\n` at
`:68`, 13px floor), `SessionTimeline.svelte` (`min-width: 0` on the grid
chain, drop the raw session-id heading, use `UsageTable`),
`DetailPane.svelte` (`overflow-x: hidden` on `.detail-body`, `min-width: 0`
chain, clamp the assignment to three lines with a toggle, add an Outcome
block rendering `turn.result` / `turn.error` / duration above Activity,
replace the empty Usage disclosure with `UsageTable`).
Stories: `MessageRow.stories.svelte`, `DetailPane.stories.svelte`.

**Slice 2 — the tabbed anatomy.**
New: `web/src/lib/run/detail/SessionDetail.svelte`, `CommandDetail.svelte`,
`ScopeDetail.svelte`, `Outcome.svelte`, `TabBar.svelte`.
Changed: `DetailPane.svelte` becomes a header + footer shell that picks one
of the three; `SessionTimeline.svelte` gains pin-to-bottom, the jump pill
and the 50-message window; `ScopeDetail` gives `scope.decisions` and
`scope.error` their first home; the footer's stop targets the selected
turn. Tests: `DetailPane.svelte.test.ts`.

**Slice 3 — view control.**
New: `web/src/lib/run/Splitter.svelte`, `web/src/lib/run/paneSize.ts`.
Changed: `web/src/routes/runs/[runID]/+page.svelte` (splitter, maximize,
shortcut handling, overlay sheet under 1024px, delete the 760px
`overflow-x`), `DetailPane.svelte` (width becomes a prop; the ≥900px rail
split), `Map.svelte` (no re-layout on resize; hidden, not unmounted, when
maximized).

**Slice 4 — selection in the URL.**
Changed: `web/src/lib/run/selection.ts` (`selectionKey` ⇄ `selectionFromKey`
on top of the existing `selectedRuntimeKey` at `:230-244`),
`+page.svelte` (read and write `?sel=`, `replaceState` on selection).
Independently useful: a selected node becomes linkable and survives reload.

**Slice 5 — the session page.**
New: `web/src/routes/runs/[runID]/sessions/[sessionID]/+page.svelte`,
`page.server.go` and its generated `+page.server.ts`,
`web/src/lib/run/detail/SessionPage.svelte`, `TurnRail.svelte`.
Changed: `DetailPane.svelte` (the `↗ Open` link), `Map.svelte`
(double-click opens), `e2e/features/` for the new route.

## 7. Not doing

- **Bottom dock** — steals the axis the map's vertical spine needs.
- **Pages for commands, scopes, loops and watchers** — maximize is their
  page; three more routes buy no new fact.
- **Virtualized transcript** — a 50-message window with `Load earlier`
  solves the real sizes without a windowing library.
- **Time scrubber / replay** — the handoff explicitly excludes it.
- **Syntax highlighting and diff rendering in payloads** — real newlines,
  wrapping and a wrap toggle are the whole complaint; highlighting is
  polish.
- **Floating or detachable inspector windows** — one dock, one page.
- **Two simultaneous detail panes / pinned comparisons** — the session
  page's turn rail covers the one real comparison (a repeated call).
- **Per-run pane width** — one app-wide setting; nobody wants to re-size
  per run.
- **Transcript search** — worth doing, but after the tabs exist and not in
  the same breath as fixing 325.
- **Raw-JSON toggle for a whole message** — `⧉ Copy` on each payload
  covers the real need, which is pasting into an editor.
