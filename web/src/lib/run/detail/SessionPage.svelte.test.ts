import { expect, test, vi } from "vite-plus/test";
import { render } from "vitest-browser-svelte";
import { userEvent } from "vitest/browser";
import { implementInterviewFixture } from "../fixtures/index.js";
import { RunObservation } from "../../observation/index.js";
import Page from "./SessionPage.svelte";
const remote = vi.hoisted(() => ({
  sent: vi.fn(),
  landed: true,
  inventoryError: undefined as Error | undefined,
  refresh: vi.fn(async () => undefined),
  inventory: {
    environments: [
      { environment: { id: "local", machine: "local" }, capabilities: { steer: true } },
    ],
    machines: [{ id: "local", name: "This Mac" }],
  },
}));
vi.mock("../../environments.remote", () => ({
  environmentSnapshot: () => ({
    current: remote.inventoryError ? undefined : remote.inventory,
    error: remote.inventoryError,
    refresh: remote.refresh,
  }),
}));
vi.mock("../../../routes/steer.remote", () => ({
  steer: {
    for: () => ({
      pending: 0,
      fields: {
        ...Object.fromEntries(
          ["run", "session", "message"].map((name) => [
            name,
            {
              as: (type: string, value?: string) => ({
                name,
                type,
                value: value ?? (name === "message" ? "Draft steer" : ""),
              }),
              value: () => "Draft steer",
            },
          ]),
        ),
        allIssues: () => [],
      },
      enhance: (handler: (submitted: unknown) => Promise<void>) => ({
        onsubmit: async (event: SubmitEvent) => {
          event.preventDefault();
          const element = event.currentTarget as HTMLFormElement;
          remote.sent(Object.fromEntries(new FormData(element)));
          await handler({ submit: async () => true, result: { landed: remote.landed }, element });
        },
      }),
    }),
  },
}));
const sessionID = "coding.1";
const turnID = "coding.1/turn.3";
function record() {
  const snapshot = structuredClone(implementInterviewFixture.snapshot);
  snapshot.sessions[sessionID].workdir = "/work/feature";
  snapshot.sessions[sessionID].steers = [
    { message: "Keep the patch small", source: "person", landed: true, created: 1 },
    { message: "Late steer", source: "person", landed: false, created: 2 },
  ];
  snapshot.transcripts[turnID] = {
    snapshot: {
      state: {
        info: {},
        family: {},
        active: {},
        pending: {},
        permission: {},
        form: {},
        message: {
          [sessionID]: [
            {
              id: "answer",
              type: "assistant",
              content: [
                { type: "text", text: "Current recorded answer." },
                {
                  type: "tool",
                  id: "tool",
                  name: "Inspect files",
                  function: { name: "inspect" },
                  state: { status: "completed", input: { path: "/work/file" } },
                },
              ],
            },
          ],
        },
      },
    },
    provenance: {},
  };
  return snapshot;
}
test("agent focus reads like chat and preserves prompt, usage, tools and steer facts", async () => {
  const snapshot = record();
  const screen = await render(Page, {
    project: "project",
    snapshot,
    observation: new RunObservation(snapshot),
    sessionID,
  });
  await expect.element(screen.getByText("Current recorded answer.", { exact: true })).toBeVisible();
  await expect.element(screen.getByText("/work/feature", { exact: true })).toBeVisible();
  await expect.element(screen.getByText("Keep the patch small", { exact: true })).toBeVisible();
  await expect.element(screen.getByText(/Landed · person/)).toBeVisible();
  await expect.element(screen.getByText(/Dropped · person/)).toBeVisible();
  await expect
    .element(screen.getByRole("button", { name: "Stop turn", exact: true }))
    .not.toBeInTheDocument();
  await expect
    .element(screen.getByRole("button", { name: "Cancel run", exact: true }))
    .not.toBeInTheDocument();
  await screen.getByText("Prompt & context", { exact: true }).click();
  await expect
    .element(screen.getByText(snapshot.turns[turnID].prompt, { exact: true }))
    .toBeVisible();
});
test("steer submit targets the current session even while viewing an earlier turn", async () => {
  remote.sent.mockClear();
  const snapshot = record();
  const screen = await render(Page, {
    snapshot,
    observation: new RunObservation(snapshot),
    sessionID,
  });
  await screen.getByRole("combobox", { name: "Recorded turn" }).selectOptions("coding.1/turn.2");
  await screen.getByRole("textbox", { name: "Steer this turn" }).fill("Check the current change");
  await userEvent.keyboard("{Enter}");
  expect(remote.sent).toHaveBeenCalledWith({
    run: snapshot.run.id,
    session: sessionID,
    message: "Check the current change",
  });
  await expect.element(screen.getByText(/Steering still goes to the running turn/)).toBeVisible();
});
test("between turns and messaging routes explain why the arrow is dimmed", async () => {
  const snapshot = record();
  for (const turn of Object.values(snapshot.turns))
    if (turn.session === sessionID) turn.ended = turn.started + 1;
  const screen = await render(Page, {
    snapshot,
    observation: new RunObservation(snapshot),
    sessionID,
  });
  await expect.element(screen.getByRole("button", { name: "Send steer" })).toBeDisabled();
  await expect
    .element(screen.getByText("Between turns. Steering is available while this agent is working."))
    .toBeVisible();
  remote.inventory.environments[0].capabilities.steer = false;
  await screen.rerender({
    project: "project",
    snapshot: record(),
    observation: new RunObservation(record()),
    sessionID,
  });
  await expect.element(screen.getByText("This route can’t be steered.")).toBeVisible();
  await expect.element(screen.getByRole("button", { name: "Send steer" })).toBeDisabled();
  remote.inventory.environments[0].capabilities.steer = true;
});

