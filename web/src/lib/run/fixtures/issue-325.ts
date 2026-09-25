// Fixture reproducing GitHub issue 325: the detail pane rendering real,
// messy run data for a "Scrabbler drag-and-drop" implementation task — a
// very long assignment, escaped-newline-laden tool payloads, a glued
// call-site + context prompt, and a running turn with no visible result yet.
//
// Its coding turns record their context the way the runtime does: `prompt` is
// what the workflow wrote, and `context` names each visible value once,
// outermost scope first, the nearest scope owning a shadowed key.
// `issue325GluedPrompt` is what the same turn's `prompt` held in a run saved
// before turns recorded their context.
import type {
  CommandRow,
  ContextEntry,
  ModelCallRow,
  RunSnapshot,
  ScopeRow,
  SessionRow,
  TurnRow,
  Usage,
} from "../../observation/index.js";
import type { Snapshot } from "../../sessionstate/index.js";
import type { Graph } from "../../workflow/types.js";
import { implementInterviewGraph } from "./implement-interview.js";

// The implement workflow's graph without its research group, which this run
// does not have, under this run's own name so the page accepts the pair.
export const issue325Graph: Graph = {
  ...implementInterviewGraph,
  name: "implement",
  body: implementInterviewGraph.body.filter(
    (operation) => !(operation.kind === "group" && operation.name === "reconnaissance"),
  ),
};

const runID = "01M2ZTP8DFJQAYF7QSK36AYPHH";
// Fixed once at module load, not a literal calendar date: the running turn's
// elapsed time (now - turn.started) must stay positive AND plausible (a
// coding turn a few minutes old, not one apparently still running 600 days
// later) for as long as this fixture is read, which a hardcoded past date
// cannot promise.
const started = Date.now() - 20 * 60 * 1000;

// ---------------------------------------------------------------------------
// Scope values: a long assignment, a long definition of done, repository
// notes (artifact-backed), and a shadowing per-task definition of done.
// ---------------------------------------------------------------------------

const longAssignment =
  "Implement GitHub issue #1: replace Scrabbler's click-to-place tile placement with native " +
  "HTML5 drag-and-drop while keeping click-to-place working side by side. The rack and the " +
  "board both live in web/src/routes/+page.svelte today, along with the selection state, the " +
  "scoring pass, and the turn-submission logic; do not split that file apart as part of this " +
  'task even though it is large. Each rack tile needs draggable="true" and needs to carry its ' +
  "own letter and rack index through the drag, most naturally via " +
  'event.dataTransfer.setData("text/plain", JSON.stringify({letter, rackIndex})) on dragstart, ' +
  "since Svelte 5's runes state cannot travel through the browser's native drag payload. Each " +
  "board square needs ondragover with preventDefault() so the drop event actually fires, a " +
  "visual hover affordance while a tile is dragged over an empty square (reuse the existing " +
  "selected-square styling token rather than inventing a new one), and an ondrop handler that " +
  "parses the transferred JSON, validates the square is empty and in-bounds, and calls the same " +
  "placeTile(letter, row, col) function the click path already calls — there must be exactly one " +
  "placement code path, not two that can drift apart. If the drop target is invalid (occupied " +
  "square, off the board, or a stale drag from a tile that has since been played by a click), " +
  "the tile must visually snap back to its rack position; do not leave a ghost tile or a gap in " +
  'the rack. Use ondragend on the source tile to clear any "dragging" class you add, and make ' +
  "sure that class does not persist if the drop never fires at all (e.g. the user drags off the " +
  "window). Keep the existing keyboard and click accessibility paths completely intact; " +
  "drag-and-drop is additive, not a replacement, since not every input device supports it. Add " +
  "vitest browser-mode tests (this repo already runs vitest in browser mode for the rack and " +
  "board components — follow the existing pattern in the neighboring *.test.ts files rather than " +
  "introducing jsdom-simulated drag events, which do not exercise the real dataTransfer object) " +
  "covering: a full drag from rack to an empty square placing the tile, a drag to an occupied " +
  "square snapping back, a drag off the board snapping back, and a drag interrupted before drop " +
  "leaving the rack tile visible and undragged-looking. Do not touch the scoring logic, the turn " +
  "submission remote function, or anything under web/src/lib/server/. Do not add a drag-and-drop " +
  "library dependency; the native API is sufficient for this interaction and this repo avoids " +
  "extra runtime dependencies for things the platform already does. When you are done, report " +
  "exactly what you changed and what you personally ran to observe it working, including the " +
  "vitest browser run output — do not report success from reading your own diff. The reference " +
  "directory under the run's working tree has a frontend/index.md with deep notes on the " +
  "installed Svelte, SvelteKit, shadcn-svelte, and Bits UI APIs available in this repository; " +
  "read it before assuming a helper exists or does not. If you find yourself wanting a pointer " +
  "events polyfill, a virtual-DOM drag library, or any other dependency to make this easier, " +
  "stop and use the native HTML5 drag-and-drop events directly instead — dragstart, dragover, " +
  "drop, and dragend are enough for a single rack-to-board interaction and this repository does " +
  "not accept new runtime dependencies for problems the platform already solves. Remember also " +
  "that this task runs inside a promise loop with an independent QA pass after it: the " +
  "`qa-orchestration` session will drive a real browser against the generated example and will " +
  "not take your word for any of this, so leave the interface in a state where dragging a tile " +
  "actually works when someone else drives it by hand, not merely where the unit tests are green.";

const definitionOfDoneLoop =
  "## Definition of done — implementation loop\n\n" +
  "This definition applies to every task the planner selects inside the `implementation` loop, " +
  "not only the current one. A task is not done because the worker says so; it is done when " +
  "every item below is independently true.\n\n" +
  "1. **Builds clean.** `just build` exits 0 with no new warnings introduced by this task's " +
  "diff. Pre-existing warnings in files this task did not touch are not this task's problem.\n" +
  "2. **Vets clean.** `just vet` exits 0. A `//nolint` comment added to silence a real finding " +
  "is not clean; it is a finding wearing a costume.\n" +
  "3. **Task check passes.** The task's own `Validation.Command`, when one is declared, exits 0. " +
  "For frontend tasks this is almost always a vitest invocation; read the actual command instead " +
  "of assuming it matches the last task's command, because they frequently differ.\n" +
  "4. **No scope creep.** The diff touches only what the assignment named. A worker who notices " +
  "an unrelated bug files it; a worker who fixes it anyway has expanded scope without asking, " +
  "and the architectural-critique watcher exists specifically to catch that.\n" +
  "5. **No proof programs.** Evidence is the worker's own account of what they ran and observed " +
  'in this turn, not a script committed to the tree that prints "PASS" and exits. Nothing under ' +
  "an ephemeral or scratch directory is proof of anything; it is notes.\n" +
  "6. **Independent QA agrees.** The `qa-orchestration` session reads the issue, the reference " +
  "directory, the diff, and the recorded command output independently, and personally exercises " +
  "the running interface — for a UI task that means driving a real browser through Playwright " +
  "against the actual generated example, not reading the worker's report and nodding along. Its " +
  "`Assessment.Complete` field, not the worker's optimism, is what ends the loop.\n" +
  "7. **The rack and board stay one code path.** Any task that touches placement must leave " +
  "exactly one function that actually places a tile on the board, reachable from every input " +
  "method. Two placement code paths is itself a defect even if both currently work.\n\n" +
  "A task that fails any item above stays open; the planner selects it again next round with the " +
  "accumulated findings folded into the next assignment, up to the loop's configured task limit.";

