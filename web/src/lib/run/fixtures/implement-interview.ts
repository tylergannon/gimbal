import type {
  CommandRow,
  InterviewRow,
  ModelCallRow,
  RunRow,
  RunSnapshot,
  ScopeRow,
  SessionRow,
  TurnRow,
  Usage,
} from "../../observation/index.js";
import type { Graph } from "../../workflow/types.js";

const workflowFile = "internal/workflows/implementinterview/implementinterview.go";

export const implementInterviewGraph: Graph = {
  name: "implement-interview",
  source: { file: workflowFile, line: 45 },
  body: [
    {
      kind: "condition",
      file: workflowFile,
      line: 46,
      branches: [
        {
          file: workflowFile,
          line: 46,
          case: "!filepath.IsAbs(params.RequirementsFile) || !filepath.IsAbs(params.ReferenceDir)",
          exits: true,
          body: [],
        },
      ],
    },
    {
      kind: "condition",
      file: workflowFile,
      line: 49,
      branches: [
        { file: workflowFile, line: 49, case: "params.MaxTasks < 1", exits: true, body: [] },
      ],
    },
    {
      kind: "condition",
      file: workflowFile,
      line: 55,
      branches: [
        {
          file: workflowFile,
          line: 55,
          case: "info, err := os.Stat(params.ReferenceDir); err != nil || !info.IsDir()",
          exits: true,
          body: [],
        },
      ],
    },
    { kind: "set", file: workflowFile, line: 59, key: "requirements-file" },
    { kind: "set", file: workflowFile, line: 60, key: "reference-directory" },
    { kind: "set", file: workflowFile, line: 61, key: "repository" },
    {
      kind: "group",
      file: workflowFile,
      line: 63,
      name: "reconnaissance",
      children: [
        {
          file: workflowFile,
          line: 64,
          name: "backend",
          body: [
            { kind: "session", file: workflowFile, line: 65, name: "api-research", from: "" },
            {
              kind: "agent_call",
              file: workflowFile,
              line: 66,
              session: "api-research",
              role: "api-research",
              prompt:
                "Read the saved issue, current Go API and design docs, pinned dependencies, runtime/events/generation code, Polytype use, and skgo Go remote-function APIs. Do not edit source. Under the reference directory, create backend/index.md with deep implementation notes and provenance, and save useful official source documentation under backend/. Distinguish verified facts from proposals.",
              supervisors: [],
            },
          ],
        },
        {
          file: workflowFile,
          line: 69,
          name: "frontend",
          body: [
            {
              kind: "session",
              file: workflowFile,
              line: 70,
              name: "frontend-research",
              from: "",
            },
            {
              kind: "agent_call",
              file: workflowFile,
              line: 71,
              session: "frontend-research",
              role: "frontend-research",
              prompt:
                "Read the saved issue, current live-run UI, pinned frontend dependencies, and installed shadcn-svelte components. Research the actual Svelte, SvelteKit, shadcn-svelte, and Bits UI APIs needed by the issue. Do not edit source. Under the reference directory, create frontend/index.md with deep implementation notes and provenance, and save useful official source documentation under frontend/. Distinguish verified facts from proposals.",
              supervisors: [],
            },
          ],
        },
      ],
    },
    { kind: "session", file: workflowFile, line: 78, name: "sprint-planning", from: "" },
    { kind: "session", file: workflowFile, line: 79, name: "coding", from: "" },
    {
      kind: "session",
      file: workflowFile,
      line: 80,
      name: "implementation-scope-review",
      from: "",
    },
    {
      kind: "session",
      file: workflowFile,
      line: 81,
      name: "architectural-critique",
      from: "",
    },
    {
      kind: "promise_loop",
      file: workflowFile,
      line: 84,
      name: "implementation",
      planner: "sprint-planning",
      supervisors: [],
      body: [
        {
          kind: "agent_call",
          file: workflowFile,
          line: 92,
          session: "coding",
          role: "coding",
          prompt:
            "Read the saved issue and the local reference directory, then implement only the selected task in the repository. Do not expand the specification, modify the requirements or this build workflow, weaken its fixed checks, commit, merge, or add proof scripts or run output. Preserve unrelated work. Answer with what changed and what you personally ran or observed.",
          supervisors: [
            {
              file: workflowFile,
              line: 93,
              session: "implementation-scope-review",
              role: "implementation-scope-review",
              instruction:
                "Watch only for concrete backend or public-API work beyond the saved issue 249. Steer against unnecessary wrappers, frameworks, speculative APIs, and unrelated features. Do not edit source, object on style, or demand improvements outside the requirements.",
              supervisors: [],
            },
            {
              file: workflowFile,
              line: 94,
              session: "architectural-critique",
              role: "architectural-critique",
              instruction:
                "Watch only for concrete frontend or general implementation work beyond the saved issue 249. Steer against unnecessary abstractions, frameworks, speculative features, and unrelated polish. Do not edit source, object on style, or demand improvements outside the requirements.",
              supervisors: [],
            },
          ],
        },
        {
          kind: "condition",
          file: workflowFile,
          line: 102,
          branches: [
            {
              file: workflowFile,
              line: 102,
              case: 'command := strings.TrimSpace(task.Validation.Command); command != ""',
              exits: false,
              body: [
                { kind: "command", file: workflowFile, line: 103, name: "task-check" },
                { kind: "set", file: workflowFile, line: 104, key: "task check" },
              ],
            },
          ],
        },
        { kind: "command", file: workflowFile, line: 112, name: "build" },
        { kind: "set", file: workflowFile, line: 113, key: "just build" },
        { kind: "command", file: workflowFile, line: 118, name: "vet" },
        { kind: "set", file: workflowFile, line: 119, key: "just vet" },
        { kind: "command", file: workflowFile, line: 124, name: "test" },
        { kind: "set", file: workflowFile, line: 125, key: "just test" },
        {
          kind: "session",
          file: workflowFile,
          line: 131,
          name: "qa-orchestration",
          from: "",
        },
        {
          kind: "agent_call",
          file: workflowFile,
          line: 132,
          session: "qa-orchestration",
          role: "qa-orchestration",
          prompt:
            "Independently read the saved issue, reference directory, implementation, tests, and recorded command results. Make no source changes and do not treat the worker report as evidence. Start the generated interview example on a separate port with its interviewer bound to Terra or Sonnet, never Gemini. Use the installed Playwright through the shell to conduct successive human answers in a real browser; do not write proof programs or commit run artifacts, and cleanly stop the example process afterward. Complete is true only if you personally demonstrate every requirement in that running interface; tests and reports alone are insufficient.",
          supervisors: [],
        },
        { kind: "set", file: workflowFile, line: 137, key: "independent assessment" },
        { kind: "set", file: workflowFile, line: 138, key: "worker report" },
        { kind: "set", file: workflowFile, line: 140, key: "deterministic checks passed" },
        {
          kind: "condition",
          file: workflowFile,
          line: 141,
          branches: [
            {
              file: workflowFile,
              line: 141,
              case: "checksPassed && assessment.Complete",
              exits: true,
              body: [],
            },
          ],
        },
        {
          kind: "condition",
          file: workflowFile,
          line: 145,
          branches: [
            {
              file: workflowFile,
              line: 145,
              case: "tasksRun >= params.MaxTasks",
              exits: true,
              body: [],
            },
          ],
        },
      ],
    },
    {
      kind: "condition",
      file: workflowFile,
      line: 156,
      branches: [{ file: workflowFile, line: 156, case: "completed", exits: true, body: [] }],
    },
    {
      kind: "condition",
      file: workflowFile,
      line: 159,
      branches: [{ file: workflowFile, line: 159, case: "exhausted", exits: true, body: [] }],
    },
  ],
  diagnostics: [],
};

