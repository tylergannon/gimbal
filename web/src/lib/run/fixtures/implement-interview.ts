import type { RunSnapshot } from "../../observation/index.js";
import type { Graph } from "../../workflow/types.js";
import { at, total, usage } from "./support.js";

/** The one file every operation of this workflow is written in, as the
 * generated graph records it: relative to the module root, forward slashes. */
const file = "internal/workflows/implementinterview/implementinterview.go";

/** The constants the source passes, named here once so the graph and the
 * turns a run recorded say the same thing. */
const backendResearchPrompt =
  "Read the saved issue, current Go API and design docs, pinned dependencies, runtime/events/generation code, Polytype use, and skgo Go remote-function APIs. Do not edit source. Under the reference directory, create backend/index.md with deep implementation notes and provenance, and save useful official source documentation under backend/. Distinguish verified facts from proposals.";
const frontendResearchPrompt =
  "Read the saved issue, current live-run UI, pinned frontend dependencies, and installed shadcn-svelte components. Research the actual Svelte, SvelteKit, shadcn-svelte, and Bits UI APIs needed by the issue. Do not edit source. Under the reference directory, create frontend/index.md with deep implementation notes and provenance, and save useful official source documentation under frontend/. Distinguish verified facts from proposals.";
const codingPrompt =
  "Read the saved issue and the local reference directory, then implement only the selected task in the repository. Do not expand the specification, modify the requirements or this build workflow, weaken its fixed checks, commit, merge, or add proof scripts or run output. Preserve unrelated work. Answer with what changed and what you personally ran or observed.";
const backendScopePrompt =
  "Watch only for concrete backend or public-API work beyond the saved issue 249. Steer against unnecessary wrappers, frameworks, speculative APIs, and unrelated features. Do not edit source, object on style, or demand improvements outside the requirements.";
const architectureScopePrompt =
  "Watch only for concrete frontend or general implementation work beyond the saved issue 249. Steer against unnecessary abstractions, frameworks, speculative features, and unrelated polish. Do not edit source, object on style, or demand improvements outside the requirements.";
const loopGoal =
  "Implement exactly the locally saved Gimble issue 249 at /Users/tyler/src/gimble/tmp/issue-249.md and demonstrate its complete definition of done through the real running web interface.";
const validationPrompt =
  "Independently read the saved issue, reference directory, implementation, tests, and recorded command results. Make no source changes and do not treat the worker report as evidence. Start the generated interview example on a separate port with its interviewer bound to Terra or Sonnet, never Gemini. Use the installed Playwright through the shell to conduct successive human answers in a real browser; do not write proof programs or commit run artifacts, and cleanly stop the example process afterward. Complete is true only if you personally demonstrate every requirement in that running interface; tests and reports alone are insufficient.";

/** implement-interview's shape, transcribed from the generated
 * `internal/workflows/implementinterview/workflow_gen.go`: the same
 * operations in the same source order, at the same lines, with the prompt
 * and instruction constants the source passes. Nothing is added and nothing
 * is left out, so the map drawn from it is the map of the real workflow. */
