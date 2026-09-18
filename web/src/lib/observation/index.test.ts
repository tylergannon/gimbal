import assert from "node:assert/strict";
import { test } from "vite-plus/test";
import {
  RunObservation,
  foldProvenance,
  usageOf,
  usageText,
  type RunSnapshot,
  type Usage,
} from "./index.ts";
import type { Snapshot } from "../sessionstate/index.ts";

const projection = (): Snapshot => ({
  state: {
    info: { ses: { id: "ses", title: "start" } },
    family: { ses: ["ses"] },
    active: {},
    message: { ses: [] },
    pending: {},
    permission: {},
    form: {},
  },
});
const usage = (input: number, cost = 0): Usage => ({
  input,
  cache_read: 0,
  cache_write: 0,
  output: 0,
  reasoning: 0,
  stated_cost: cost,
});
const snapshot = (title = "start"): RunSnapshot => {
  const value = projection();
  value.state.info.ses.title = title;
  return {
    stream: "stream-1",
    position: 0,
    run: { id: "run", name: "Run", status: "running", error: "", started: 1, ended: 0 },
    scopes: {
      "loop.1": {
        run: "run",
        key: "loop.1",
        name: "loop.1",
        loop: true,
        status: "running",
        error: "",
        began: 1,
        ended: 0,
        values: {},
        decisions: [],
      },
    },
    sessions: {
      ses: {
        run: "run",
        id: "ses",
        name: "agent",
        adapter: "codex",
        model: "m",
        scope: "lap",
        parent: "",
        created: 1,
      },
    },
    interviews: {},
    turns: {
      turn: {
        run: "run",
        id: "turn",
        session: "ses",
        scope: "lap",
        prompt: "go",
        output_type: "gimble.Text",
        result: "",
        error: "",
        interrupted: false,
        started: 2,
        ended: 0,
        duration: 0,
      },
    },
    turn_usage: {},
    model_calls: {},
    totals: { scopes: {}, sessions: {} },
    transcripts: { turn: { snapshot: value, provenance: {} } },
  };
};

test("replacement snapshot resets machines and stale connection callbacks cannot mutate them", () => {
  const observation = new RunObservation(snapshot("first"));
  const oldConnection = observation.beginConnection();
  observation.apply({ type: "snapshot", data: snapshot("replacement") }, oldConnection);
  const currentConnection = observation.beginConnection();
  assert.equal(
    observation.apply(
      {
        type: "event",
        data: {
          scope: "lap",
          session: "ses",
          turn: "turn",
          event: {
            id: "old",
            created: 1,
            type: "session.renamed",
            data: { sessionID: "ses", title: "stale" },
          },
        },
      },
      oldConnection,
    ),
    false,
  );
  observation.apply(
    {
      type: "event",
      data: {
        scope: "lap",
        session: "ses",
        turn: "turn",
        event: {
          id: "new",
          created: 2,
          type: "session.permissions.updated",
          data: { sessionID: "ses", permissions: ["read"] },
        },
      },
    },
    currentConnection,
  );
  assert.equal(observation.state("turn")?.info.ses.title, "replacement");
  assert.deepEqual(observation.state("turn")?.info.ses.permissions, ["read"]);
});

test("a replacement snapshot replaces every table, not only the transcripts", () => {
  const observation = new RunObservation(snapshot());
  const connection = observation.beginConnection();
  observation.apply(
    { type: "row", data: { table: "turn_usage", key: "turn", row: { m: usage(5) } } },
    connection,
  );
  const replacement = snapshot();
  replacement.run.status = "completed";
  observation.apply({ type: "snapshot", data: replacement }, connection);
  assert.equal(observation.run.status, "completed");
  assert.deepEqual(observation.turnUsage, {});
});