const repositoryNotesFull =
  "# Repository notes — Scrabbler frontend\n\n" +
  "`web/` is a SvelteKit app on Svelte 5 runes, TypeScript, and pnpm. The game screen is " +
  "`web/src/routes/+page.svelte`; it currently owns rack state, board state, the active " +
  "selection, scoring, and the submit-turn call to the server remote function in one file — " +
  "large, but intentionally not yet split (see the svelte-component-factoring guidance before " +
  "proposing a split as part of an unrelated task). Tiles are rendered from a `$state` rack " +
  "array; the board is a `$state` 15x15 grid of square objects with `letter`, `bonus`, and " +
  "`locked` fields. Click-to-place currently works by setting a `selectedTile` rune on rack-tile " +
  "click and consuming it on board-square click via `placeTile`. There is no drag affordance " +
  "today; issue #1 is the first drag-and-drop work in this codebase, so there is no existing " +
  "convention to match beyond the general native-DOM-first bias the frontend research notes " +
  "describe. Styling uses plain CSS in each component's `<style>` block, scoped by Svelte by " +
  "default; the selected-square hover token is `--square-selected-bg` and is already themed for " +
  "dark mode. Vitest is configured in browser mode via `@vitest/browser` with Playwright's " +
  "Chromium provider; existing tests for the rack and board live beside their components as " +
  "`*.test.ts` and drive real pointer and keyboard events through `@testing-library/svelte` — " +
  "follow that pattern for drag events too rather than hand-rolling a `DataTransfer` polyfill, " +
  "since the real browser already provides one in browser-mode tests.";

const taskAssignmentValue =
  "Task 1 of this loop pass: add native HTML5 drag-and-drop for rack tiles onto board squares " +
  "in web/src/routes/+page.svelte, landing on the same placeTile() call click-to-place already " +
  "uses, with invalid drops snapping the tile back to the rack and vitest browser tests covering " +
  "the drop, snap-back, off-board, and interrupted-drag cases. See the full assignment above for " +
  "the complete constraints; this is the short form the planner recorded when it selected the " +
  "task.";

const taskDefinitionOfDone =
  "## Definition of done — this task\n\n" +
  "Shadows the loop-level definition above with what applies specifically to drag-and-drop:\n\n" +
  "- A rack tile is draggable and a board square accepts the drop, both routed through the " +
  "existing `placeTile(letter, row, col)` function — no second placement path.\n" +
  "- Click-to-place still places tiles exactly as it did before this task started.\n" +
  "- Dropping on an occupied square, off the board, or with a stale payload snaps the tile back " +
  "to its rack slot with no ghost tile and no gap.\n" +
  "- `ondragend` always clears the dragging affordance, including when no drop ever fires.\n" +
  "- New vitest browser tests exercise: successful drop, occupied-square snap-back, off-board " +
  "snap-back, interrupted drag.\n" +
  "- `just build`, `just vet`, and the task's own vitest check all exit 0.\n" +
  "- QA has personally driven the running interface with Playwright and confirms every item " +
  "above, not just read the diff.";

const repositoryNotesArtifact = {
  file: "artifacts/implementation.1/repository_notes.md",
  size: repositoryNotesFull.length,
  format: "text" as const,
  preview: repositoryNotesFull.slice(0, 320) + " …",
};

// ---------------------------------------------------------------------------
// Scopes
// ---------------------------------------------------------------------------

const scopes = {
  "": {
    run: runID,
    key: "",
    name: "implement",
    loop: false,
    status: "running",
    error: "",
    began: started,
    ended: 0,
    values: {},
    decisions: [],
  },
  "implementation.1": {
    run: runID,
    key: "implementation.1",
    name: "implementation",
    loop: true,
    status: "running",
    error: "",
    began: started + 40_000,
    ended: 0,
    values: {
      definition_of_done: { value: definitionOfDoneLoop },
      repository_notes: { artifact: repositoryNotesArtifact },
    },
    decisions: [
      { seq: 1, body: { task: 1, name: "Scrabbler drag-and-drop", note: "" } },
      {
        seq: 2,
        body: {
          task: 1,
          name: "Scrabbler drag-and-drop",
          note: "Reselected after task check failed on the previous pass.",
        },
      },
    ],
  },
  "implementation.1/task.1": {
    run: runID,
    key: "implementation.1/task.1",
    name: "task",
    loop: false,
    status: "running",
    error: "",
    task: { name: "Scrabbler drag-and-drop", description: longAssignment },
    began: started + 120_000,
    ended: 0,
    values: {
      assignment: { value: taskAssignmentValue },
      definition_of_done: { value: taskDefinitionOfDone },
    },
    decisions: [],
  },
} satisfies Record<string, ScopeRow>;

// ---------------------------------------------------------------------------
// Sessions
// ---------------------------------------------------------------------------

const sessions = {
  "sprint-planning.1": {
    run: runID,
    id: "sprint-planning.1",
    name: "sprint-planning",
    adapter: "claude",
    model: "claude-sonnet-5",
    scope: "",
    parent: "",
    created: started + 40_000,
  },
  "coding.1": {
    run: runID,
    id: "coding.1",
    name: "coding",
    adapter: "claude",
    model: "claude-sonnet-5",
    scope: "",
    parent: "",
    created: started + 120_000,
  },
  "architectural-critique.1": {
    run: runID,
    id: "architectural-critique.1",
    name: "architectural-critique",
    adapter: "claude",
    model: "claude-sonnet-5",
    scope: "",
    parent: "",
    created: started + 120_000,
  },
  "implementation.1/task.1/qa-orchestration.1": {
    run: runID,
    id: "implementation.1/task.1/qa-orchestration.1",
    name: "qa-orchestration",
    adapter: "claude",
    model: "claude-sonnet-5",
    scope: "implementation.1/task.1",
    parent: "",
    created: started + 130_000,
  },
} satisfies Record<string, SessionRow>;

// ---------------------------------------------------------------------------
// Turns: the folded call-site + context wall of text, reproducing issue 325.
// ---------------------------------------------------------------------------

const bareCodingPrompt =
  "Read the saved issue and the local reference directory, then implement only the selected " +
  "task in the repository. Do not expand the specification, modify the requirements, commit, " +
  "merge, or add proof scripts or run output. Answer with what changed and what you personally " +
  "ran or observed.";

const renderSection = (key: string, value: string) => `## ${key}\n\n${value}`;

const promptFor = (sections: [string, string][]) =>
  bareCodingPrompt +
  "\n\n" +
  sections.map(([key, value]) => renderSection(key, value)).join("\n\n");

const sentSections: [string, string][] = [
  ["repository_notes", repositoryNotesArtifact.preview],
  ["assignment", taskAssignmentValue],
  ["definition_of_done", taskDefinitionOfDone],
];

export const issue325GluedPrompt = promptFor(sentSections);

const turn1Context: ContextEntry[] = [
  { key: "repository_notes", scope: "implementation.1", complete: false },
  { key: "assignment", scope: "implementation.1/task.1", complete: true },
  { key: "definition_of_done", scope: "implementation.1/task.1", complete: true },
];

