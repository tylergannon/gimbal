import type { InterviewRow, RunRow } from "../../observation/index.js";
import { planTripSnapshot } from "./plan-trip.js";

const rows = [
  {
    id: "01M2RWXNFFYQA4HHQWMQ252CYF",
    name: "plan-trip",
    status: "running",
    error: "",
    started: 1_800_000_000_000,
    ended: 0,
  },
  {
    id: "01M2RX4A9Q0ZT7V4C3D8N1KMB2",
    name: "implement-interview",
    status: "running",
    error: "",
    started: 1_799_997_340_000,
    ended: 0,
  },
  {
    id: "01M2R9K2WQ6HS8E5A1F0J4TPCX",
    name: "interview",
    status: "completed",
    error: "",
    started: 1_799_910_000_000,
    ended: 1_799_910_545_000,
  },
  {
    id: "01M2QZ7PB3N6D0R9X2G5HKW8VY",
    name: "implement-interview",
    status: "failed",
    error: "test exited with code 1",
    started: 1_799_910_662_000,
    ended: 1_799_914_982_000,
  },
  {
    id: "01M2Q1HD4T8KJ2M7P5S0B9XNAE",
    name: "plan-trip",
    status: "cancelled",
    error: "",
    started: 1_799_820_000_000,
    ended: 1_799_820_201_000,
  },
  {
    id: "01M2P0AAZ5V3C1K8Y6W2E4RQHT",
    name: "interview",
    status: "completed",
    error: "",
    started: 1_799_730_000_000,
    ended: 1_799_730_470_000,
  },
] satisfies RunRow[];

const attention = [
  planTripSnapshot.interviews["01M2RWXNFFYQA4HHQWMQ252CYF"],
  planTripSnapshot.interviews["01M2RWXNFFYQA4HHQWMQ252CYG"],
] satisfies InterviewRow[];

export const runsListFixture = {
  runs: rows,
  summaries: {
    "01M2RWXNFFYQA4HHQWMQ252CYF":
      "2 questions waiting · research.1/lodging.1 · research.1/transport.1",
    "01M2RX4A9Q0ZT7V4C3D8N1KMB2":
      "coding.1 / turn.3 in implementation.1/task.3 · 2 supervisors watching",
    "01M2R9K2WQ6HS8E5A1F0J4TPCX": "5 exchanges · preferences returned",
    "01M2QZ7PB3N6D0R9X2G5HKW8VY": "test exit 1 in implementation.1/task.4",
    "01M2Q1HD4T8KJ2M7P5S0B9XNAE": "Cancelled by the operator during research.1",
    "01M2P0AAZ5V3C1K8Y6W2E4RQHT": "3 exchanges · no compiled graph for this version",
  },
  elapsed: {
    "01M2RWXNFFYQA4HHQWMQ252CYF": "12m 40s",
    "01M2RX4A9Q0ZT7V4C3D8N1KMB2": "48m 12s",
    "01M2R9K2WQ6HS8E5A1F0J4TPCX": "9m 05s",
    "01M2QZ7PB3N6D0R9X2G5HKW8VY": "1h 12m",
    "01M2Q1HD4T8KJ2M7P5S0B9XNAE": "3m 21s",
    "01M2P0AAZ5V3C1K8Y6W2E4RQHT": "7m 50s",
  },
  attention,
};