test("a row frame is one row of one table, replacing what was there", () => {
  const observation = new RunObservation(snapshot());
  const connection = observation.beginConnection();
  observation.apply(
    {
      type: "row",
      data: {
        table: "scopes",
        key: "loop.1/task.2",
        row: {
          run: "run",
          key: "loop.1/task.2",
          name: "task.2",
          loop: false,
          status: "ended",
          error: "",
          task: { name: "write a.txt" },
          began: 3,
          ended: 4,
          values: { result: { value: "done" } },
          decisions: [],
        },
      },
    },
    connection,
  );
  assert.equal(observation.scopes["loop.1/task.2"].status, "ended");
  assert.deepEqual(observation.scopes["loop.1/task.2"].values, { result: { value: "done" } });

  observation.apply(
    {
      type: "row",
      data: {
        table: "turns",
        key: "turn.2",
        row: {
          run: "run",
          id: "turn.2",
          session: "ses",
          scope: "loop.1/task.2",
          prompt: "build",
          output_type: "gimble.Text",
          result: "",
          error: "",
          interrupted: false,
          started: 5,
          ended: 0,
          duration: 0,
        },
      },
    },
    connection,
  );
  // The turn is charged to the scope it ran in, not the one its session was
  // created in.
  assert.equal(observation.turns["turn.2"].scope, "loop.1/task.2");
  assert.equal(observation.sessions.ses.scope, "lap");

  observation.apply(
    {
      type: "row",
      data: {
        table: "interviews",
        key: "question-1",
        row: {
          run: "run",
          question_id: "question-1",
          name: "preferences",
          scope: "loop.1/task.2",
          session: "ses",
          question: "Which color?",
          status: "pending",
          answer: "",
          asked: 6,
          answered: 0,
        },
      },
    },
    connection,
  );
  assert.equal(observation.interviews["question-1"].question, "Which color?");
  observation.apply(
    {
      type: "row",
      data: {
        table: "interviews",
        key: "question-1",
        row: {
          run: "run",
          question_id: "question-1",
          name: "preferences",
          scope: "loop.1/task.2",
          session: "ses",
          question: "Which color?",
          status: "answered",
          answer: "Blue",
          asked: 6,
          answered: 7,
        },
      },
    },
    connection,
  );
  assert.equal(observation.interviews["question-1"].answer, "Blue");

  observation.apply(
    { type: "row", data: { table: "turn_usage", key: "turn.2", row: { m: usage(12, 0.5) } } },
    connection,
  );
  assert.deepEqual(observation.turnUsage["turn.2"], { m: usage(12, 0.5) });
  // The row is the turn's whole model map, so a later frame replaces it.
  observation.apply(
    { type: "row", data: { table: "turn_usage", key: "turn.2", row: { other: usage(1) } } },
    connection,
  );
  assert.deepEqual(observation.turnUsage["turn.2"], { other: usage(1) });

  observation.apply(
    {
      type: "row",
      data: {
        table: "run",
        key: "",
        row: {
          id: "run",
          name: "Run",
          status: "cancelled",
          error: "context canceled",
          started: 1,
          ended: 9,
        },
      },
    },
    connection,
  );
  assert.equal(observation.run.status, "cancelled");
  assert.equal(observation.run.error, "context canceled");
});

test("a totals frame is the roll-ups, and the page never sums anything itself", () => {
  const observation = new RunObservation(snapshot());
  const connection = observation.beginConnection();
  observation.apply(
    {
      type: "totals",
      data: {
        scopes: {
          "": { all: usage(30), by_model: { m: usage(30) } },
          lap: { all: usage(30), by_model: { m: usage(30) } },
        },
        sessions: { ses: { all: usage(30), by_model: { m: usage(30) } } },
      },
    },
    connection,
  );
  assert.deepEqual(observation.totals.scopes[""].all, usage(30));
  assert.deepEqual(observation.totals.sessions.ses.by_model.m, usage(30));
  // A second frame is the whole roll-up again.
  observation.apply(
    { type: "totals", data: { scopes: { "": { all: usage(40), by_model: {} } }, sessions: {} } },
    connection,
  );
  assert.deepEqual(observation.totals.scopes[""].all, usage(40));
  assert.equal(observation.totals.sessions.ses, undefined);
});

