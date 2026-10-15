import { expect, test, vi } from "vite-plus/test";
import { render } from "vitest-browser-svelte";
import { userEvent } from "vitest/browser";
import { implementInterviewFixture } from "../fixtures/index.js";
import { RunObservation } from "../../observation/index.js";
import Page from "./SessionPage.svelte";
const remote = vi.hoisted(() => ({
  sent: vi.fn(),
  inventory: {
    environments: [
      { environment: { id: "local", machine: "local" }, capabilities: { steer: true } },
    ],
    machines: [{ id: "local", name: "This Mac" }],
  },
}));
vi.mock("../../environments.remote", () => ({
  environmentSnapshot: () => ({ current: remote.inventory }),
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
      enhance: () => ({
        onsubmit: (event: SubmitEvent) => {
          event.preventDefault();
          remote.sent(Object.fromEntries(new FormData(event.currentTarget as HTMLFormElement)));
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
  await expect.element(screen.getByText("This route can't be steered")).toBeVisible();
  await expect.element(screen.getByRole("button", { name: "Send steer" })).toBeDisabled();
  remote.inventory.environments[0].capabilities.steer = true;
});