const runID = "01M2RX4A9Q0ZT7V4C3D8N1KMB2";
const started = 1_800_000_000_000;

const run: RunRow = {
  id: runID,
  name: "implement-interview",
  status: "running",
  error: "",
  started,
  ended: 0,
};

const scopes = {
  "": {
    run: runID,
    key: "",
    name: "implement-interview",
    loop: false,
    status: "running",
    error: "",
    began: started,
    ended: 0,
    values: {},
    decisions: [],
  },
  "reconnaissance.1": {
    run: runID,
    key: "reconnaissance.1",
    name: "reconnaissance",
    loop: false,
    status: "ended",
    error: "",
    began: started + 10_000,
    ended: started + 312_000,
    values: {},
    decisions: [],
  },
  "reconnaissance.1/backend.1": {
    run: runID,
    key: "reconnaissance.1/backend.1",
    name: "backend",
    loop: false,
    status: "ended",
    error: "",
    began: started + 12_000,
    ended: started + 306_000,
    values: {},
    decisions: [],
  },
  "reconnaissance.1/frontend.1": {
    run: runID,
    key: "reconnaissance.1/frontend.1",
    name: "frontend",
    loop: false,
    status: "ended",
    error: "",
    began: started + 12_000,
    ended: started + 312_000,
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
    began: started + 320_000,
    ended: 0,
    values: {},
    decisions: [
      { seq: 1, body: { task: 1, name: "Observation timestamp" } },
      { seq: 2, body: { task: 2, name: "Answer endpoint timestamp" } },
      { seq: 3, body: { task: 3, name: "Live answered timestamp" } },
    ],
  },
  "implementation.1/task.1": {
    run: runID,
    key: "implementation.1/task.1",
    name: "task",
    loop: false,
    status: "ended",
    error: "",
    task: {
      name: "Observation timestamp",
      description: "Add answered timestamps to the observation rows.",
    },
    began: started + 410_000,
    ended: started + 1_120_000,
    values: {},
    decisions: [],
  },
  "implementation.1/task.2": {
    run: runID,
    key: "implementation.1/task.2",
    name: "task",
    loop: false,
    status: "ended",
    error: "",
    task: {
      name: "Answer endpoint timestamp",
      description: "Return the answered timestamp through the answer endpoint.",
    },
    began: started + 1_130_000,
    ended: started + 2_630_000,
    values: {},
    decisions: [],
  },
  "implementation.1/task.3": {
    run: runID,
    key: "implementation.1/task.3",
    name: "task",
    loop: false,
    status: "running",
    error: "",
    task: {
      name: "Live answered timestamp",
      description:
        "Wire the answered timestamp through the web answer flow so the interview row updates without a reload. Stay inside the observation rows.",
    },
    began: started + 2_640_000,
    ended: 0,
    values: {},
    decisions: [],
  },
} satisfies Record<string, ScopeRow>;