const turn1EndedResult =
  'Added draggable="true" and dragstart/dragend handlers to the rack tile component, and ' +
  "ondragover/ondrop handlers to the board square component, both routed through the existing " +
  "placeTile() function so click-to-place and drag-and-drop share one placement path.\n\n" +
  "Invalid drops (occupied square, off-board, stale rack index) now snap the tile back by " +
  "simply not calling placeTile and letting the rack's own state re-render the tile in place; " +
  "the dragging class is cleared unconditionally in ondragend so it never persists.\n\n" +
  "Added four vitest browser tests in +page.test.ts covering a successful drop, an occupied-" +
  "square snap-back, an off-board snap-back, and an interrupted drag. Ran " +
  "`pnpm exec vitest run src/routes/+page.test.ts` locally and watched all four new cases plus " +
  "the pre-existing click-to-place cases pass in a real Chromium instance.\n\n" +
  "Did not touch scoring, turn submission, or anything under lib/server/.";

const turns = {
  "coding.1/turn.1": {
    run: runID,
    id: "coding.1/turn.1",
    session: "coding.1",
    scope: "implementation.1/task.1",
    prompt: bareCodingPrompt,
    output_type: "text",
    result: turn1EndedResult,
    error: "",
    interrupted: false,
    started: started + 120_000,
    ended: started + 890_000,
    duration: 770_000,
    context: turn1Context,
  },
  "coding.1/turn.2": {
    run: runID,
    id: "coding.1/turn.2",
    session: "coding.1",
    scope: "implementation.1/task.1",
    prompt: bareCodingPrompt,
    output_type: "text",
    result: "",
    error: "",
    interrupted: false,
    started: started + 940_000,
    ended: 0,
    duration: 0,
    context: turn1Context,
  },
  "sprint-planning.1/turn.1": {
    run: runID,
    id: "sprint-planning.1/turn.1",
    session: "sprint-planning.1",
    scope: "implementation.1",
    prompt: "Choose the next task from the remaining milestone work.",
    output_type: "Task",
    result:
      '{"task":1,"name":"Scrabbler drag-and-drop","rationale":"Highest-value remaining item; ' +
      'no dependency on the scoring rework."}',
    error: "",
    interrupted: false,
    started: started + 20_000,
    ended: started + 38_000,
    duration: 18_000,
  },
  "architectural-critique.1/turn.1": {
    run: runID,
    id: "architectural-critique.1/turn.1",
    session: "architectural-critique.1",
    scope: "implementation.1/task.1",
    prompt: "Review task 1 for frontend or general implementation scope expansion.",
    output_type: "text",
    result:
      "No scope expansion observed. Drag-and-drop stays inside +page.svelte and routes through " +
      "the existing placeTile() call; no new dependency was added.",
    error: "",
    interrupted: false,
    started: started + 900_000,
    ended: started + 918_000,
    duration: 18_000,
  },
} satisfies Record<string, TurnRow>;

// ---------------------------------------------------------------------------
// Command: task check FAILED on the first pass.
// ---------------------------------------------------------------------------

const failingStdout =
  "> gimbal-web@0.0.1 test:task /workspace/gimbal/web\n" +
  "> vitest run src/routes/+page.test.ts\n\n" +
  " RUN  v2.1.4 /workspace/gimbal/web\n\n" +
  " ✓ src/routes/+page.test.ts (9)\n" +
  "   ✓ rack renders seven tiles\n" +
  "   ✓ click-to-place fills the selected square\n" +
  "   ✓ click-to-place refuses an occupied square\n" +
  "   ✓ drag from rack to empty square places the tile\n" +
  "   ✗ drag to an occupied square snaps back\n" +
  "   ✗ drag off the board snaps back\n" +
  "   ✓ interrupted drag clears the dragging class\n" +
  "   ✓ scoring is unchanged by drag placement\n" +
  "   ✓ submit-turn call is unaffected\n\n" +
  " Test Files  1 failed (1)\n" +
  "      Tests  2 failed | 7 passed (9)\n" +
  "   Start at  14:02:11\n" +
  "   Duration  4.21s\n";

const failingStderr =
  "FAIL  src/routes/+page.test.ts > drag to an occupied square snaps back\n" +
  'AssertionError: expected square [3,7] to still read "Q" but got ""\n' +
  "  ❯ src/routes/+page.test.ts:142:18\n" +
  "      140|   await fireDrop(occupiedSquare, dragPayload);\n" +
  "      141|   await tick();\n" +
  '      142|   expect(occupiedSquare.textContent).toBe("Q");\n' +
  "         |                                       ^\n" +
  "      143| });\n\n" +
  "FAIL  src/routes/+page.test.ts > drag off the board snaps back\n" +
  'AssertionError: expected rack to contain tile "R" at index 3 but rack[3] was undefined\n' +
  "  ❯ src/routes/+page.test.ts:168:24\n" +
  "      166|   await fireDropOutsideBoard(rackTile);\n" +
  "      167|   await tick();\n" +
  '      168|   expect(rack[3]?.letter).toBe("R");\n' +
  "         |                          ^\n" +
  "      169| });\n\n" +
  "The ondrop handler calls placeTile() before checking square.locked, so a drop on an occupied " +
  "square overwrites it instead of snapping back; the same handler never runs for a drop outside " +
  "any square, so the rack entry stays removed from dragstart's optimistic update.\n";

const commands = {
  "implementation.1/task.1/task-check.1": {
    run: runID,
    id: "implementation.1/task.1/task-check.1",
    scope: "implementation.1/task.1",
    name: "task-check",
    command: "sh",
    args: ["-lc", "pnpm exec vitest run src/routes/+page.test.ts"],
    workdir: "/workspace/gimbal/web",
    exit_code: 1,
    stdout: failingStdout,
    stderr: failingStderr,
    stdout_file: "commands/implementation.1/task.1/task-check.1.stdout",
    stderr_file: "commands/implementation.1/task.1/task-check.1.stderr",
    error: "",
    interrupted: false,
    started: started + 891_000,
    ended: started + 895_210,
    duration: 4_210,
  },
} satisfies Record<string, CommandRow>;

// ---------------------------------------------------------------------------
// Usage
// ---------------------------------------------------------------------------

const codingUsage: Usage = {
  input: 84_200,
  cache_read: 61_500,
  cache_write: 9_800,
  output: 6_400,
  reasoning: 2_100,
  stated_cost: 1.42,
};

const modelCalls = {
  "coding.1/turn.2": [
    {
      run: runID,
      turn: "coding.1/turn.2",
      message: "msg_coding2_01",
      model: "claude-sonnet-5",
      input: 28_000,
      cache_read: 20_500,
      cache_write: 9_800,
      output: 1_900,
      reasoning: 800,
      started: started + 940_000,
      ended: started + 962_000,
    },
    {
      run: runID,
      turn: "coding.1/turn.2",
      message: "msg_coding2_06",
      model: "claude-sonnet-5",
      input: 31_400,
      cache_read: 25_100,
      cache_write: 0,
      output: 2_600,
      reasoning: 900,
      started: started + 1_010_000,
      ended: started + 1_048_000,
    },
    {
      run: runID,
      turn: "coding.1/turn.2",
      message: "msg_coding2_12",
      model: "claude-sonnet-5",
      input: 24_800,
      cache_read: 15_900,
      cache_write: 0,
      output: 1_900,
      reasoning: 400,
      started: started + 1_090_000,
      ended: 0,
    },
  ],
} satisfies Record<string, ModelCallRow[]>;

