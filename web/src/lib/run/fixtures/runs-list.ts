import type {
  CommandRow,
  InterviewRow,
  RunRow,
  ScopeRow,
  TurnRow,
} from "../../observation/index.js";
import { implementInterviewSnapshot } from "./implement-interview.js";
import { planTripPending, planTripSnapshot } from "./plan-trip.js";
import { recordedFailure, recordedRunSnapshot } from "./recorded-run.js";
import { at } from "./support.js";

const interviewRun = "01M2R9K2WQ6HS8E5A1F0J4TPCX";
const cancelledRun = "01M2Q1HD4T8KJ2M7P5S0B9XNAE";
const oldInterviewRun = "01M2P0AAZ5V3C1K8Y6W2E4RQHT";

/** One ended exchange of a finished `interview` run. The list counts these
 * to say how many exchanges a run returned. */
const exchange = (
  run: string,
  n: number,
  question: string,
  answer: string,
  asked: string,
  answered: string,
): InterviewRow => ({
  run,
  question_id: `${run}-q${n}`,
  name: "preferences",
  scope: "",
  session: "interviewer.1",
  question,
  status: "answered",
  answer,
  asked: at(asked),
  answered: at(answered),
});

/** Every run this project runtime has recorded, newest start first, which
 * is current work first. The two running rows are the same runs the map
 * fixtures draw, so opening one from the list arrives at the run it named:
 * at 12:48:50 plan-trip is 12m 40s old and implement-interview 48m 12s. */
export const runsListRuns: RunRow[] = [
  { ...planTripSnapshot.run },
  { ...implementInterviewSnapshot.run },
  {
    id: interviewRun,
    name: "interview",
    status: "completed",
    error: "",
    started: at("2026-09-16T16:20:04"),
    ended: at("2026-09-16T16:29:09"),
  },
  { ...recordedRunSnapshot.run },
  {
    id: cancelledRun,
    name: "plan-trip",
    status: "cancelled",
    error: "",
    started: at("2026-09-14T09:02:11"),
    ended: at("2026-09-14T09:05:32"),
  },
  {
    id: oldInterviewRun,
    name: "interview",
    status: "completed",
    error: "",
    started: at("2026-09-12T11:47:40"),
    ended: at("2026-09-12T11:55:30"),
  },
];

/** The questions waiting on a person, across every run. "Needs your answer"
 * is read from these, never from a run status. */
export const runsListAttention: InterviewRow[] = planTripPending;

/** Every interview row of the listed runs: the two waiting questions, and
 * the answered exchanges the two finished `interview` runs returned. */
export const runsListInterviews: InterviewRow[] = [
  ...planTripPending,
  exchange(
    interviewRun,
    1,
    "What are you trying to decide?",
    "Whether to take the coast road or the pass.",
    "2026-09-16T16:20:40",
    "2026-09-16T16:21:55",
  ),
  exchange(
    interviewRun,
    2,
    "How much time do you have for the drive?",
    "Half a day, no more.",
    "2026-09-16T16:22:10",
    "2026-09-16T16:23:02",
  ),
  exchange(
    interviewRun,
    3,
    "Does the view matter more than the time?",
    "Yes, if it costs under an hour.",
    "2026-09-16T16:23:20",
    "2026-09-16T16:24:40",
  ),
  exchange(
    interviewRun,
    4,
    "Are you driving alone?",
    "No, two of us, and we can swap.",
    "2026-09-16T16:25:02",
    "2026-09-16T16:26:18",
  ),
  exchange(
    interviewRun,
    5,
    "Would you stop overnight on the way?",
    "Only if the pass is closed.",
    "2026-09-16T16:26:40",
    "2026-09-16T16:28:50",
  ),
  exchange(
    oldInterviewRun,
    1,
    "What should this interview settle?",
    "Which weekend to take off.",
    "2026-09-12T11:48:10",
    "2026-09-12T11:49:40",
  ),
  exchange(
    oldInterviewRun,
    2,
    "Which weekends are already spoken for?",
    "The first and the last of the month.",
    "2026-09-12T11:50:02",
    "2026-09-12T11:51:30",
  ),
  exchange(
    oldInterviewRun,
    3,
    "Do you need to be back by Sunday evening?",
    "Yes, before eight.",
    "2026-09-12T11:52:04",
    "2026-09-12T11:55:10",
  ),
];

/** The scopes the list names: where the running implement-interview is now,
 * where each waiting question is, and the scope the cancelled run stopped
 * inside. */
export const runsListScopes: ScopeRow[] = [
  implementInterviewSnapshot.scopes["implementation.1/task.3"],
  {
    run: cancelledRun,
    key: "research.1",
    name: "research",
    loop: false,
    status: "ended",
    error: "",
    began: at("2026-09-14T09:02:12"),
    ended: at("2026-09-14T09:05:32"),
    values: {},
    decisions: [],
  },
];

/** The turn the list names: the coding turn implement-interview is in. Its
 * call carries the two supervisors the list counts, which the
 * implement-interview graph states. */
export const runsListTurns: TurnRow[] = [implementInterviewSnapshot.turns["coding.1/turn.3"]];

/** The command the failed run is summarised by: `test`, exit 1, in
 * implementation.1/task.4. */
export const runsListCommands: CommandRow[] = [recordedFailure];

/** Runs.html: the rows every line of the runs list is read from. */
export const runsList = {
  runs: runsListRuns,
  attention: runsListAttention,
  interviews: runsListInterviews,
  scopes: runsListScopes,
  turns: runsListTurns,
  commands: runsListCommands,
};