// Real mutable projection events, while the component remains mounted.
test("focus renders new messages, text deltas, tool progress/results and retry causes", async () => {
  const snapshot = record();
  snapshot.transcripts[turnID].snapshot.state.message[sessionID][0].time = {
    created: 1,
    completed: 2,
  };
  const observation = new RunObservation(snapshot);
  const generation = observation.beginConnection();
  const screen = await render(Page, {
    snapshot,
    observation,
    sessionID,
    revision: observation.revision,
  });
  let sequence = 0;
  async function event(type: string, data: Record<string, unknown>) {
    observation.apply(
      {
        type: "event",
        data: {
          turn: turnID,
          scope: "coding",
          session: sessionID,
          event: {
            type,
            id: `incremental-${++sequence}`,
            created: sequence,
            data: { sessionID, assistantMessageID: "live-answer", ...data },
          },
        },
      },
      generation,
    );
    await screen.rerender({
      snapshot: observation.snapshot(),
      observation,
      sessionID,
      revision: observation.revision,
    });
  }
  await event("session.step.started", { agent: "coding", model: { id: "test" } });
  await event("session.text.started", {});
  await event("session.text.delta", { delta: "New live text" });
  await expect.element(screen.getByText("New live text", { exact: true })).toBeVisible();
  await event("session.text.delta", { delta: " keeps growing" });
  await expect
    .element(screen.getByText("New live text keeps growing", { exact: true }))
    .toBeVisible();
  await event("session.tool.input.started", { id: "live-tool", name: "Read live file" });
  await event("session.tool.called", { id: "live-tool", input: { path: "/work/live" } });
  await screen.getByText("Read live file", { exact: true }).first().click();
  await event("session.tool.progress", {
    id: "live-tool",
    metadata: { progress: "Reading chunk 2" },
  });
  await expect.element(screen.getByText(/Reading chunk 2/)).toBeVisible();
  await event("session.tool.success", {
    id: "live-tool",
    content: [{ type: "text", text: "New live result" }],
  });
  await screen.getByRole("button", { name: /Read live file.*ok/ }).click();
  await expect.element(screen.getByText("New live result", { exact: true })).toBeVisible();
  await event("session.retry.scheduled", {
    attempt: 2,
    error: { message: "Provider rate limit: back off" },
  });
  await expect.element(screen.getByText(/Provider rate limit: back off/)).toBeVisible();
});