// ---------------------------------------------------------------------------
// Transcript for coding.1/turn.1 (ended) — short, for contrast.
// ---------------------------------------------------------------------------

const nativeSession1 = "ses_coding_turn1";

const transcript1: Snapshot = {
  state: {
    info: {
      [nativeSession1]: {
        agent: "coding",
        model: { id: "claude-sonnet-5", providerID: "anthropic" },
      },
    },
    family: {},
    active: { [nativeSession1]: "idle" },
    message: {
      [nativeSession1]: [
        {
          id: "msg_coding1_01",
          type: "assistant",
          agent: "coding",
          model: { id: "claude-sonnet-5", providerID: "anthropic" },
          content: [
            {
              type: "reasoning",
              text:
                "The rack tile and board square both live in +page.svelte. I'll add draggable " +
                "attributes and dataTransfer plumbing, then route both the click and drop paths " +
                "through the existing placeTile() call so there is exactly one placement path.",
              time: { created: started + 121_000, completed: started + 121_800 },
            },
            {
              type: "text",
              text:
                "Adding native drag-and-drop to the rack and board, keeping click-to-place " +
                "working and routing both through placeTile().",
            },
            {
              type: "tool",
              id: "call_edit_page_1",
              name: "Edit",
              time: {
                created: started + 122_000,
                ran: started + 122_100,
                completed: started + 122_600,
              },
              state: {
                status: "completed",
                input: {
                  file_path: "web/src/routes/+page.svelte",
                  old_string:
                    '<button class="tile" onclick={() => selectTile(index)}>{tile.letter}</button>',
                  new_string:
                    '<button\n\tclass="tile"\n\tdraggable="true"\n\tonclick={() => selectTile(index)}\n\tondragstart={(event) => {\n\t\tevent.dataTransfer?.setData(\n\t\t\t\'text/plain\',\n\t\t\tJSON.stringify({ letter: tile.letter, rackIndex: index })\n\t\t);\n\t\tdragging = index;\n\t}}\n\tondragend={() => {\n\t\tdragging = undefined;\n\t}}\n>{tile.letter}</button>',
                },
                content: [{ type: "text", text: "Applied 1 edit to web/src/routes/+page.svelte" }],
              },
              tokens: { input: 4_200, output: 310, reasoning: 0, cache: { read: 3_100, write: 0 } },
              cost: 0.04,
            },
          ],
          tokens: { input: 4_200, output: 310, reasoning: 90, cache: { read: 3_100, write: 0 } },
          cost: 0.04,
          finish: "tool-calls",
          time: { created: started + 121_000, completed: started + 122_700 },
        },
        {
          id: "msg_coding1_02",
          type: "assistant",
          agent: "coding",
          model: { id: "claude-sonnet-5", providerID: "anthropic" },
          content: [
            {
              type: "tool",
              id: "call_bash_vitest_1",
              name: "Bash",
              time: {
                created: started + 880_000,
                ran: started + 880_100,
                completed: started + 884_300,
              },
              state: {
                status: "completed",
                input: { command: "pnpm exec vitest run src/routes/+page.test.ts" },
                content: [
                  {
                    type: "text",
                    text:
                      " RUN  v2.1.4 /workspace/gimbal/web\n\n ✓ src/routes/+page.test.ts (9)\n" +
                      "   ✓ rack renders seven tiles\n   ✓ click-to-place fills the selected square\n" +
                      "   ✓ drag from rack to empty square places the tile\n" +
                      "   ✓ drag to an occupied square snaps back\n   ✓ drag off the board snaps back\n" +
                      "   ✓ interrupted drag clears the dragging class\n\n Test Files  1 passed (1)\n" +
                      "      Tests  9 passed (9)\n",
                  },
                ],
              },
              tokens: { input: 1_100, output: 60, reasoning: 0, cache: { read: 900, write: 0 } },
              cost: 0.01,
            },
            { type: "text", text: turn1EndedResult },
          ],
          tokens: { input: 1_100, output: 640, reasoning: 0, cache: { read: 900, write: 0 } },
          cost: 0.05,
          finish: "stop",
          time: { created: started + 880_000, completed: started + 889_500 },
        },
      ],
    },
    pending: {},
    permission: {},
    form: {},
  },
};

// ---------------------------------------------------------------------------
// Transcript for coding.1/turn.2 (running) — the ugly one from issue 325.
// ---------------------------------------------------------------------------

const nativeSession2 = "ses_coding_turn2";

