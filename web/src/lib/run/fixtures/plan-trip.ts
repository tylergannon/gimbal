import type { InterviewRow, RunSnapshot } from "../../observation/index.js";
import type { Graph } from "../../workflow/types.js";
import { at, total, usage } from "./support.js";

const file = "internal/workflows/plantrip/plantrip.go";

const lodgingPurpose =
  "Learn what would make a weekend away feel restful, and what the person will trade for it.";
const transportPurpose =
  "Learn how far the person is willing to travel and how they would rather get there.";
const plannerPrompt =
  "Read both preference interviews and propose one weekend itinerary: where to stay, how to get there, and what to do on each day. Name the trade-offs the answers made explicit.";

/** plan-trip's shape: one group whose two children each hold their own
 * interviewer and ask the same named interview, then the planner that reads
 * both. The group is the scope the two legs sit in, so their runtime keys
 * are `research.1/lodging.1` and `research.1/transport.1`. */
export const planTripGraph: Graph = {
  name: "plan-trip",
  source: { file, line: 18 },
  body: [
    {
      kind: "group",
      file,
      line: 22,
      name: "research",
      children: [
        {
          file,
          line: 23,
          name: "lodging",
          body: [
            { kind: "session", file, line: 24, name: "interviewer", from: "" },
            { kind: "interview", file, line: 31, name: "preferences", session: "interviewer" },
          ],
        },
        {
          file,
          line: 35,
          name: "transport",
          body: [
            { kind: "session", file, line: 36, name: "interviewer", from: "" },
            { kind: "interview", file, line: 42, name: "preferences", session: "interviewer" },
          ],
        },
      ],
    },
    { kind: "session", file, line: 47, name: "planner", from: "" },
    {
      kind: "agent_call",
      file,
      line: 48,
      session: "planner",
      role: "planner",
      prompt: plannerPrompt,
      supervisors: [],
    },
  ],
  diagnostics: [],
};

const run = "01M2RWXNFFYQA4HHQWMQ252CYF";
const sonnet = "claude-fable-5-1";

/** The two questions the run is waiting on, newest first. Runs.html lists
 * them as its attention items and Interview.html opens the first. */
export const planTripPending: InterviewRow[] = [
  {
    run,
    question_id: "01M2RWY8K3VQ2D7N5R1T9B4XZM",
    name: "preferences",
    scope: "research.1/lodging.1",
    session: "research.1/lodging.1/interviewer.1",
    question: "Would you trade reliable Wi-Fi for a more secluded cabin?",
    status: "pending",
    answer: "",
    asked: at("2026-09-17T12:48:10"),
    answered: 0,
  },
  {
    run,
    question_id: "01M2RWY9A6HJ8F0S3C2W7K5NPQ",
    name: "preferences",
    scope: "research.1/transport.1",
    session: "research.1/transport.1/interviewer.1",
    question: "What is the longest drive you would enjoy?",
    status: "pending",
    answer: "",
    asked: at("2026-09-17T12:48:15"),
    answered: 0,
  },
];

/** plan-trip as Interview.html draws it: twelve minutes in, both legs of the
 * research group waiting on their second question, and the planner not
 * started. The lodging leg's first exchange is answered, which is what the
 * inspector reads out above the waiting question.
 *
 * The clock is the one implement-interview runs on, so the runs list can
 * order the two live runs against each other: at 12:48:50 this run is 12m
 * 40s old and its two questions have waited 40 s and 35 s. */
