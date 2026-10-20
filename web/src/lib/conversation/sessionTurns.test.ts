import { expect, test } from "vite-plus/test";
import type { Conversation } from "#lib/skgo/conversation/types";
import type { ProjectionState } from "#lib/sessionstate";
import { sessionTurns, runEvents, sessionTurnSeconds } from "./sessionTurns";
import { groupSummary, turnPresentation } from "./activity";
import steered from "./fixtures/steered-turns.json";
import codex from "./fixtures/codex-turns.json";

test("run provenance requires known completed launch output and explicit source request", () => {
  const record = {
    id: "source",
    requests: [
      { id: "first", kind: "prompt", delivery: "turn" },
      { id: "second", kind: "prompt", delivery: "turn" },
    ],
    runs: [{ id: "real-run", workflow: "review", status: "completed" }],
  } as Conversation;
  const tool = {
    type: "tool",
    name: "Bash",
    state: {
      status: "completed",
      input: { command: "gimbal run review --conversation source" },
      content: [{ type: "text", text: "real-run\n" }],
    },
  };
  const message = { id: "second/provider-id", type: "assistant", content: [tool] };
  const groups = sessionTurns(record, [
    { id: "user", type: "user", requestID: "second", text: "Review" },
    message,
  ]);
  expect(groups.turns[0].messages).toHaveLength(0);
  expect(groups.turns[1].messages).toHaveLength(2);
  expect(runEvents(message, record, groups.turns[1].request)[0]).toEqual(record.runs);
  expect(runEvents(message, record, groups.turns[0].request)).toEqual({});
  for (const changed of [
    { ...tool, state: { ...tool.state, status: "running" } },
    {
      ...tool,
      state: { ...tool.state, input: { command: "gimbal run review --conversation another" } },
    },
    { ...tool, state: { ...tool.state, content: [{ type: "text", text: "I launched real-run" }] } },
  ])
    expect(
      runEvents({ ...message, content: [changed] }, record, groups.turns[1].request)[0],
    ).toEqual([]);
});

test("turn elapsed excludes idle draft time and remains unknown without recorded delivery", () => {
  const turn = {
    request: {
      id: "request",
      created: 1000,
      kind: "prompt",
      message: "",
      target: "",
      file_id: "",
      status: "delivered",
      delivery: "turn",
      error: "",
    },
    ordinal: 1,
    messages: [
      { id: "user", type: "user", requestID: "request", time: { created: 121000 } },
      { id: "request/assistant", type: "assistant", time: { created: 121100, completed: 122000 } },
    ],
  } as Parameters<typeof sessionTurnSeconds>[0];
  expect(sessionTurnSeconds(turn)).toBe(1);
  expect(sessionTurnSeconds(turn, 123000)).toBe(2);
  expect(sessionTurnSeconds({ ...turn, messages: turn.messages.slice(1) })).toBeUndefined();
});

// The fixtures are what the Go conversation manager produced for a recorded run.
const grouped = (fixture: unknown) => {
  const record = fixture as Conversation;
  const messages = (JSON.parse(record.state) as ProjectionState).message[record.id] ?? [];
  return sessionTurns(record, messages);
};

test("a steer is part of the turn that took it and a later identical prompt is its own turn", () => {
  const groups = grouped(steered);
  expect(groups.turns.map((turn) => [turn.request.id, turn.ordinal])).toEqual([
    ["req-1", 1],
    ["req-3", 2],
  ]);
  expect(groups.turns[0].messages.map((message) => message.id)).toEqual([
    "req-1/prompt",
    "req-1/msg_1",
    "req-1/turn-1/steer/req-2",
  ]);
  expect(groups.turns[1].messages.map((message) => message.id)).toEqual([
    "req-3/prompt",
    "req-3/msg_2",
  ]);
  expect(groups.history).toEqual([]);
  expect((steered as Conversation).active_request).toBe("req-3");
});

test("two ended turns with the same words each keep their own reply", () => {
  const groups = grouped(codex);
  expect(groups.turns.map((turn) => turn.request.id)).toEqual(["req-1", "req-2"]);
  expect(groups.turns.map((turn) => turn.messages.length)).toEqual([2, 2]);
  expect(groups.history).toEqual([]);
});

test("activity keeps all final paragraphs visible and hides completion-only reasoning", () => {
  const messages = [
    { id: "intro", type: "assistant", content: [{ type: "text", text: "Checking the files." }] },
    {
      id: "tool",
      type: "assistant",
      content: [{ type: "tool", name: "Bash", state: { status: "completed" } }],
    },
    {
      id: "answer",
      type: "assistant",
      content: [
        { type: "text", text: "The check passed." },
        { type: "text", text: "Here are the details." },
      ],
    },
    {
      id: "protocol",
      type: "assistant",
      content: [
        { type: "reasoning", text: "Return the structured result." },
        { type: "tool", name: "StructuredOutput" },
      ],
    },
  ];
  const shown = turnPresentation(messages, false);
  expect(shown.results.map((item) => item.message.content[0].text)).toEqual([
    "The check passed.",
    "Here are the details.",
  ]);
  expect(shown.activity.map((item) => item.type)).toEqual(["message", "tools"]);
  expect(JSON.stringify(shown)).not.toContain("Return the structured result.");
  expect(turnPresentation(messages, true).results).toEqual([]);
});

test("a batch summarizes distinct action types once without including commands", () => {
  const part = (name: string, status = "completed") => ({
    type: "tool",
    name,
    state: { status, input: { command: "private command details" } },
  });
  expect(
    groupSummary([
      part("ToolSearch"),
      part("Read"),
      part("Read"),
      part("Bash"),
      part("mcp__codex_app__list_artifacts"),
      part("mcp__codex_app__create_worktree"),
    ]),
  ).toBe("Loaded tools, read files, ran commands, listed attached artifacts, created worktree");
  expect(groupSummary([part("functions.exec_command", "running"), part("Read")])).toBe(
    "Running commands, read files",
  );
  expect(groupSummary([part("UnknownTool")])).toBe("Used tools");
});