const pageSvelteLines = [
  '<script lang="ts">',
  "\timport { onMount } from 'svelte';",
  "\timport type { BoardSquare, RackTile } from '$lib/scrabbler/types.js';",
  "\timport { placeTile, canPlace, scoreWord } from '$lib/scrabbler/rules.js';",
  "\timport { submitTurn } from './scrabbler.remote.js';",
  "",
  "\tlet rack = $state<RackTile[]>(initialRack());",
  "\tlet board = $state<BoardSquare[][]>(initialBoard());",
  "\tlet selectedTile = $state<number | undefined>(undefined);",
  "\tlet dragging = $state<number | undefined>(undefined);",
  "\tlet hoverSquare = $state<{ row: number; col: number } | undefined>(undefined);",
  "\tlet turnScore = $derived(scoreWord(board, currentWord(board)));",
  "",
  "\tfunction selectTile(index: number) {",
  "\t\tselectedTile = selectedTile === index ? undefined : index;",
  "\t}",
  "",
  "\tfunction onSquareClick(row: number, col: number) {",
  "\t\tif (selectedTile === undefined) return;",
  "\t\tif (!canPlace(board, row, col)) return;",
  "\t\tconst tile = rack[selectedTile];",
  "\t\tif (!tile) return;",
  "\t\tplaceTile(board, rack, selectedTile, row, col);",
  "\t\tselectedTile = undefined;",
  "\t}",
  "",
  "\tfunction onTileDragStart(event: DragEvent, index: number) {",
  "\t\tconst tile = rack[index];",
  "\t\tif (!tile || !event.dataTransfer) return;",
  "\t\tevent.dataTransfer.effectAllowed = 'move';",
  "\t\tevent.dataTransfer.setData(",
  "\t\t\t'text/plain',",
  "\t\t\tJSON.stringify({ letter: tile.letter, rackIndex: index }),",
  "\t\t);",
  "\t\tdragging = index;",
  "\t}",
  "",
  "\tfunction onTileDragEnd() {",
  "\t\tdragging = undefined;",
  "\t\thoverSquare = undefined;",
  "\t}",
  "",
  "\tfunction onSquareDragOver(event: DragEvent, row: number, col: number) {",
  "\t\tevent.preventDefault();",
  "\t\tif (!canPlace(board, row, col)) return;",
  "\t\thoverSquare = { row, col };",
  "\t}",
  "",
  "\tfunction onSquareDrop(event: DragEvent, row: number, col: number) {",
  "\t\tevent.preventDefault();",
  "\t\thoverSquare = undefined;",
  "\t\tconst raw = event.dataTransfer?.getData('text/plain');",
  "\t\tif (!raw) return;",
  "\t\tconst payload = JSON.parse(raw) as { letter: string; rackIndex: number };",
  "\t\tconst tile = rack[payload.rackIndex];",
  "\t\tif (!tile || tile.letter !== payload.letter) return;",
  "\t\tif (!canPlace(board, row, col)) return;",
  "\t\tplaceTile(board, rack, payload.rackIndex, row, col);",
  "\t\tdragging = undefined;",
  "\t}",
  "",
  "\tfunction initialRack(): RackTile[] {",
  "\t\treturn drawTiles(7);",
  "\t}",
  "",
  "\tfunction initialBoard(): BoardSquare[][] {",
  "\t\treturn Array.from({ length: 15 }, (_, row) =>",
  "\t\t\tArray.from({ length: 15 }, (_, col) => ({ row, col, letter: undefined, bonus: bonusFor(row, col), locked: false })),",
  "\t\t);",
  "\t}",
  "",
  "\tonMount(() => {",
  "\t\t// Nothing to do yet; state above is initialized eagerly.",
  "\t});",
  "</script>",
  "",
  '<div class="board">',
  "\t{#each board as row, rowIndex}",
  '\t\t<div class="row">',
  "\t\t\t{#each row as square, colIndex}",
  "\t\t\t\t<button",
  '\t\t\t\t\tclass="square"',
  "\t\t\t\t\tclass:hover={hoverSquare?.row === rowIndex && hoverSquare?.col === colIndex}",
  "\t\t\t\t\tclass:locked={square.locked}",
  "\t\t\t\t\tondragover={(event) => onSquareDragOver(event, rowIndex, colIndex)}",
  "\t\t\t\t\tondrop={(event) => onSquareDrop(event, rowIndex, colIndex)}",
  "\t\t\t\t\tonclick={() => onSquareClick(rowIndex, colIndex)}",
  "\t\t\t\t>{square.letter ?? ''}</button>",
  "\t\t\t{/each}",
  "\t\t</div>",
  "\t{/each}",
  "</div>",
  "",
  '<div class="rack">',
  "\t{#each rack as tile, index}",
  "\t\t<button",
  '\t\t\tclass="tile"',
  "\t\t\tclass:selected={selectedTile === index}",
  "\t\t\tclass:dragging={dragging === index}",
  '\t\t\tdraggable="true"',
  "\t\t\tondragstart={(event) => onTileDragStart(event, index)}",
  "\t\t\tondragend={onTileDragEnd}",
  "\t\t\tonclick={() => selectTile(index)}",
  "\t\t>{tile.letter}</button>",
  "\t{/each}",
  "</div>",
  "",
  "<footer>",
  "\t<span>Score this turn: {turnScore}</span>",
  "\t<button onclick={() => submitTurn({ board, rack })}>Submit turn</button>",
  "</footer>",
  "",
  "<style>",
  "\t.board { display: grid; gap: 2px; grid-template-columns: repeat(15, 1fr); }",
  "\t.square { aspect-ratio: 1; border: 1px solid var(--square-border); background: var(--square-bg); }",
  "\t.square.hover { background: var(--square-selected-bg); }",
  "\t.square.locked { cursor: default; }",
  "\t.rack { display: flex; gap: 6px; margin-top: 12px; }",
  "\t.tile { width: 40px; height: 40px; cursor: grab; }",
  "\t.tile.selected { outline: 2px solid var(--square-selected-bg); }",
  "\t.tile.dragging { opacity: 0.4; }",
  "\tfooter { display: flex; align-items: center; gap: 12px; margin-top: 16px; }",
  "</style>",
];

const catNumbered = (lines: string[], start = 1) =>
  lines.map((line, index) => `${String(start + index).padStart(6)}\t${line}`).join("\n");

// Pad past 150 lines with a second, realistic pass: the pre-existing test
// file's setup boilerplate, which the worker also had to read before adding
// cases to it.
const pageTestSetupLines = [
  "import { describe, it, expect, beforeEach } from 'vitest';",
  "import { render, fireEvent, cleanup } from '@testing-library/svelte';",
  "import { tick } from 'svelte';",
  "import Page from './+page.svelte';",
  "",
  "function dragPayload(letter: string, rackIndex: number) {",
  "\treturn { letter, rackIndex };",
  "}",
  "",
  "function makeDataTransfer(data: Record<string, string>) {",
  "\tconst store = new Map(Object.entries(data));",
  "\treturn {",
  "\t\tsetData: (type: string, value: string) => store.set(type, value),",
  "\t\tgetData: (type: string) => store.get(type) ?? '',",
  "\t\teffectAllowed: 'move',",
  "\t} as unknown as DataTransfer;",
  "}",
  "",
  "async function fireDrop(element: Element, payload: { letter: string; rackIndex: number }) {",
  "\tconst dataTransfer = makeDataTransfer({ 'text/plain': JSON.stringify(payload) });",
  "\tawait fireEvent.dragOver(element, { dataTransfer });",
  "\tawait fireEvent.drop(element, { dataTransfer });",
  "\tawait tick();",
  "}",
  "",
  "describe('Scrabbler board drag-and-drop', () => {",
  "\tbeforeEach(() => {",
  "\t\tcleanup();",
  "\t});",
  "",
  "\tit('rack renders seven tiles', async () => {",
  "\t\tconst { findAllByTestId } = render(Page);",
  "\t\tconst tiles = await findAllByTestId('rack-tile');",
  "\t\texpect(tiles).toHaveLength(7);",
  "\t});",
  "",
  "\tit('click-to-place fills the selected square', async () => {",
  "\t\tconst { getAllByTestId } = render(Page);",
  "\t\tconst [tile] = getAllByTestId('rack-tile');",
  "\t\tconst [square] = getAllByTestId('board-square');",
  "\t\tawait fireEvent.click(tile);",
  "\t\tawait fireEvent.click(square);",
  "\t\texpect(square.textContent).not.toBe('');",
  "\t});",
  "});",
];

const readPageResult =
  catNumbered(pageSvelteLines, 1) + "\n" + catNumbered(pageTestSetupLines, 201);

// A single unbreakable 600-character token, as if a minified vitest browser
// bundle or a base64 blob had been dumped straight into a tool result.
const unbreakableToken =
  Array.from(
    { length: 10 },
    (_, i) =>
      "sc" +
      i.toString(36).padStart(3, "0") +
      "bW9kdWxlLmV4cG9ydHM9ZnVuY3Rpb24oZSx0KXtyZXR1cm4gZS5tYXAoZnVuY3Rpb24oZSl7cmV0dXJuIHQoZSl9KX0",
  ).join("") + "==";