export const implementInterviewGraph: Graph = {
  name: "implement-interview",
  source: { file, line: 45 },
  body: [
    {
      kind: "condition",
      file,
      line: 46,
      branches: [
        {
          file,
          line: 46,
          case: "!filepath.IsAbs(params.RequirementsFile) || !filepath.IsAbs(params.ReferenceDir)",
          exits: true,
          body: [],
        },
      ],
    },
    {
      kind: "condition",
      file,
      line: 49,
      branches: [{ file, line: 49, case: "params.MaxTasks < 1", exits: true, body: [] }],
    },
    {
      kind: "condition",
      file,
      line: 55,
      branches: [
        {
          file,
          line: 55,
          case: "info, err := os.Stat(params.ReferenceDir); err != nil || !info.IsDir()",
          exits: true,
          body: [],
        },
      ],
    },
    { kind: "set", file, line: 59, key: "requirements-file" },
    { kind: "set", file, line: 60, key: "reference-directory" },
    { kind: "set", file, line: 61, key: "repository" },
    {
      kind: "group",
      file,
      line: 63,
      name: "reconnaissance",
      children: [
        {
          file,
          line: 64,
          name: "backend",
          body: [
            { kind: "session", file, line: 65, name: "api-research", from: "" },
            {
              kind: "agent_call",
              file,
              line: 66,
              session: "api-research",
              role: "api-research",
              prompt: backendResearchPrompt,
              supervisors: [],
            },
          ],
        },
        {
          file,
          line: 69,
          name: "frontend",
          body: [
            { kind: "session", file, line: 70, name: "frontend-research", from: "" },
            {
              kind: "agent_call",
              file,
              line: 71,
              session: "frontend-research",
              role: "frontend-research",
              prompt: frontendResearchPrompt,
              supervisors: [],
            },
          ],
        },
      ],
    },
    { kind: "session", file, line: 78, name: "sprint-planning", from: "" },
    { kind: "session", file, line: 79, name: "coding", from: "" },
    { kind: "session", file, line: 80, name: "implementation-scope-review", from: "" },
    { kind: "session", file, line: 81, name: "architectural-critique", from: "" },
    {
      kind: "promise_loop",
      file,
      line: 84,
      name: "implementation",
      planner: "sprint-planning",
      supervisors: [],
      body: [
        {
          kind: "agent_call",
          file,
          line: 92,
          session: "coding",
          role: "coding",
          prompt: codingPrompt,
          supervisors: [
            {
              file,
              line: 93,
              session: "implementation-scope-review",
              role: "implementation-scope-review",
              instruction: backendScopePrompt,
              supervisors: [],
            },
            {
              file,
              line: 94,
              session: "architectural-critique",
              role: "architectural-critique",
              instruction: architectureScopePrompt,
              supervisors: [],
            },
          ],
        },
        {
          kind: "condition",
          file,
          line: 102,
          branches: [
            {
              file,
              line: 102,
              case: 'command := strings.TrimSpace(task.Validation.Command); command != ""',
              exits: false,
              body: [
                { kind: "command", file, line: 103, name: "task-check" },
                { kind: "set", file, line: 104, key: "task check" },
              ],
            },
          ],
        },
        { kind: "command", file, line: 112, name: "build" },
        { kind: "set", file, line: 113, key: "just build" },
        { kind: "command", file, line: 118, name: "vet" },
        { kind: "set", file, line: 119, key: "just vet" },
        { kind: "command", file, line: 124, name: "test" },
        { kind: "set", file, line: 125, key: "just test" },
        { kind: "session", file, line: 131, name: "qa-orchestration", from: "" },
        {
          kind: "agent_call",
          file,
          line: 132,
          session: "qa-orchestration",
          role: "qa-orchestration",
          prompt: validationPrompt,
          supervisors: [],
        },
        { kind: "set", file, line: 137, key: "independent assessment" },
        { kind: "set", file, line: 138, key: "worker report" },
        { kind: "set", file, line: 140, key: "deterministic checks passed" },
        {
          kind: "condition",
          file,
          line: 141,
          branches: [
            { file, line: 141, case: "checksPassed && assessment.Complete", exits: true, body: [] },
          ],
        },
        {
          kind: "condition",
          file,
          line: 145,
          branches: [
            { file, line: 145, case: "tasksRun >= params.MaxTasks", exits: true, body: [] },
          ],
        },
      ],
    },
    {
      kind: "condition",
      file,
      line: 156,
      branches: [{ file, line: 156, case: "completed", exits: true, body: [] }],
    },
    {
      kind: "condition",
      file,
      line: 159,
      branches: [{ file, line: 159, case: "exhausted", exits: true, body: [] }],
    },
  ],
  diagnostics: [],
};

const run = "01M2RX4A9Q0ZT7V4C3D8N1KMB2";
const workdir = "/Users/tyler/src/gimble";
const claude = "claude-fable-5-1";
const codex = "gpt-5.6-sol";

/** The three assignments the planner has dispatched. Task 3 is the one the
 * run is inside now, and its description is the assignment Main.html reads
 * out under the selected coding turn. */
const assignments = [
  {
    name: "Answer routing",
    description:
      "Route an accepted interview answer back to the waiting question so the run continues without a reload.",
    definition_of_done:
      "Answering a pending question in the browser resumes the interview and the row turns answered.",
    validation: { command: "just test", query: "Does an answered question resume the interview?" },
  },
  {
    name: "Pending question list",
    description:
      "Serve every pending question of every run so the runs list can show what needs a person.",
    definition_of_done: "The runs list names each pending question and the scope it waits in.",
    validation: { command: "just test", query: "Are pending questions listed across runs?" },
  },
  {
    name: "Answered timestamp",
    description:
      "Wire the answered timestamp through the web answer flow so the interview row updates without a reload. Stay inside the observation rows.",
    definition_of_done:
      "The answered row carries the time the answer was accepted, and the page shows it as soon as it lands.",
    validation: {
      command: "just test",
      query: "Does the answered row carry the time the answer was accepted?",
    },
  },
];