test("message revision survives a following row frame", () => {
  const observation = new RunObservation(snapshot());
  const connection = observation.beginConnection();
  observation.apply(
    {
      type: "event",
      data: {
        scope: "lap",
        session: "ses",
        turn: "turn",
        event: {
          id: "delta",
          created: 3,
          type: "session.text.delta",
          data: { sessionID: "ses", assistantMessageID: "message", delta: "done" },
        },
      },
    },
    connection,
  );
  const changed = observation.messageRevision("turn", "message");
  observation.apply(
    { type: "row", data: { table: "turn_usage", key: "turn", row: { m: usage(2) } } },
    connection,
  );
  assert.equal(observation.messageRevision("turn", "message"), changed);
});

test("usage is five zero-filled token counts and a stated cost, whatever the harness reported", () => {
  assert.deepEqual(usageOf(undefined), {
    input: 0,
    cache_read: 0,
    cache_write: 0,
    output: 0,
    reasoning: 0,
    stated_cost: 0,
  });
  // A message carries the counts nested; the page holds them flat.
  assert.deepEqual(
    usageOf({
      cost: 0.25,
      tokens: { input: 10, output: 2, reasoning: 3, cache: { read: 4, write: 5 } },
    }),
    { input: 10, cache_read: 4, cache_write: 5, output: 2, reasoning: 3, stated_cost: 0.25 },
  );
  // A field the harness never reported reads as the number zero, with no
  // wording about availability anywhere in the line.
  assert.equal(
    usageText(usageOf({ tokens: { input: 7 } })),
    "7 in · 0 out · 0 reasoning · 0 cache read · 0 cache write · $0",
  );
});

test("provenance keys the latest native sidecar by normalized message ID", () => {
  const provenance: Record<string, unknown> = {};
  foldProvenance(
    provenance,
    { data: { assistantMessageID: "source-id" } },
    { normalizedMessageID: "msg_canonical", messageID: "provider-id" },
  );
  assert.deepEqual(Object.keys(provenance), ["msg_canonical"]);
  assert.equal((provenance.msg_canonical as { messageID: string }).messageID, "provider-id");
});

test("the snapshot it hands back is every table and every transcript", () => {
  const observation = new RunObservation(snapshot());
  const connection = observation.beginConnection();
  observation.apply(
    { type: "row", data: { table: "turn_usage", key: "turn", row: { m: usage(3) } } },
    connection,
  );
  const out = observation.snapshot();
  assert.deepEqual(out.turn_usage, { turn: { m: usage(3) } });
  assert.deepEqual(out.scopes, observation.scopes);
  assert.deepEqual(out.sessions, observation.sessions);
  assert.deepEqual(out.interviews, observation.interviews);
  assert.deepEqual(out.turns, observation.turns);
  assert.equal(out.transcripts.turn.snapshot.state.info.ses.title, "start");
});

test("a delta advances the cursor only after its complete frame group applies", () => {
  const observation = new RunObservation(snapshot());
  const connection = observation.beginConnection();
  assert.equal(
    observation.applyDelta(
      {
        stream: "stream-1",
        position: 1,
        frames: [
          {
            type: "event",
            data: {
              scope: "lap",
              session: "ses",
              turn: "turn",
              event: {
                id: "delta",
                created: 3,
                type: "session.permissions.updated",
                data: { sessionID: "ses", permissions: ["read"] },
              },
            },
          },
          { type: "row", data: { table: "turn_usage", key: "turn", row: { m: usage(2) } } },
        ],
      },
      connection,
    ),
    true,
  );
  assert.equal(observation.position, 1);
  assert.deepEqual(observation.state("turn")?.info.ses.permissions, ["read"]);
  assert.deepEqual(observation.turnUsage.turn, { m: usage(2) });
  assert.equal(
    observation.applyDelta({ stream: "stream-1", position: 1, frames: [] }, connection),
    false,
  );
  assert.equal(
    observation.applyDelta({ stream: "stale", position: 2, frames: [] }, connection),
    false,
  );
});