const dragPreviewWriteLines = [
  '<script lang="ts">',
  "\t// Small helper component: a floating preview of the tile under the",
  "\t// cursor while a native drag is in progress. HTML5 drag-and-drop does",
  "\t// not render a controllable custom drag image on every platform, so",
  "\t// this sits fixed-positioned and hidden except while `dragging` is set,",
  "\t// purely as a CSS affordance layered on top of the browser's own ghost.",
  "\timport type { RackTile } from './types.js';",
  "",
  "\tlet { tile, visible }: { tile: RackTile | undefined; visible: boolean } = $props();",
  "</script>",
  "",
  "{#if visible && tile}",
  '\t<div class="drag-preview" aria-hidden="true">{tile.letter}</div>',
  "{/if}",
  "",
  "<style>",
  "\t.drag-preview {",
  "\t\tposition: fixed;",
  "\t\tpointer-events: none;",
  "\t\ttop: -9999px;",
  "\t\tleft: -9999px;",
  "\t\twidth: 40px;",
  "\t\theight: 40px;",
  "\t\tdisplay: flex;",
  "\t\talign-items: center;",
  "\t\tjustify-content: center;",
  "\t\tbackground: var(--card);",
  "\t\tborder: 1px solid var(--border);",
  "\t\tborder-radius: 6px;",
  "\t\tfont-weight: 600;",
  "\t\tz-index: 40;",
  "\t}",
  "</style>",
  "",
  "<!--",
  "\tNOTE: not wired into +page.svelte yet — evaluating whether the native",
  "\tdrag image (set via event.dataTransfer.setDragImage) is sufficient on",
  "\tits own before adding this as a second rendering path. Left in the tree",
  "\tunused so QA can see the option; will delete if the native image reads",
  "\tclearly enough in the Playwright pass.",
  "-->",
];
// Pad to 80+ lines with plausible, on-topic comment/blank filler so the tool
// result reproduces a long single Write payload.
while (dragPreviewWriteLines.length < 82) {
  dragPreviewWriteLines.push(
    dragPreviewWriteLines.length % 2 === 0
      ? ""
      : "<!-- kept minimal on purpose; see note above before extending this file -->",
  );
}
const dragPreviewWriteContent = dragPreviewWriteLines.join("\n");

const nestedSubagentTranscript = [
  {
    delta: { text: "Checking whether any other component already listens for native drag events…" },
  },
  {
    delta: {
      text: 'grep -rn "ondragstart\\|ondrop" web/src --include=*.svelte found no matches outside +page.svelte.',
    },
  },
  { delta: { text: "No conflicting drag handling found. Safe to proceed in +page.svelte alone." } },
];