const sessions = {
  "reconnaissance.1/backend.1/api-research.1": {
    run: runID,
    id: "reconnaissance.1/backend.1/api-research.1",
    name: "api-research",
    adapter: "codex",
    model: "gpt-5.6-luna",
    scope: "reconnaissance.1/backend.1",
    parent: "",
    created: started + 16_000,
  },
  "reconnaissance.1/frontend.1/frontend-research.1": {
    run: runID,
    id: "reconnaissance.1/frontend.1/frontend-research.1",
    name: "frontend-research",
    adapter: "codex",
    model: "gpt-5.6-luna",
    scope: "reconnaissance.1/frontend.1",
    parent: "",
    created: started + 16_000,
  },
  "sprint-planning.1": {
    run: runID,
    id: "sprint-planning.1",
    name: "sprint-planning",
    adapter: "codex",
    model: "gpt-6-astra",
    scope: "",
    parent: "",
    created: started + 320_000,
  },
  "coding.1": {
    run: runID,
    id: "coding.1",
    name: "coding",
    adapter: "codex",
    model: "gpt-5.6-luna",
    scope: "",
    parent: "",
    created: started + 410_000,
  },
  "implementation-scope-review.1": {
    run: runID,
    id: "implementation-scope-review.1",
    name: "implementation-scope-review",
    adapter: "codex",
    model: "gpt-5.6-luna",
    scope: "",
    parent: "",
    created: started + 410_000,
  },
  "architectural-critique.1": {
    run: runID,
    id: "architectural-critique.1",
    name: "architectural-critique",
    adapter: "codex",
    model: "gpt-5.6-luna",
    scope: "",
    parent: "",
    created: started + 410_000,
  },
} satisfies Record<string, SessionRow>;

