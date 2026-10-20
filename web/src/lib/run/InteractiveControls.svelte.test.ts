import { expect, test, vi } from "vite-plus/test";
import { render } from "vitest-browser-svelte";
import { RunObservation, type RunSnapshot } from "../observation/index.js";
import { implementInteractiveFixture, planTripFixture } from "./fixtures/index.js";
import Transcript from "./RunTranscript.svelte";
import Focus from "./detail/SessionPage.svelte";
import Detail from "./DetailPane.svelte";

const controls = vi.hoisted(() => ({
  messages: vi.fn(),
  genericSteers: vi.fn(),
  finish: vi.fn(async () => ({ accepted: true })),
  stop: vi.fn(async () => ({ accepted: true })),
}));
vi.mock("../environments.remote", () => ({
  environmentSnapshot: () => ({
    current: {
      environments: [
        { environment: { id: "local", machine: "local" }, capabilities: { steer: true } },
      ],
      machines: [{ id: "local", name: "This Mac" }],
    },
  }),
}));
vi.mock("../../routes/control.remote", () => ({ stopTurn: controls.stop }));
vi.mock("../../routes/interactive.remote", () => ({
  finishInteractive: controls.finish,
  answerInteractive: {
    for: () => ({
      pending: 0,
      fields: {
        ...Object.fromEntries(
          ["request_id", "run", "question_id", "answer"].map((name) => [
            name,
            {
              as: (type: string, value?: string) => ({ name, type, value }),
            },
          ]),
        ),
        allIssues: () => [],
      },
      enhance: (handler: (submitted: unknown) => Promise<void>) => ({
        onsubmit: async (event: SubmitEvent) => {
          event.preventDefault();
          controls.messages(
            Object.fromEntries(new FormData(event.currentTarget as HTMLFormElement)),
          );
          await handler({ submit: async () => true, result: { accepted: true } });
        },
      }),
    }),
  },
}));
vi.mock("../../routes/steer.remote", () => ({
  steerLoop: { for: () => ({}) },
  steer: {
    for: () => ({
      pending: 0,
      fields: {
        ...Object.fromEntries(
          ["run", "session", "message"].map((name) => [
            name,
            {
              as: (type: string, value?: string) => ({ name, type, value }),
              value: () => "A steer",
            },
          ]),
        ),
        allIssues: () => [],
      },
      enhance: (handler: (submitted: unknown) => Promise<void>) => ({
        onsubmit: async (event: SubmitEvent) => {
          event.preventDefault();
          const element = event.currentTarget as HTMLFormElement;
          controls.genericSteers(Object.fromEntries(new FormData(element)));
          await handler({ submit: async () => true, result: { landed: true }, element });
        },
      }),
    }),
  },
}));

const sessionID = "coding.1";
const turnID = "coding.1/turn.3";
const address = "implementation.1/task.3/dialogue";
function snapshot() {
  const snapshot = structuredClone(implementInteractiveFixture.snapshot);
  snapshot.interactions.dialogue = {
    ...Object.values(planTripFixture.snapshot.interactions)[0],
    question_id: "dialogue",
    name: "dialogue",
    address,
    session: sessionID,
    scope: snapshot.turns[turnID].scope,
    status: "active",
    messages: [],
    ended_by: "",
  };
  return snapshot;
}
function mount(view: string, snapshot: RunSnapshot) {
  const observation = new RunObservation(snapshot);
  if (view === "transcript")
    return render(Transcript, {
      project: "project",
      snapshot,
      observation,
      revision: 0,
      onmap: vi.fn(),
    });
  if (view === "focus")
    return render(Focus, { project: "project", snapshot, observation, sessionID });
  const loop = implementInteractiveFixture.graph.body.find(
    (operation) => operation.kind === "promise_loop",
  );
  const operation = loop?.body.find((operation) => operation.kind === "agent_call");
  if (!operation) throw new Error("missing agent operation");
  return render(Detail, {
    snapshot,
    observation,
    selection: {
      kind: "node",
      scope: snapshot.scopes[snapshot.turns[turnID].scope],
      operation,
      runtime: snapshot.turns[turnID],
    },
  });
}

test.each(["transcript", "focus", "detail"])(
  "%s sends active Interactive messages through durable ingress",
  async (view) => {
    controls.messages.mockClear();
    controls.genericSteers.mockClear();
    controls.stop.mockClear();
    controls.finish.mockClear();
    const record = snapshot();
    const screen = await mount(view, record);
    if (view === "transcript") await screen.getByRole("button", { name: /^coding/ }).click();
    await screen
      .getByRole("textbox", { name: "Answer the waiting question" })
      .fill("Keep this input if the turn ends");
    await screen.getByRole("button", { name: "Send answer", exact: true }).click();
    expect(controls.messages).toHaveBeenCalledWith(
      expect.objectContaining({
        run: record.run.id,
        question_id: address,
        answer: "Keep this input if the turn ends",
        request_id: expect.any(String),
      }),
    );
    expect(controls.messages.mock.calls[0][0].request_id).not.toBe("");
    expect(controls.genericSteers).not.toHaveBeenCalled();
    await screen.getByRole("button", { name: "Stop turn", exact: true }).click();
    expect(controls.stop).toHaveBeenCalledWith({ run: record.run.id, turn: turnID });
    await screen.getByRole("button", { name: "Finish interaction", exact: true }).click();
    expect(controls.finish).toHaveBeenCalledWith(
      expect.objectContaining({ run: record.run.id, address }),
    );
  },
);

test("an ordinary Generate turn retains generic steering", async () => {
  controls.genericSteers.mockClear();
  controls.messages.mockClear();
  const record = structuredClone(implementInteractiveFixture.snapshot);
  const screen = await mount("transcript", record);
  await screen.getByRole("button", { name: /^coding/ }).click();
  await screen.getByRole("textbox", { name: "Steer this turn" }).fill("A generic steer");
  await screen.getByRole("button", { name: "Send steer" }).click();
  expect(controls.genericSteers).toHaveBeenCalledWith(
    expect.objectContaining({ run: record.run.id, session: sessionID, message: "A generic steer" }),
  );
  expect(controls.messages).not.toHaveBeenCalled();
});