const transcript2: Snapshot = {
  state: {
    info: {
      [nativeSession2]: {
        agent: "coding",
        model: { id: "claude-sonnet-5", providerID: "anthropic" },
      },
    },
    family: {},
    active: { [nativeSession2]: "running" },
    message: {
      [nativeSession2]: [
        {
          id: "msg_coding2_00",
          type: "user",
          text:
            "Implement task 1 of the Scrabbler drag-and-drop milestone. See the assignment and " +
            "definition of done in this turn's context.",
          time: { created: started + 939_000 },
        },
        {
          id: "msg_coding2_01",
          type: "assistant",
          agent: "coding",
          model: { id: "claude-sonnet-5", providerID: "anthropic" },
          content: [
            {
              type: "reasoning",
              text:
                "The previous pass failed task check on two cases: drop onto an occupied square " +
                "and drop off the board. Both point at the same bug — onSquareDrop calls " +
                "placeTile() before checking square.locked, and there's no drop target for " +
                "off-board drags at all so the rack's optimistic removal never gets reverted. " +
                "I should read the current file fully before touching it again rather than " +
                "trusting my memory of the previous edit.",
              time: { created: started + 940_200, completed: started + 941_100 },
            },
            {
              type: "text",
              text: "Re-reading +page.svelte and the test file before making the fix.",
            },
          ],
          tokens: { input: 3_100, output: 240, reasoning: 620, cache: { read: 2_400, write: 0 } },
          cost: 0.03,
          finish: "tool-calls",
          time: { created: started + 940_000, completed: started + 941_300 },
        },
        {
          id: "msg_coding2_02",
          type: "assistant",
          agent: "coding",
          model: { id: "claude-sonnet-5", providerID: "anthropic" },
          content: [
            {
              type: "tool",
              id: "call_read_page",
              name: "Read",
              time: {
                created: started + 941_400,
                ran: started + 941_450,
                completed: started + 941_900,
              },
              state: {
                status: "completed",
                input: { file_path: "web/src/routes/+page.svelte", offset: 1, limit: 400 },
                content: [{ type: "text", text: readPageResult }],
              },
            },
          ],
          tokens: { input: 5_900, output: 40, reasoning: 0, cache: { read: 4_800, write: 0 } },
          cost: 0.02,
          finish: "tool-calls",
          time: { created: started + 941_400, completed: started + 941_950 },
        },
        {
          id: "msg_coding2_03",
          type: "assistant",
          agent: "coding",
          model: { id: "claude-sonnet-5", providerID: "anthropic" },
          content: [
            {
              type: "tool",
              id: "call_grep_ondrop",
              name: "Grep",
              time: {
                created: started + 942_000,
                ran: started + 942_050,
                completed: started + 942_300,
              },
              state: {
                status: "completed",
                input: {
                  pattern: "onSquareDrop|placeTile",
                  path: "web/src/routes",
                  output_mode: "content",
                },
                content: [
                  {
                    type: "text",
                    text:
                      "web/src/routes/+page.svelte:52:\tfunction onSquareDrop(event: DragEvent, row: number, col: number) {\n" +
                      "web/src/routes/+page.svelte:58:\t\tplaceTile(board, rack, payload.rackIndex, row, col);\n" +
                      "web/src/routes/+page.svelte:22:\t\tplaceTile(board, rack, selectedTile, row, col);\n",
                  },
                ],
              },
            },
          ],
          tokens: { input: 2_200, output: 30, reasoning: 0, cache: { read: 1_800, write: 0 } },
          cost: 0.01,
          finish: "tool-calls",
          time: { created: started + 942_000, completed: started + 942_350 },
        },
        {
          id: "msg_coding2_04",
          type: "assistant",
          agent: "coding",
          model: { id: "claude-sonnet-5", providerID: "anthropic" },
          content: [
            {
              type: "reasoning",
              text:
                "Confirmed: onSquareDrop never checks canPlace()/square.locked before calling " +
                "placeTile, and there is no window-level dragend/drop handler to catch an " +
                "off-board release. I'll guard the in-board drop with the same canPlace() check " +
                "the click path already uses, and add a document-level dragend listener that " +
                "clears `dragging` if drop never fired, which by itself restores the rack's " +
                "visual state because rack rendering already keys off `dragging`.",
              time: { created: started + 942_500, completed: started + 943_600 },
            },
            {
              type: "tool",
              id: "call_edit_drop_guard",
              name: "Edit",
              time: {
                created: started + 943_700,
                ran: started + 943_750,
                completed: started + 944_400,
              },
              state: {
                status: "completed",
                input: {
                  file_path: "web/src/routes/+page.svelte",
                  old_string:
                    "\tfunction onSquareDrop(event: DragEvent, row: number, col: number) {\n\t\tevent.preventDefault();\n\t\thoverSquare = undefined;\n\t\tconst raw = event.dataTransfer?.getData('text/plain');\n\t\tif (!raw) return;\n\t\tconst payload = JSON.parse(raw) as { letter: string; rackIndex: number };\n\t\tconst tile = rack[payload.rackIndex];\n\t\tif (!tile || tile.letter !== payload.letter) return;\n\t\tif (!canPlace(board, row, col)) return;\n\t\tplaceTile(board, rack, payload.rackIndex, row, col);\n\t\tdragging = undefined;\n\t}",
                  new_string:
                    "\tfunction onSquareDrop(event: DragEvent, row: number, col: number) {\n\t\tevent.preventDefault();\n\t\thoverSquare = undefined;\n\t\tconst raw = event.dataTransfer?.getData('text/plain');\n\t\tif (!raw) {\n\t\t\tdragging = undefined;\n\t\t\treturn;\n\t\t}\n\t\tconst payload = JSON.parse(raw) as { letter: string; rackIndex: number };\n\t\tconst tile = rack[payload.rackIndex];\n\t\tif (!tile || tile.letter !== payload.letter) {\n\t\t\tdragging = undefined;\n\t\t\treturn;\n\t\t}\n\t\t// Guard: a locked/occupied square must snap back, not overwrite.\n\t\tif (!canPlace(board, row, col)) {\n\t\t\tdragging = undefined;\n\t\t\treturn;\n\t\t}\n\t\tplaceTile(board, rack, payload.rackIndex, row, col);\n\t\tdragging = undefined;\n\t}\n\n\tfunction onWindowDragEnd() {\n\t\t// Off-board release: no square's ondrop ever fires, so this is the\n\t\t// only place that clears `dragging` and lets the rack re-render the\n\t\t// tile in place. Bound on <svelte:window> below.\n\t\tdragging = undefined;\n\t\thoverSquare = undefined;\n\t}",
                },
                content: [{ type: "text", text: "Applied 1 edit to web/src/routes/+page.svelte" }],
              },
            },
          ],
          tokens: { input: 6_400, output: 980, reasoning: 410, cache: { read: 5_100, write: 0 } },
          cost: 0.09,
          finish: "tool-calls",
          time: { created: started + 942_500, completed: started + 944_500 },
        },
        {
          id: "msg_coding2_05",
          type: "assistant",
          agent: "coding",
          model: { id: "claude-sonnet-5", providerID: "anthropic" },
          content: [
            {
              type: "tool",
              id: "call_edit_svelte_window",
              name: "Edit",
              time: {
                created: started + 944_600,
                ran: started + 944_650,
                completed: started + 944_950,
              },
              state: {
                status: "completed",
                input: {
                  file_path: "web/src/routes/+page.svelte",
                  old_string: '<div class="board">',
                  new_string:
                    '<svelte:window ondragend={onWindowDragEnd} />\n\n<div class="board">',
                },
                content: [{ type: "text", text: "Applied 1 edit to web/src/routes/+page.svelte" }],
              },
            },
          ],
          tokens: { input: 2_000, output: 90, reasoning: 0, cache: { read: 1_600, write: 0 } },
          cost: 0.01,
          finish: "tool-calls",
          time: { created: started + 944_600, completed: started + 945_000 },
        },
        {
          id: "msg_coding2_06",
          type: "assistant",
          agent: "coding",
          model: { id: "claude-sonnet-5", providerID: "anthropic" },
          content: [
            {
              type: "tool",
              id: "call_bash_vitest_run_1",
              name: "Bash",
              time: {
                created: started + 945_100,
                ran: started + 945_150,
                completed: started + 949_800,
              },
              state: {
                status: "completed",
                input: { command: "pnpm exec vitest run src/routes/+page.test.ts" },
                content: [
                  {
                    type: "text",
                    text:
                      " RUN  v2.1.4 /workspace/gimbal/web\n\n ✓ src/routes/+page.test.ts (9)\n" +
                      "   ✓ rack renders seven tiles\n   ✓ click-to-place fills the selected square\n" +
                      "   ✓ drag from rack to empty square places the tile\n" +
                      "   ✓ drag to an occupied square snaps back\n   ✓ drag off the board snaps back\n" +
                      "   ✓ interrupted drag clears the dragging class\n\n" +
                      " minified-report-blob: " +
                      unbreakableToken +
                      "\n\n" +
                      " Test Files  1 passed (1)\n      Tests  9 passed (9)\n   Duration  3.98s\n",
                  },
                ],
              },
              tokens: { input: 1_400, output: 70, reasoning: 0, cache: { read: 1_100, write: 0 } },
              cost: 0.01,
            },
          ],
          tokens: { input: 1_400, output: 70, reasoning: 0, cache: { read: 1_100, write: 0 } },
          cost: 0.01,
          finish: "tool-calls",
          time: { created: started + 945_100, completed: started + 949_850 },
        },
        {
          id: "msg_coding2_07",
          type: "assistant",
          agent: "coding",
          model: { id: "claude-sonnet-5", providerID: "anthropic" },
          content: [
            {
              type: "text",
              text:
                "Both previously-failing cases pass now. Writing a small drag-preview component " +
                "to evaluate as an optional visual affordance before deciding whether it's worth " +
                "wiring in, since the native browser drag image may already be enough on its own.",
            },
            {
              type: "tool",
              id: "call_write_drag_preview",
              name: "Write",
              time: {
                created: started + 950_000,
                ran: started + 950_050,
                completed: started + 950_600,
              },
              state: {
                status: "completed",
                input: {
                  file_path: "web/src/lib/scrabbler/DragPreview.svelte",
                  content: dragPreviewWriteContent,
                },
                content: [
                  {
                    type: "text",
                    text: "Wrote web/src/lib/scrabbler/DragPreview.svelte (82 lines)",
                  },
                ],
              },
            },
          ],
          tokens: { input: 1_900, output: 1_240, reasoning: 0, cache: { read: 1_500, write: 0 } },
          cost: 0.05,
          finish: "tool-calls",
          time: { created: started + 950_000, completed: started + 950_700 },
        },
        {
          id: "msg_coding2_08",
          type: "assistant",
          agent: "coding",
          model: { id: "claude-sonnet-5", providerID: "anthropic" },
          content: [
            {
              type: "reasoning",
              text:
                "The definition of done says not to add a second rendering path without reason; " +
                "I left DragPreview.svelte unused for now rather than wiring it in, and should " +
                "check whether anything else in the tree already listens for drag events before " +
                "going further, so a conflict doesn't surface later in QA.",
              time: { created: started + 951_000, completed: started + 951_500 },
            },
            {
              type: "tool",
              id: "call_task_subagent_check",
              name: "Task",
              time: {
                created: started + 951_600,
                ran: started + 951_650,
                completed: started + 953_100,
              },
              state: {
                status: "completed",
                input: {
                  description: "Check for conflicting drag handlers",
                  prompt:
                    "Search web/src for any component other than +page.svelte that already binds " +
                    "ondragstart, ondragover, or ondrop, and report what you find.",
                },
                content: [
                  {
                    type: "text",
                    text: "No conflicting drag handling found outside +page.svelte.",
                  },
                ],
                metadata: { transcript: nestedSubagentTranscript },
              },
            },
          ],
          tokens: { input: 2_600, output: 210, reasoning: 340, cache: { read: 2_000, write: 0 } },
          cost: 0.02,
          finish: "tool-calls",
          time: { created: started + 951_000, completed: started + 953_200 },
        },
        {
          id: "msg_coding2_09",
          type: "assistant",
          agent: "coding",
          model: { id: "claude-sonnet-5", providerID: "anthropic" },
          content: [
            {
              type: "tool",
              id: "call_bash_full_suite",
              name: "Bash",
              time: { created: started + 953_300, ran: started + 953_350 },
              state: {
                status: "error",
                error: {
                  type: "shell.exit",
                  message:
                    "Command exited with code 127: pnpm exec vitest run --browser.headless (browser " +
                    "binary not found in this sandbox; Playwright's Chromium was not installed with " +
                    "`pnpm exec playwright install chromium` before this call)",
                },
                input: { command: "pnpm exec vitest run --browser.headless" },
                content: [
                  {
                    type: "text",
                    text:
                      "Error: browserType.launch: Executable doesn't exist at " +
                      "/home/worker/.cache/ms-playwright/chromium-1148/chrome-linux/chrome\n" +
                      "Looks like Playwright Test or Playwright was just installed or updated.\n" +
                      "Please run the following command to download new browsers:\n\n" +
                      "    pnpm exec playwright install chromium\n",
                  },
                ],
              },
            },
          ],
          tokens: { input: 900, output: 20, reasoning: 0, cache: { read: 700, write: 0 } },
          cost: 0,
          finish: "tool-calls",
          time: { created: started + 953_300, completed: started + 953_450 },
        },
        {
          id: "msg_coding2_10",
          type: "assistant",
          agent: "coding",
          model: { id: "claude-sonnet-5", providerID: "anthropic" },
          content: [
            {
              type: "reasoning",
              text:
                "The headless flag rebuilds Playwright's browser matrix; the plain `vitest run` " +
                "invocation I used above already exercises the browser-mode config the project " +
                "ships and passed cleanly, so this second invocation was redundant and the " +
                "missing-binary error doesn't call the earlier pass into question. I'll drop the " +
                "extra flag and confirm the whole suite, not just this file, still passes.",
              time: { created: started + 954_000, completed: started + 954_500 },
            },
            {
              type: "text",
              text: "Retrying without the redundant --browser.headless flag, against the whole test suite this time.",
            },
          ],
          tokens: { input: 1_300, output: 160, reasoning: 260, cache: { read: 1_000, write: 0 } },
          cost: 0.01,
          finish: "tool-calls",
          time: { created: started + 954_000, completed: started + 954_650 },
        },
        {
          id: "msg_coding2_11",
          type: "assistant",
          agent: "coding",
          model: { id: "claude-sonnet-5", providerID: "anthropic" },
          content: [
            {
              type: "text",
              text: "Running the full vitest suite now to confirm nothing outside +page.test.ts regressed.",
            },
            {
              type: "tool",
              id: "call_bash_full_suite_2",
              name: "Bash",
              time: { created: started + 955_000, ran: started + 955_050 },
              state: { status: "running", input: { command: "pnpm exec vitest run" } },
            },
          ],
          tokens: { input: 1_100, output: 40, reasoning: 0, cache: { read: 900, write: 0 } },
          cost: 0,
          time: { created: started + 955_000 },
        },
      ],
    },
    pending: {},
    permission: {},
    form: {},
  },
};