const turns = {
  "coding.1/turn.1": {
    run: runID,
    id: "coding.1/turn.1",
    session: "coding.1",
    scope: "implementation.1/task.1",
    prompt: "Implement task 1 of 3: add answered timestamps to the observation rows.",
    output_type: "text",
    result: "Added the observation timestamp and its focused tests.",
    error: "",
    interrupted: false,
    started: started + 410_000,
    ended: started + 1_020_000,
    duration: 610_000,
  },
  "coding.1/turn.2": {
    run: runID,
    id: "coding.1/turn.2",
    session: "coding.1",
    scope: "implementation.1/task.2",
    prompt: "Implement task 2 of 3: return the answered timestamp through the answer endpoint.",
    output_type: "text",
    result: "Returned the timestamp through the existing remote boundary.",
    error: "",
    interrupted: false,
    started: started + 1_130_000,
    ended: started + 2_520_000,
    duration: 1_390_000,
  },
  "coding.1/turn.3": {
    run: runID,
    id: "coding.1/turn.3",
    session: "coding.1",
    scope: "implementation.1/task.3",
    prompt:
      "Wire the answered timestamp through the web answer flow so the interview row updates without a reload. Stay inside the observation rows.",
    output_type: "text",
    result: "",
    error: "",
    interrupted: false,
    started: started + 2_784_000,
    ended: 0,
    duration: 108_000,
  },
  "implementation-scope-review.1/turn.3": {
    run: runID,
    id: "implementation-scope-review.1/turn.3",
    session: "implementation-scope-review.1",
    scope: "implementation.1/task.3",
    prompt: "Review task 3 for backend or public-API scope expansion.",
    output_type: "text",
    result: "No backend or public-API scope expansion observed.",
    error: "",
    interrupted: false,
    started: started + 2_790_000,
    ended: started + 2_850_000,
    duration: 60_000,
  },
  "architectural-critique.1/turn.3": {
    run: runID,
    id: "architectural-critique.1/turn.3",
    session: "architectural-critique.1",
    scope: "implementation.1/task.3",
    prompt: "Review task 3 for frontend or general implementation scope expansion.",
    output_type: "text",
    result: "",
    error: "",
    interrupted: false,
    started: started + 2_805_000,
    ended: 0,
    duration: 87_000,
  },
} satisfies Record<string, TurnRow>;

const usage: Usage = {
  input: 18_000,
  cache_read: 0,
  cache_write: 0,
  output: 1_100,
  reasoning: 0,
  stated_cost: 0,
};

const modelCalls = {
  "coding.1/turn.3": [
    {
      run: runID,
      turn: "coding.1/turn.3",
      message: "coding-message-3",
      model: "gpt-5.6-luna",
      input: 18_000,
      cache_read: 0,
      cache_write: 0,
      output: 1_100,
      reasoning: 0,
      started: started + 2_784_000,
      ended: 0,
    },
  ],
} satisfies Record<string, ModelCallRow[]>;

const interviews = {} satisfies Record<string, InterviewRow>;
const commands = {} satisfies Record<string, CommandRow>;

export const implementInterviewSnapshot: RunSnapshot = {
  stream: "implement-interview-specimen",
  position: 38,
  run,
  scopes,
  sessions,
  interviews,
  turns,
  turn_usage: { "coding.1/turn.3": { "gpt-5.6-luna": usage } },
  model_calls: modelCalls,
  commands,
  totals: {
    scopes: {
      "implementation.1/task.3": { all: usage, by_model: { "gpt-5.6-luna": usage } },
    },
    sessions: { "coding.1": { all: usage, by_model: { "gpt-5.6-luna": usage } } },
  },
  transcripts: {},
};

export const implementInterviewFixture = {
  graph: implementInterviewGraph,
  snapshot: implementInterviewSnapshot,
};