/** What the run appends to a constant prompt: every key in scope, in the
 * order an agent reads them back. The coding turn of task 3 is sent the
 * three keys the run set at its root and the task the loop wrote, which is
 * the "4 context sections" the Prompt sent disclosure counts. */
const codingTurnPrompt = (task: (typeof assignments)[number]): string =>
  [
    codingPrompt,
    "## requirements-file\n\n/Users/tyler/src/gimble/tmp/issue-249.md",
    "## reference-directory\n\n/Users/tyler/src/gimble/tmp/reference",
    "## repository\n\n" + workdir,
    "## task\n\n" + JSON.stringify(task, null, 2),
  ].join("\n\n");

/** The coding session's native id, which is the key its projection is under. */
const nativeCoding = "ses_01M2RX4A_coding";

/** One tool call of the running coding turn, as the projection holds it. */
const toolCall = (id: string, name: string, input: unknown, completed?: string) => ({
  id,
  type: "tool",
  name,
  state: {
    status: completed === undefined ? "running" : "completed",
    input,
    ...(completed === undefined ? {} : { time: { completed: at(completed) } }),
  },
});

/** implement-interview as Main.html draws it: 48 minutes in, the planner's
 * third task open, the coding turn running inside it with both supervisors
 * attached, and neither the task's commands nor its validator reached yet.
 * The task scope holds one of the eight keys its body writes, which is the
 * "context 1 of 8" chip. The clock stops at 12:48:50, which is the run's
 * 48m 12s and the coding turn's 1m 48s, and the same instant plan-trip is
 * 12m 40s old at. */
