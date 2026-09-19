import type {
  CommandRow,
  InterviewRow,
  ModelCallRow,
  RunRow,
  RunSnapshot,
  ScopeRow,
  SessionRow,
  TurnRow,
} from "../../observation/index.js";
import type { Graph } from "../../workflow/types.js";

const workflowFile = "examples/plantrip/plantrip.go";

export const planTripGraph: Graph = {
  name: "plan-trip",
  source: { file: workflowFile, line: 20 },
  services: [],
  body: [
    {
      kind: "group",
      file: workflowFile,
      line: 22,
      name: "research",
      children: [
        {
          file: workflowFile,
          line: 23,
          name: "lodging",
          services: [],
          body: [
            { kind: "session", file: workflowFile, line: 24, name: "interviewer", from: "" },
            {
              kind: "interview",
              file: workflowFile,
              line: 31,
              name: "preferences",
              session: "interviewer",
            },
          ],
        },
        {
          file: workflowFile,
          line: 36,
          name: "transport",
          services: [],
          body: [
            { kind: "session", file: workflowFile, line: 37, name: "interviewer", from: "" },
            {
              kind: "interview",
              file: workflowFile,
              line: 44,
              name: "preferences",
              session: "interviewer",
            },
          ],
        },
      ],
    },
    { kind: "session", file: workflowFile, line: 49, name: "planner", from: "" },
    {
      kind: "agent_call",
      file: workflowFile,
      line: 50,
      session: "planner",
      role: "",
      prompt: "Plan the weekend trip from the lodging and transport preferences.",
      supervisors: [],
    },
  ],
  diagnostics: [],
};

const runID = "01M2RWXNFFYQA4HHQWMQ252CYF";
const started = 1_800_000_000_000;

const run: RunRow = {
  id: runID,
  name: "plan-trip",
  status: "running",
  error: "",
  started,
  ended: 0,
};

const scopes = {
  "": {
    run: runID,
    key: "",
    name: "plan-trip",
    loop: false,
    status: "running",
    error: "",
    began: started,
    ended: 0,
    values: {},
    decisions: [],
  },
  "research.1": {
    run: runID,
    key: "research.1",
    name: "research",
    loop: false,
    status: "running",
    error: "",
    began: started + 5_000,
    ended: 0,
    values: {},
    decisions: [],
  },
  "research.1/lodging.1": {
    run: runID,
    key: "research.1/lodging.1",
    name: "lodging",
    loop: false,
    status: "running",
    error: "",
    began: started + 7_000,
    ended: 0,
    values: {
      "previous answer": {
        value: "A quiet mountain cabin, easy walks, and no crowds.",
      },
    },
    decisions: [],
  },
  "research.1/transport.1": {
    run: runID,
    key: "research.1/transport.1",
    name: "transport",
    loop: false,
    status: "running",
    error: "",
    began: started + 7_000,
    ended: 0,
    values: {},
    decisions: [],
  },
} satisfies Record<string, ScopeRow>;

const sessions = {
  "research.1/lodging.1/interviewer.1": {
    run: runID,
    id: "research.1/lodging.1/interviewer.1",
    name: "interviewer",
    adapter: "claude",
    model: "claude-sonnet-4-5",
    scope: "research.1/lodging.1",
    parent: "",
    created: started + 10_000,
  },
  "research.1/transport.1/interviewer.1": {
    run: runID,
    id: "research.1/transport.1/interviewer.1",
    name: "interviewer",
    adapter: "claude",
    model: "claude-sonnet-4-5",
    scope: "research.1/transport.1",
    parent: "",
    created: started + 10_000,
  },
} satisfies Record<string, SessionRow>;

const interviews = {
  "01M2RWXNFFYQA4HHQWMQ252CYA": {
    run: runID,
    question_id: "01M2RWXNFFYQA4HHQWMQ252CYA",
    name: "preferences",
    scope: "research.1/lodging.1",
    session: "research.1/lodging.1/interviewer.1",
    question: "What would make a weekend trip feel restful to you?",
    status: "answered",
    answer: "A quiet mountain cabin, easy walks, and no crowds.",
    asked: started + 12_000,
    answered: started + 22_000,
  },
  "01M2RWXNFFYQA4HHQWMQ252CYB": {
    run: runID,
    question_id: "01M2RWXNFFYQA4HHQWMQ252CYB",
    name: "preferences",
    scope: "research.1/transport.1",
    session: "research.1/transport.1/interviewer.1",
    question: "Would you rather drive or take a train?",
    status: "answered",
    answer: "Drive, if the route itself is pleasant.",
    asked: started + 14_000,
    answered: started + 24_000,
  },
  "01M2RWXNFFYQA4HHQWMQ252CYF": {
    run: runID,
    question_id: "01M2RWXNFFYQA4HHQWMQ252CYF",
    name: "preferences",
    scope: "research.1/lodging.1",
    session: "research.1/lodging.1/interviewer.1",
    question: "Would you trade reliable Wi-Fi for a more secluded cabin?",
    status: "pending",
    answer: "",
    asked: started + 30_000,
    answered: 0,
  },
  "01M2RWXNFFYQA4HHQWMQ252CYG": {
    run: runID,
    question_id: "01M2RWXNFFYQA4HHQWMQ252CYG",
    name: "preferences",
    scope: "research.1/transport.1",
    session: "research.1/transport.1/interviewer.1",
    question: "What is the longest drive you would enjoy?",
    status: "pending",
    answer: "",
    asked: started + 35_000,
    answered: 0,
  },
} satisfies Record<string, InterviewRow>;

const turns = {} satisfies Record<string, TurnRow>;
const modelCalls = {} satisfies Record<string, ModelCallRow[]>;
const commands = {} satisfies Record<string, CommandRow>;

export const planTripSnapshot: RunSnapshot = {
  stream: "plan-trip-specimen",
  position: 12,
  run,
  scopes,
  sessions,
  interviews,
  turns,
  turn_usage: {},
  model_calls: modelCalls,
  commands,
  totals: { scopes: {}, sessions: {} },
  transcripts: {},
};

export const planTripFixture = {
  graph: planTripGraph,
  snapshot: planTripSnapshot,
  priorExchange: {
    question: "What would make a weekend trip feel restful to you?",
    answer: "A quiet mountain cabin, easy walks, and no crowds.",
  },
};