test("message errors, shell and compaction facts remain accessible", async () => {
  const snapshot = record();
  const messages = snapshot.transcripts[turnID].snapshot.state.message[sessionID];
  messages[0].error = { message: "Provider message failed" };
  messages.push({ id: "shell", type: "shell", command: "pwd", output: "/work/tree" });
  messages.push({ id: "compact", type: "compaction", summary: "Earlier work retained" });
  const screen = await render(Page, {
    snapshot,
    observation: new RunObservation(snapshot),
    sessionID,
  });
  await expect.element(screen.getByText(/Provider message failed/)).toBeVisible();
  await screen.getByText("Shell command", { exact: true }).click();
  await expect.element(screen.getByText(/\$ pwd/)).toBeVisible();
  await screen.getByText("Context compaction", { exact: true }).click();
  await expect.element(screen.getByText("Earlier work retained", { exact: true })).toBeVisible();
});

test("steering availability failures explain the problem and offer retry", async () => {
  remote.inventoryError = new Error("inventory unavailable");
  try {
    const snapshot = record();
    const screen = await render(Page, {
      project: "project",
      snapshot,
      observation: new RunObservation(snapshot),
      sessionID,
    });
    await expect
      .element(screen.getByText("Could not check steering availability. Retry below."))
      .toBeVisible();
    await expect.element(screen.getByRole("button", { name: "Send steer" })).toBeDisabled();
    await screen.getByRole("button", { name: "Retry steering availability" }).click();
    expect(remote.refresh).toHaveBeenCalled();
  } finally {
    remote.inventoryError = undefined;
  }
});

test("the arrow reports whether a steer landed or was dropped", async () => {
  const snapshot = record();
  const screen = await render(Page, {
    snapshot,
    observation: new RunObservation(snapshot),
    sessionID,
  });
  remote.landed = true;
  await screen.getByRole("textbox", { name: "Steer this turn" }).fill("Use the running session");
  await screen.getByRole("button", { name: "Send steer" }).click();
  await expect.element(screen.getByText("Steer landed.", { exact: true })).toBeVisible();
  remote.landed = false;
  await screen.getByRole("textbox", { name: "Steer this turn" }).fill("Late steer");
  await screen.getByRole("button", { name: "Send steer" }).click();
  await expect
    .element(screen.getByText("The turn ended before this steer landed. It was dropped."))
    .toBeVisible();
  remote.landed = true;
});

test("live focus follows its tail but leaves an earlier reader in place", async () => {
  const snapshot = record();
  const list = snapshot.transcripts[turnID].snapshot.state.message[sessionID];
  for (let index = 0; index < 35; index++)
    list.push({
      id: `earlier-${index}`,
      type: "assistant",
      time: { created: index, completed: index + 1 },
      content: [{ type: "text", text: `Earlier ${index}: ` + "Recorded prose. ".repeat(30) }],
    });
  list.push({
    id: "tail",
    type: "assistant",
    time: { created: 50 },
    content: [{ type: "text", text: "Live tail" }],
  });
  const observation = new RunObservation(snapshot);
  const generation = observation.beginConnection();
  const screen = await render(Page, {
    snapshot,
    observation,
    sessionID,
    revision: observation.revision,
  });
  screen.container.style.height = "500px";
  const scroller = screen.container.querySelector<HTMLDivElement>(".conversation")!;
  async function delta(text: string) {
    observation.apply(
      {
        type: "event",
        data: {
          turn: turnID,
          scope: "coding",
          session: sessionID,
          event: {
            type: "session.text.delta",
            id: text,
            created: 100,
            data: { sessionID, assistantMessageID: "tail", delta: text },
          },
        },
      },
      generation,
    );
    await screen.rerender({
      snapshot: observation.snapshot(),
      observation,
      sessionID,
      revision: observation.revision,
    });
  }
  // Explicitly start following after constraining the mounted test viewport.
  scroller.scrollTop = scroller.scrollHeight;
  await delta(" more prose".repeat(30));
  await expect
    .poll(() => scroller.scrollHeight - scroller.clientHeight - scroller.scrollTop)
    .toBeLessThan(48);
  scroller.scrollTop = 0;
  await delta(" latest prose".repeat(30));
  await expect.poll(() => scroller.scrollTop).toBe(0);
});