export const planTripSnapshot: RunSnapshot = {
  stream: "01M2RWXPD2GK6M4V8Z1Y5H3TBW",
  position: 96,
  run: {
    id: run,
    name: "plan-trip",
    status: "running",
    error: "",
    started: at("2026-09-17T12:36:10"),
    ended: 0,
  },
  scopes: {
    // path.Base("") is ".", the name the runtime writes for the root scope.
    "": {
      run,
      key: "",
      name: ".",
      loop: false,
      status: "running",
      error: "",
      began: at("2026-09-17T12:36:10"),
      ended: 0,
      values: {},
      decisions: [],
    },
    "research.1": {
      run,
      key: "research.1",
      name: "research",
      loop: false,
      status: "running",
      error: "",
      began: at("2026-09-17T12:36:11"),
      ended: 0,
      values: {},
      decisions: [],
    },
    "research.1/lodging.1": {
      run,
      key: "research.1/lodging.1",
      name: "lodging",
      loop: false,
      status: "running",
      error: "",
      began: at("2026-09-17T12:36:12"),
      ended: 0,
      values: {},
      decisions: [],
    },
    "research.1/transport.1": {
      run,
      key: "research.1/transport.1",
      name: "transport",
      loop: false,
      status: "running",
      error: "",
      began: at("2026-09-17T12:36:12"),
      ended: 0,
      values: {},
      decisions: [],
    },
  },
  sessions: {
    "research.1/lodging.1/interviewer.1": {
      run,
      id: "research.1/lodging.1/interviewer.1",
      name: "interviewer",
      adapter: "claude",
      model: sonnet,
      scope: "research.1/lodging.1",
      parent: "",
      created: at("2026-09-17T12:36:12"),
    },
    "research.1/transport.1/interviewer.1": {
      run,
      id: "research.1/transport.1/interviewer.1",
      name: "interviewer",
      adapter: "claude",
      model: sonnet,
      scope: "research.1/transport.1",
      parent: "",
      created: at("2026-09-17T12:36:12"),
    },
  },
  interviews: {
    "01M2RWY1P8CNZ4G6L0M2V7XDKS": {
      run,
      question_id: "01M2RWY1P8CNZ4G6L0M2V7XDKS",
      name: "preferences",
      scope: "research.1/lodging.1",
      session: "research.1/lodging.1/interviewer.1",
      question: "What would make a weekend trip feel restful to you?",
      status: "answered",
      answer: "A quiet mountain cabin, easy walks, and no crowds.",
      asked: at("2026-09-17T12:46:14"),
      answered: at("2026-09-17T12:47:40"),
    },
    "01M2RWY2R4TBXH9J7Q5F1D8WLC": {
      run,
      question_id: "01M2RWY2R4TBXH9J7Q5F1D8WLC",
      name: "preferences",
      scope: "research.1/transport.1",
      session: "research.1/transport.1/interviewer.1",
      question: "How do you like to travel when the trip itself should be restful?",
      status: "answered",
      answer: "By car, as long as someone else can take a turn at the wheel.",
      asked: at("2026-09-17T12:46:20"),
      answered: at("2026-09-17T12:47:55"),
    },
    [planTripPending[0].question_id]: planTripPending[0],
    [planTripPending[1].question_id]: planTripPending[1],
  },
  turns: {
    "research.1/lodging.1/interviewer.1/turn.1": {
      run,
      id: "research.1/lodging.1/interviewer.1/turn.1",
      session: "research.1/lodging.1/interviewer.1",
      scope: "research.1/lodging.1",
      prompt: lodgingPurpose,
      output_type: "gimble.interviewDecision",
      result: "What would make a weekend trip feel restful to you?",
      error: "",
      interrupted: false,
      started: at("2026-09-17T12:45:52"),
      ended: at("2026-09-17T12:46:14"),
      duration: 22000,
    },
    // The turn that produced the waiting question: the inspector reads its
    // 18 seconds of thinking out beside the first exchange.
    "research.1/lodging.1/interviewer.1/turn.2": {
      run,
      id: "research.1/lodging.1/interviewer.1/turn.2",
      session: "research.1/lodging.1/interviewer.1",
      scope: "research.1/lodging.1",
      prompt: lodgingPurpose,
      output_type: "gimble.interviewDecision",
      result: "Would you trade reliable Wi-Fi for a more secluded cabin?",
      error: "",
      interrupted: false,
      started: at("2026-09-17T12:47:52"),
      ended: at("2026-09-17T12:48:10"),
      duration: 18000,
    },
    "research.1/transport.1/interviewer.1/turn.1": {
      run,
      id: "research.1/transport.1/interviewer.1/turn.1",
      session: "research.1/transport.1/interviewer.1",
      scope: "research.1/transport.1",
      prompt: transportPurpose,
      output_type: "gimble.interviewDecision",
      result: "How do you like to travel when the trip itself should be restful?",
      error: "",
      interrupted: false,
      started: at("2026-09-17T12:46:02"),
      ended: at("2026-09-17T12:46:20"),
      duration: 18000,
    },
    "research.1/transport.1/interviewer.1/turn.2": {
      run,
      id: "research.1/transport.1/interviewer.1/turn.2",
      session: "research.1/transport.1/interviewer.1",
      scope: "research.1/transport.1",
      prompt: transportPurpose,
      output_type: "gimble.interviewDecision",
      result: "What is the longest drive you would enjoy?",
      error: "",
      interrupted: false,
      started: at("2026-09-17T12:48:00"),
      ended: at("2026-09-17T12:48:15"),
      duration: 15000,
    },
  },
  turn_usage: {
    "research.1/lodging.1/interviewer.1/turn.1": { [sonnet]: usage({ input: 1840, output: 62 }) },
    "research.1/lodging.1/interviewer.1/turn.2": {
      [sonnet]: usage({ input: 2410, output: 58, cache_read: 1792 }),
    },
    "research.1/transport.1/interviewer.1/turn.1": { [sonnet]: usage({ input: 1806, output: 66 }) },
    "research.1/transport.1/interviewer.1/turn.2": {
      [sonnet]: usage({ input: 2388, output: 54, cache_read: 1760 }),
    },
  },
  model_calls: {},
  // plan-trip runs no commands.
  commands: {},
  totals: {
    scopes: {
      "": total(sonnet, { input: 8444, output: 240, cache_read: 3552 }),
      "research.1": total(sonnet, { input: 8444, output: 240, cache_read: 3552 }),
      "research.1/lodging.1": total(sonnet, { input: 4250, output: 120, cache_read: 1792 }),
      "research.1/transport.1": total(sonnet, { input: 4194, output: 120, cache_read: 1760 }),
    },
    sessions: {
      "research.1/lodging.1/interviewer.1": total(sonnet, {
        input: 4250,
        output: 120,
        cache_read: 1792,
      }),
      "research.1/transport.1/interviewer.1": total(sonnet, {
        input: 4194,
        output: 120,
        cache_read: 1760,
      }),
    },
  },
  transcripts: {},
};

/** Interview.html: the graph and the snapshot the map is drawn from. */
export const planTripRun = { graph: planTripGraph, snapshot: planTripSnapshot };