const transcripts = {
  "coding.1/turn.1": { snapshot: transcript1, provenance: {} },
  "coding.1/turn.2": { snapshot: transcript2, provenance: {} },
};

// ---------------------------------------------------------------------------
// Snapshot
// ---------------------------------------------------------------------------

export const issue325Snapshot: RunSnapshot = {
  stream: "issue-325-specimen",
  position: 61,
  run: {
    id: runID,
    name: "implement",
    status: "running",
    error: "",
    started,
    ended: 0,
  },
  scopes,
  sessions,
  interviews: {},
  turns,
  turn_usage: { "coding.1/turn.2": { "claude-sonnet-5": codingUsage } },
  model_calls: modelCalls,
  commands,
  totals: {
    scopes: {
      "implementation.1/task.1": { all: codingUsage, by_model: { "claude-sonnet-5": codingUsage } },
    },
    sessions: { "coding.1": { all: codingUsage, by_model: { "claude-sonnet-5": codingUsage } } },
  },
  transcripts,
};

export const issue325Fixture = {
  graph: issue325Graph,
  snapshot: issue325Snapshot,
};

// ---------------------------------------------------------------------------
// Finished variant: turn.2 ended, task check passed, scope and run ended.
// ---------------------------------------------------------------------------

const finishedSnapshot: RunSnapshot = structuredClone(issue325Snapshot);

const finishedEnd = started + 1_180_000;

const finishedTurn2 = finishedSnapshot.turns["coding.1/turn.2"] as TurnRow;
finishedTurn2.result =
  "Added the document-level dragend listener and the canPlace() guard on the in-board drop " +
  "path, fixing both cases task check flagged: an occupied-square drop no longer overwrites the " +
  "existing tile, and an off-board release now clears `dragging` so the rack tile reappears.\n\n" +
  "Ran `pnpm exec vitest run` for the full suite (not just +page.test.ts) and watched all cases " +
  "pass in a real Chromium instance, including the four new drag-and-drop cases and the " +
  "pre-existing click-to-place and scoring cases.\n\n" +
  "Left DragPreview.svelte unused in the tree pending a decision on whether the native drag " +
  "image is sufficient; flagged this for the architectural-critique watcher rather than deciding " +
  "unilaterally.\n\n" +
  "Did not touch scoring, turn submission, or lib/server/.";
finishedTurn2.ended = finishedEnd;
finishedTurn2.duration = finishedEnd - finishedTurn2.started;

const finishedTranscript2 = finishedSnapshot.transcripts["coding.1/turn.2"]!.snapshot;
const finishedMessages = finishedTranscript2.state.message[nativeSession2]!;
const runningToolMessage = finishedMessages.at(-1)!;
const runningTool = runningToolMessage.content.find(
  (part: { type: string }) => part.type === "tool",
) as { state: Record<string, unknown>; time: Record<string, unknown> } | undefined;
if (runningTool) {
  const previousInput = (runningTool.state as { input?: unknown }).input;
  runningTool.state = {
    status: "completed",
    input: previousInput,
    content: [
      {
        type: "text",
        text: " RUN  v2.1.4 /workspace/gimbal/web\n\n Test Files  6 passed (6)\n      Tests  41 passed (41)\n   Duration  11.42s\n",
      },
    ],
  };
  runningTool.time.completed = finishedEnd - 2_000;
}
runningToolMessage.time.completed = finishedEnd - 1_500;
runningToolMessage.finish = "stop";
finishedTranscript2.state.active[nativeSession2] = "idle";

const finishedCommand = finishedSnapshot.commands["implementation.1/task.1/task-check.1"]!;
finishedCommand.exit_code = 0;
finishedCommand.stdout =
  " RUN  v2.1.4 /workspace/gimbal/web\n\n ✓ src/routes/+page.test.ts (9)\n Test Files  1 passed (1)\n      Tests  9 passed (9)\n";
finishedCommand.stderr = "";
finishedCommand.ended = finishedCommand.started + 4_100;
finishedCommand.duration = 4_100;

finishedSnapshot.scopes["implementation.1/task.1"]!.status = "ended";
finishedSnapshot.scopes["implementation.1/task.1"]!.ended = finishedEnd;
finishedSnapshot.scopes["implementation.1"]!.status = "ended";
finishedSnapshot.scopes["implementation.1"]!.ended = finishedEnd + 5_000;
finishedSnapshot.scopes[""]!.status = "ended";
finishedSnapshot.scopes[""]!.ended = finishedEnd + 6_000;
finishedSnapshot.run.status = "completed";
finishedSnapshot.run.ended = finishedEnd + 6_000;

export const issue325Finished: RunSnapshot = finishedSnapshot;