export const implementInterviewSnapshot: RunSnapshot = {
  stream: "01M2RX4BQ7YH3F5K9D2S6W8NVC",
  position: 1184,
  run: {
    id: run,
    name: "implement-interview",
    status: "running",
    error: "",
    started: at("2026-09-17T12:00:38"),
    ended: 0,
  },
  scopes: {
    // path.Base("") is ".", which is the name the runtime writes for the
    // root scope. Its key is the empty string and the map draws it as the
    // canvas rather than as a sheet.
    "": {
      run,
      key: "",
      name: ".",
      loop: false,
      status: "running",
      error: "",
      began: at("2026-09-17T12:00:38"),
      ended: 0,
      values: {
        "requirements-file": { value: "/Users/tyler/src/gimble/tmp/issue-249.md" },
        "reference-directory": { value: "/Users/tyler/src/gimble/tmp/reference" },
        repository: { value: workdir },
      },
      decisions: [],
    },
    "reconnaissance.1": {
      run,
      key: "reconnaissance.1",
      name: "reconnaissance",
      loop: false,
      status: "ended",
      error: "",
      began: at("2026-09-17T12:00:40"),
      ended: at("2026-09-17T12:13:52"),
      values: {},
      decisions: [],
    },
    "reconnaissance.1/backend.1": {
      run,
      key: "reconnaissance.1/backend.1",
      name: "backend",
      loop: false,
      status: "ended",
      error: "",
      began: at("2026-09-17T12:00:41"),
      ended: at("2026-09-17T12:12:19"),
      values: {},
      decisions: [],
    },
    "reconnaissance.1/frontend.1": {
      run,
      key: "reconnaissance.1/frontend.1",
      name: "frontend",
      loop: false,
      status: "ended",
      error: "",
      began: at("2026-09-17T12:00:41"),
      ended: at("2026-09-17T12:13:51"),
      values: {},
      decisions: [],
    },
    "implementation.1": {
      run,
      key: "implementation.1",
      name: "implementation",
      loop: true,
      status: "running",
      error: "",
      began: at("2026-09-17T12:13:54"),
      ended: 0,
      values: {},
      // One record sequence and the planner's own words, in the order the
      // loop dispatched them.
      decisions: [
        { seq: 214, body: { dispatched: assignments[0] } },
        { seq: 603, body: { dispatched: assignments[1] } },
        { seq: 978, body: { dispatched: assignments[2] } },
      ],
    },
    "implementation.1/task.1": {
      run,
      key: "implementation.1/task.1",
      name: "task",
      loop: false,
      status: "ended",
      error: "",
      task: assignments[0],
      began: at("2026-09-17T12:14:33"),
      ended: at("2026-09-17T12:29:40"),
      values: {
        task: { value: assignments[0] },
        "task check": {
          value: "exit 0\n\nok  \tgithub.com/tylergannon/gimble/internal/observation",
        },
        "just build": { value: "exit 0" },
        "just vet": { value: "exit 0" },
        "just test": { value: "exit 0" },
        "independent assessment": {
          value: {
            complete: false,
            unmet_requirements: ["The answered row still carries no accepted time."],
          },
        },
        "worker report": {
          value:
            "Routed the accepted answer back to the waiting question. Ran just build, just vet and just test.",
        },
        "deterministic checks passed": { value: true },
      },
      decisions: [],
    },
    "implementation.1/task.2": {
      run,
      key: "implementation.1/task.2",
      name: "task",
      loop: false,
      status: "ended",
      error: "",
      task: assignments[1],
      began: at("2026-09-17T12:30:17"),
      ended: at("2026-09-17T12:46:30"),
      values: {
        task: { value: assignments[1] },
        "task check": { value: "exit 0" },
        "just build": { value: "exit 0" },
        "just vet": { value: "exit 0" },
        "just test": { value: "exit 0" },
        "independent assessment": {
          value: {
            complete: false,
            unmet_requirements: ["The answered row still carries no accepted time."],
          },
        },
        "worker report": {
          value:
            "Served every pending question with the scope it waits in. Ran just build, just vet and just test.",
        },
        "deterministic checks passed": { value: true },
      },
      decisions: [],
    },
    // Task 3 of 3: its body writes eight keys and has written one, which is
    // the "context 1 of 8" chip on its sheet.
    "implementation.1/task.3": {
      run,
      key: "implementation.1/task.3",
      name: "task",
      loop: false,
      status: "running",
      error: "",
      task: assignments[2],
      began: at("2026-09-17T12:46:56"),
      ended: 0,
      values: { task: { value: assignments[2] } },
      decisions: [],
    },
  },
  sessions: {
    "reconnaissance.1/backend.1/api-research.1": {
      run,
      id: "reconnaissance.1/backend.1/api-research.1",
      name: "api-research",
      adapter: "codex",
      model: codex,
      scope: "reconnaissance.1/backend.1",
      parent: "",
      created: at("2026-09-17T12:00:41"),
    },
    "reconnaissance.1/frontend.1/frontend-research.1": {
      run,
      id: "reconnaissance.1/frontend.1/frontend-research.1",
      name: "frontend-research",
      adapter: "codex",
      model: codex,
      scope: "reconnaissance.1/frontend.1",
      parent: "",
      created: at("2026-09-17T12:00:41"),
    },
    "sprint-planning.1": {
      run,
      id: "sprint-planning.1",
      name: "sprint-planning",
      adapter: "claude",
      model: claude,
      scope: "",
      parent: "",
      created: at("2026-09-17T12:13:53"),
    },
    "coding.1": {
      run,
      id: "coding.1",
      name: "coding",
      adapter: "claude",
      model: claude,
      scope: "",
      parent: "",
      created: at("2026-09-17T12:13:53"),
    },
    "implementation-scope-review.1": {
      run,
      id: "implementation-scope-review.1",
      name: "implementation-scope-review",
      adapter: "codex",
      model: codex,
      scope: "",
      parent: "",
      created: at("2026-09-17T12:13:53"),
    },
    "architectural-critique.1": {
      run,
      id: "architectural-critique.1",
      name: "architectural-critique",
      adapter: "codex",
      model: codex,
      scope: "",
      parent: "",
      created: at("2026-09-17T12:13:53"),
    },
    "implementation.1/task.1/qa-orchestration.1": {
      run,
      id: "implementation.1/task.1/qa-orchestration.1",
      name: "qa-orchestration",
      adapter: "claude",
      model: claude,
      scope: "implementation.1/task.1",
      parent: "",
      created: at("2026-09-17T12:26:12"),
    },
    "implementation.1/task.2/qa-orchestration.1": {
      run,
      id: "implementation.1/task.2/qa-orchestration.1",
      name: "qa-orchestration",
      adapter: "claude",
      model: claude,
      scope: "implementation.1/task.2",
      parent: "",
      created: at("2026-09-17T12:43:48"),
    },
  },
  interviews: {},
  turns: {
    "reconnaissance.1/backend.1/api-research.1/turn.1": {
      run,
      id: "reconnaissance.1/backend.1/api-research.1/turn.1",
      session: "reconnaissance.1/backend.1/api-research.1",
      scope: "reconnaissance.1/backend.1",
      prompt: backendResearchPrompt,
      output_type: "gimble.Text",
      result:
        "backend/index.md records the observation rows, the skgo remote functions and their provenance.",
      error: "",
      interrupted: false,
      started: at("2026-09-17T12:00:42"),
      ended: at("2026-09-17T12:12:18"),
      duration: 696000,
    },
    "reconnaissance.1/frontend.1/frontend-research.1/turn.1": {
      run,
      id: "reconnaissance.1/frontend.1/frontend-research.1/turn.1",
      session: "reconnaissance.1/frontend.1/frontend-research.1",
      scope: "reconnaissance.1/frontend.1",
      prompt: frontendResearchPrompt,
      output_type: "gimble.Text",
      result:
        "frontend/index.md records the Svelte 5 runes, the shadcn-svelte components installed here and their provenance.",
      error: "",
      interrupted: false,
      started: at("2026-09-17T12:00:42"),
      ended: at("2026-09-17T12:13:50"),
      duration: 788000,
    },
    "sprint-planning.1/turn.1": {
      run,
      id: "sprint-planning.1/turn.1",
      session: "sprint-planning.1",
      scope: "implementation.1",
      prompt: loopGoal,
      output_type: "gimble.answer",
      result: assignments[0].name,
      error: "",
      interrupted: false,
      started: at("2026-09-17T12:13:55"),
      ended: at("2026-09-17T12:14:31"),
      duration: 36000,
    },
    "sprint-planning.1/turn.2": {
      run,
      id: "sprint-planning.1/turn.2",
      session: "sprint-planning.1",
      scope: "implementation.1",
      prompt: loopGoal,
      output_type: "gimble.answer",
      result: assignments[1].name,
      error: "",
      interrupted: false,
      started: at("2026-09-17T12:29:42"),
      ended: at("2026-09-17T12:30:15"),
      duration: 33000,
    },
    "sprint-planning.1/turn.3": {
      run,
      id: "sprint-planning.1/turn.3",
      session: "sprint-planning.1",
      scope: "implementation.1",
      prompt: loopGoal,
      output_type: "gimble.answer",
      result: assignments[2].name,
      error: "",
      interrupted: false,
      started: at("2026-09-17T12:46:32"),
      ended: at("2026-09-17T12:46:54"),
      duration: 22000,
    },
    "coding.1/turn.1": {
      run,
      id: "coding.1/turn.1",
      session: "coding.1",
      scope: "implementation.1/task.1",
      prompt: codingTurnPrompt(assignments[0]),
      output_type: "gimble.Text",
      result: "Routed the accepted answer back to the waiting question.",
      error: "",
      interrupted: false,
      started: at("2026-09-17T12:14:35"),
      ended: at("2026-09-17T12:22:10"),
      duration: 455000,
    },
    "coding.1/turn.2": {
      run,
      id: "coding.1/turn.2",
      session: "coding.1",
      scope: "implementation.1/task.2",
      prompt: codingTurnPrompt(assignments[1]),
      output_type: "gimble.Text",
      result: "Served every pending question with the scope it waits in.",
      error: "",
      interrupted: false,
      started: at("2026-09-17T12:30:19"),
      ended: at("2026-09-17T12:39:48"),
      duration: 569000,
    },
    // The turn Main.html selects: running, 1m 48s in at 12:48:50.
    "coding.1/turn.3": {
      run,
      id: "coding.1/turn.3",
      session: "coding.1",
      scope: "implementation.1/task.3",
      prompt: codingTurnPrompt(assignments[2]),
      output_type: "gimble.Text",
      result: "",
      error: "",
      interrupted: false,
      started: at("2026-09-17T12:47:02"),
      ended: 0,
      duration: 0,
    },
    "implementation.1/task.1/qa-orchestration.1/turn.1": {
      run,
      id: "implementation.1/task.1/qa-orchestration.1/turn.1",
      session: "implementation.1/task.1/qa-orchestration.1",
      scope: "implementation.1/task.1",
      prompt: validationPrompt,
      output_type: "implementinterview.Assessment",
      result: "The answered row still carries no accepted time.",
      error: "",
      interrupted: false,
      started: at("2026-09-17T12:26:12"),
      ended: at("2026-09-17T12:29:38"),
      duration: 206000,
    },
    "implementation.1/task.2/qa-orchestration.1/turn.1": {
      run,
      id: "implementation.1/task.2/qa-orchestration.1/turn.1",
      session: "implementation.1/task.2/qa-orchestration.1",
      scope: "implementation.1/task.2",
      prompt: validationPrompt,
      output_type: "implementinterview.Assessment",
      result: "The answered row still carries no accepted time.",
      error: "",
      interrupted: false,
      started: at("2026-09-17T12:43:48"),
      ended: at("2026-09-17T12:46:28"),
      duration: 160000,
    },
    // The two watchers of the running coding call. One has finished its
    // look and one is looking now, which is the "2 · one looking now" the
    // Watchers disclosure states.
    "implementation-scope-review.1/turn.5": {
      run,
      id: "implementation-scope-review.1/turn.5",
      session: "implementation-scope-review.1",
      scope: "implementation.1/task.3",
      prompt: backendScopePrompt,
      output_type: "gimble.review",
      result: "Nothing outside issue 249 so far.",
      error: "",
      interrupted: false,
      started: at("2026-09-17T12:48:06"),
      ended: at("2026-09-17T12:48:20"),
      duration: 14000,
    },
    "architectural-critique.1/turn.6": {
      run,
      id: "architectural-critique.1/turn.6",
      session: "architectural-critique.1",
      scope: "implementation.1/task.3",
      prompt: architectureScopePrompt,
      output_type: "gimble.review",
      result: "",
      error: "",
      interrupted: false,
      started: at("2026-09-17T12:48:44"),
      ended: 0,
      duration: 0,
    },
  },
  // The selected turn's usage is the "18k in · 1.1k out" the Usage
  // disclosure states.
  turn_usage: {
    "coding.1/turn.1": { [claude]: usage({ input: 42118, output: 3204, cache_read: 118400 }) },
    "coding.1/turn.2": { [claude]: usage({ input: 51902, output: 4118, cache_read: 164210 }) },
    "coding.1/turn.3": { [claude]: usage({ input: 18412, output: 1104, cache_read: 92640 }) },
    "sprint-planning.1/turn.3": { [claude]: usage({ input: 9840, output: 612 }) },
    "implementation-scope-review.1/turn.5": { [codex]: usage({ input: 7210, output: 184 }) },
    "architectural-critique.1/turn.6": { [codex]: usage({ input: 6904, output: 0 }) },
  },
  model_calls: {
    "coding.1/turn.3": [
      {
        run,
        turn: "coding.1/turn.3",
        message: "msg_01M2RX4Aread",
        model: claude,
        input: 9210,
        cache_read: 46320,
        cache_write: 0,
        output: 618,
        reasoning: 0,
        started: at("2026-09-17T12:47:02"),
        ended: at("2026-09-17T12:47:38"),
      },
      {
        run,
        turn: "coding.1/turn.3",
        message: "msg_01M2RX4Aedit",
        model: claude,
        input: 9202,
        cache_read: 46320,
        cache_write: 0,
        output: 486,
        reasoning: 0,
        started: at("2026-09-17T12:47:41"),
        ended: at("2026-09-17T12:48:44"),
      },
    ],
  },
  // Task 3 has reached none of its commands, so the four command nodes on
  // its sheet have no row and read as "not yet".
  commands: {
    "implementation.1/task.1/task-check.1": {
      run,
      id: "implementation.1/task.1/task-check.1",
      scope: "implementation.1/task.1",
      name: "task-check",
      command: "sh",
      args: ["-lc", "just test"],
      workdir,
      exit_code: 0,
      stdout: "ok  \tgithub.com/tylergannon/gimble/internal/observation\t0.402s",
      stderr: "",
      stdout_file: "commands/implementation.1/task.1/task-check.1/stdout.log",
      stderr_file: "commands/implementation.1/task.1/task-check.1/stderr.log",
      error: "",
      interrupted: false,
      started: at("2026-09-17T12:22:12"),
      ended: at("2026-09-17T12:23:00"),
      duration: 48000,
    },
    "implementation.1/task.1/build.1": {
      run,
      id: "implementation.1/task.1/build.1",
      scope: "implementation.1/task.1",
      name: "build",
      command: "just",
      args: ["build"],
      workdir,
      exit_code: 0,
      stdout: "go build ./...",
      stderr: "",
      stdout_file: "commands/implementation.1/task.1/build.1/stdout.log",
      stderr_file: "commands/implementation.1/task.1/build.1/stderr.log",
      error: "",
      interrupted: false,
      started: at("2026-09-17T12:23:01"),
      ended: at("2026-09-17T12:23:42"),
      duration: 41000,
    },
    "implementation.1/task.1/vet.1": {
      run,
      id: "implementation.1/task.1/vet.1",
      scope: "implementation.1/task.1",
      name: "vet",
      command: "just",
      args: ["vet"],
      workdir,
      exit_code: 0,
      stdout: "go vet ./...",
      stderr: "",
      stdout_file: "commands/implementation.1/task.1/vet.1/stdout.log",
      stderr_file: "commands/implementation.1/task.1/vet.1/stderr.log",
      error: "",
      interrupted: false,
      started: at("2026-09-17T12:23:43"),
      ended: at("2026-09-17T12:24:21"),
      duration: 38000,
    },
    "implementation.1/task.1/test.1": {
      run,
      id: "implementation.1/task.1/test.1",
      scope: "implementation.1/task.1",
      name: "test",
      command: "just",
      args: ["test"],
      workdir,
      exit_code: 0,
      stdout: "ok  \tgithub.com/tylergannon/gimble/internal/observation\t0.402s",
      stderr: "",
      stdout_file: "commands/implementation.1/task.1/test.1/stdout.log",
      stderr_file: "commands/implementation.1/task.1/test.1/stderr.log",
      error: "",
      interrupted: false,
      started: at("2026-09-17T12:24:22"),
      ended: at("2026-09-17T12:26:10"),
      duration: 108000,
    },
    "implementation.1/task.2/task-check.1": {
      run,
      id: "implementation.1/task.2/task-check.1",
      scope: "implementation.1/task.2",
      name: "task-check",
      command: "sh",
      args: ["-lc", "just test"],
      workdir,
      exit_code: 0,
      stdout: "ok  \tgithub.com/tylergannon/gimble/internal/observation\t0.411s",
      stderr: "",
      stdout_file: "commands/implementation.1/task.2/task-check.1/stdout.log",
      stderr_file: "commands/implementation.1/task.2/task-check.1/stderr.log",
      error: "",
      interrupted: false,
      started: at("2026-09-17T12:39:50"),
      ended: at("2026-09-17T12:40:38"),
      duration: 48000,
    },
    "implementation.1/task.2/build.1": {
      run,
      id: "implementation.1/task.2/build.1",
      scope: "implementation.1/task.2",
      name: "build",
      command: "just",
      args: ["build"],
      workdir,
      exit_code: 0,
      stdout: "go build ./...",
      stderr: "",
      stdout_file: "commands/implementation.1/task.2/build.1/stdout.log",
      stderr_file: "commands/implementation.1/task.2/build.1/stderr.log",
      error: "",
      interrupted: false,
      started: at("2026-09-17T12:40:39"),
      ended: at("2026-09-17T12:41:20"),
      duration: 41000,
    },
    "implementation.1/task.2/vet.1": {
      run,
      id: "implementation.1/task.2/vet.1",
      scope: "implementation.1/task.2",
      name: "vet",
      command: "just",
      args: ["vet"],
      workdir,
      exit_code: 0,
      stdout: "go vet ./...",
      stderr: "",
      stdout_file: "commands/implementation.1/task.2/vet.1/stdout.log",
      stderr_file: "commands/implementation.1/task.2/vet.1/stderr.log",
      error: "",
      interrupted: false,
      started: at("2026-09-17T12:41:21"),
      ended: at("2026-09-17T12:41:58"),
      duration: 37000,
    },
    "implementation.1/task.2/test.1": {
      run,
      id: "implementation.1/task.2/test.1",
      scope: "implementation.1/task.2",
      name: "test",
      command: "just",
      args: ["test"],
      workdir,
      exit_code: 0,
      stdout: "ok  \tgithub.com/tylergannon/gimble/internal/observation\t0.411s",
      stderr: "",
      stdout_file: "commands/implementation.1/task.2/test.1/stdout.log",
      stderr_file: "commands/implementation.1/task.2/test.1/stderr.log",
      error: "",
      interrupted: false,
      started: at("2026-09-17T12:41:59"),
      ended: at("2026-09-17T12:43:46"),
      duration: 107000,
    },
  },
  totals: {
    scopes: {
      "": total(claude, { input: 214802, output: 14806, cache_read: 604210 }),
      "reconnaissance.1": total(codex, { input: 48210, output: 3902 }),
      "reconnaissance.1/backend.1": total(codex, { input: 22104, output: 1840 }),
      "reconnaissance.1/frontend.1": total(codex, { input: 26106, output: 2062 }),
      "implementation.1": total(claude, { input: 152480, output: 10212, cache_read: 512480 }),
      "implementation.1/task.1": total(claude, { input: 61208, output: 4416, cache_read: 208640 }),
      "implementation.1/task.2": total(claude, { input: 68744, output: 5088, cache_read: 260170 }),
      "implementation.1/task.3": total(claude, { input: 32526, output: 1288, cache_read: 92640 }),
    },
    sessions: {
      "coding.1": total(claude, { input: 112432, output: 8426, cache_read: 423370 }),
      "sprint-planning.1": total(claude, { input: 28104, output: 1806 }),
      "implementation-scope-review.1": total(codex, { input: 31408, output: 902 }),
      "architectural-critique.1": total(codex, { input: 29806, output: 864 }),
    },
  },
  // The running turn's transcript: the nine tool calls the Activity section
  // counts, the first three of which it lists.
  transcripts: {
    "coding.1/turn.3": {
      snapshot: {
        state: {
          info: {
            [nativeCoding]: { id: nativeCoding, title: "Answered timestamp" },
          },
          family: { [nativeCoding]: [nativeCoding] },
          active: { [nativeCoding]: "running" },
          message: {
            [nativeCoding]: [
              {
                id: "coding.1/turn.3/prompt",
                type: "user",
                time: { created: at("2026-09-17T12:47:02") },
                text: assignments[2].description,
              },
              {
                id: "msg_01M2RX4Acoding3",
                type: "assistant",
                time: { created: at("2026-09-17T12:47:02") },
                content: [
                  toolCall(
                    "tool-1",
                    "Read",
                    { file_path: "web/src/lib/workflow/types.ts" },
                    "2026-09-17T12:47:08",
                  ),
                  toolCall(
                    "tool-2",
                    "Edit",
                    { file_path: "web/src/routes/runs/[id]/+page.svelte" },
                    "2026-09-17T12:47:52",
                  ),
                  toolCall(
                    "tool-3",
                    "Bash",
                    { command: "go test ./internal/observation/" },
                    "2026-09-17T12:48:02",
                  ),
                  toolCall(
                    "tool-4",
                    "Read",
                    { file_path: "internal/observation/rows.go" },
                    "2026-09-17T12:48:11",
                  ),
                  toolCall(
                    "tool-5",
                    "Edit",
                    { file_path: "internal/observation/store.go" },
                    "2026-09-17T12:48:19",
                  ),
                  toolCall(
                    "tool-6",
                    "Read",
                    { file_path: "web/src/lib/observation/index.ts" },
                    "2026-09-17T12:48:26",
                  ),
                  toolCall(
                    "tool-7",
                    "Edit",
                    { file_path: "web/src/lib/observation/index.ts" },
                    "2026-09-17T12:48:34",
                  ),
                  toolCall("tool-8", "Bash", { command: "go build ./..." }, "2026-09-17T12:48:44"),
                  toolCall("tool-9", "Bash", { command: "just test" }),
                ],
                tokens: {
                  input: 18412,
                  output: 1104,
                  reasoning: 0,
                  cache: { read: 92640, write: 0 },
                },
                cost: 0,
              },
            ],
          },
          pending: {},
          permission: {},
          form: {},
        },
      },
      provenance: {
        msg_01M2RX4Acoding3: {
          provider: "claude",
          sessionID: nativeCoding,
          turnID: "coding.1/turn.3",
          normalizedMessageID: "msg_01M2RX4Acoding3",
        },
      },
    },
  },
};

/** Main.html: the graph and the snapshot the map is drawn from. */
export const implementInterviewRun = {
  graph: implementInterviewGraph,
  snapshot: implementInterviewSnapshot,
};
