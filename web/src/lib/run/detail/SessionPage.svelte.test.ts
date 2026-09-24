import { expect, test, vi } from "vite-plus/test";
import { render } from "vitest-browser-svelte";
import { implementInterviewFixture } from "../fixtures/index.js";
import { RunObservation } from "../../observation/index.js";
import Page from "./SessionPage.svelte";

const sessionID = "coding.1";
const oldTurn = "coding.1/turn.2";
const liveTurn = "coding.1/turn.3";

function snapshotWithTranscript(reasoning = "") {
  const snapshot = structuredClone(implementInterviewFixture.snapshot);
  for (const turn of [oldTurn, liveTurn]) {
    snapshot.transcripts[turn] = {
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
                id: `${turn}-message-1`,
                type: "assistant",
                time: {
                  created: snapshot.turns[turn].started,
                  completed: snapshot.turns[turn].ended || undefined,
                },
                content: [
                  {
                    type: "text",
                    text:
                      turn === oldTurn ? "Earlier recorded answer." : "Current recorded answer.",
                  },
                  ...(turn === liveTurn ? [{ type: "reasoning", text: reasoning }] : []),
                  {
                    type: "tool",
                    id: `${turn}-tool`,
                    callID: "cmd-1",
                    function: { name: "check" },
                  },
                ],
              },
              ...(turn === liveTurn
                ? [
                    {
                      id: `${turn}-message-2`,
                      type: "assistant",
                      content: [{ type: "text", text: "Latest recorded answer." }],
                    },
                  ]
                : []),
            ],
          },
        },
      },
      provenance: {},
    };
  }
  return snapshot;
}

async function renderFocus() {
  const snapshot = snapshotWithTranscript();
  const onsteer = vi.fn(async () => ({ ok: true, message: "sent" }));
  const onstop = vi.fn(async () => ({ ok: true, message: "stopped" }));
  const screen = await render(Page, {
    snapshot,
    observation: new RunObservation(snapshot),
    revision: 1,
    sessionID,
    onsteer,
    onstop,
  });
  return { screen, onsteer, onstop };
}

test("Focus browses recorded assistant moments, returns live, and flips the same card to exact input", async () => {
  const { screen } = await renderFocus();
  const card = screen.getByRole("region", { name: "Focus card" });
  await expect.element(card.getByText("Latest recorded answer.", { exact: true })).toBeVisible();
  await expect.element(card.getByText("Recorded activity", { exact: true })).toBeVisible();
  await expect
    .element(screen.getByText("2 messages · 1 tool calls", { exact: true }))
    .toBeVisible();

  await screen.getByRole("button", { name: /Latest recorded answer/ }).click();
  expect(document.querySelector(".return-live")).toBeNull();
  await screen.getByRole("button", { name: /Current recorded answer/ }).click();
  await expect.element(card.getByText("Current recorded answer.", { exact: true })).toBeVisible();
  await expect.element(screen.getByRole("button", { name: "Return to live" })).toBeVisible();
  await screen.getByRole("button", { name: "Return to live" }).click();
  await expect.element(card.getByText("Latest recorded answer.", { exact: true })).toBeVisible();

  await screen.getByRole("button", { name: "Prompt & context" }).click();
  await expect.element(screen.getByRole("region", { name: "Prompt and context" })).toBeVisible();
  await expect
    .element(
      screen.getByText(implementInterviewFixture.snapshot.turns[liveTurn].prompt, { exact: true }),
    )
    .toBeVisible();
  await screen.getByRole("button", { name: /Back to live/ }).click();
  await expect.element(card.getByText("Latest recorded answer.", { exact: true })).toBeVisible();
});

test("reasoning tokens do not become prose, and history steering and stop still target the running turn", async () => {
  const snapshot = snapshotWithTranscript("");
  snapshot.turn_usage[liveTurn] = {
    "gpt-5.6-luna": {
      input: 1,
      cache_read: 0,
      cache_write: 0,
      output: 2,
      reasoning: 999,
      stated_cost: 0,
    },
  };
  const onsteer = vi.fn(async () => ({ ok: true, message: "sent" }));
  const onstop = vi.fn(async (turn) => ({ ok: true, message: `stopped ${turn.id}` }));
  const screen = await render(Page, {
    snapshot,
    observation: new RunObservation(snapshot),
    revision: 1,
    sessionID,
    onsteer,
    onstop,
  });

  const card = screen.getByRole("region", { name: "Focus card" });
  await expect.element(card.getByText("Latest recorded answer.", { exact: true })).toBeVisible();
  await screen.getByRole("button", { name: /Current recorded answer/ }).click();
  await expect.element(card.getByText("Current recorded answer.", { exact: true })).toBeVisible();
  expect(card.element().querySelector(".reasoning")).toBeNull();

  await screen.getByRole("button", { name: /Turn 2/ }).click();
  await expect.element(card.getByText("Earlier recorded answer.", { exact: true })).toBeVisible();
  await screen.getByPlaceholder("Steer this turn…").fill("Please check the current change.");
  await screen.getByRole("button", { name: "Steer" }).click();
  expect(onsteer).toHaveBeenCalledWith({
    run: snapshot.run.id,
    session: sessionID,
    message: "Please check the current change.",
  });
  await screen.getByRole("button", { name: "Stop turn" }).click();
  expect(onstop).toHaveBeenCalledWith(snapshot.turns[liveTurn]);

  await screen.getByText("Full activity and turn details", { exact: true }).click();
  await expect.element(screen.getByRole("tab", { name: "Activity" })).toBeVisible();
  await expect.element(screen.getByRole("tab", { name: "Result" })).toBeVisible();
  await expect.element(screen.getByRole("tab", { name: "Usage" })).toBeVisible();
  await expect.element(screen.getByRole("tab", { name: "Source" })).toBeVisible();
  await screen.getByText("Session details and usage", { exact: true }).click();
  await expect.element(screen.getByText("Session total", { exact: true })).toBeVisible();
  await expect.element(screen.getByText("reasoning", { exact: true })).toBeVisible();
  const page = document.querySelector<HTMLElement>(".session-page");
  if (page) {
    page.style.height = "320px";
    page.style.flex = "none";
  }
  expect(page && page.scrollHeight > page.clientHeight).toBe(true);
});

test("a tool-only latest assistant message keeps the latest prose and labels current tool activity", async () => {
  const snapshot = snapshotWithTranscript();
  snapshot.transcripts[oldTurn].snapshot.state.message[sessionID] = [
    {
      id: "old-turn-prose",
      type: "assistant",
      content: [{ type: "text", text: "Older turn prose." }],
    },
  ];
  snapshot.transcripts[liveTurn].snapshot.state.message[sessionID].push({
    id: "latest-tools-only",
    type: "assistant",
    content: [{ type: "tool", id: "latest-tool", function: { name: "inspect" } }],
  });
  const screen = await render(Page, {
    snapshot,
    observation: new RunObservation(snapshot),
    revision: 1,
    sessionID,
  });
  const card = screen.getByRole("region", { name: "Focus card" });
  await expect.element(card.getByText("Latest recorded answer.", { exact: true })).toBeVisible();
  await expect.element(card.getByText("Current tool activity", { exact: true })).toBeVisible();
  await expect.element(card.getByText("1 tool call", { exact: true })).toBeVisible();
  await expect.element(card.getByRole("button", { name: /inspect/ })).toBeVisible();
  await screen.getByRole("button", { name: /Turn 2/ }).click();
  await expect.element(card.getByText("Older turn prose.", { exact: true })).toBeVisible();
  await expect.element(card.getByText("Current tool activity", { exact: true })).toBeVisible();
  await expect.element(card.getByText("1 tool call", { exact: true })).toBeVisible();
});

test("live Focus follows new moments and Return to live reaches the newest recorded update", async () => {
  const snapshot = snapshotWithTranscript();
  const screen = await render(Page, {
    snapshot,
    observation: new RunObservation(snapshot),
    revision: 1,
    sessionID,
  });
  const card = screen.getByRole("region", { name: "Focus card" });
  await expect.element(card.getByText("Latest recorded answer.", { exact: true })).toBeVisible();
  const updated = structuredClone(snapshot);
  updated.transcripts[liveTurn].snapshot.state.message[sessionID].push({
    id: "new-live-update",
    type: "assistant",
    content: [{ type: "text", text: "Newest live update." }],
  });
  await screen.rerender({
    snapshot: updated,
    observation: new RunObservation(updated),
    revision: 2,
    sessionID,
  });
  await expect.element(card.getByText("Newest live update.", { exact: true })).toBeVisible();
  await screen.getByRole("button", { name: /Current recorded answer/ }).click();
  await expect.element(screen.getByRole("button", { name: "Return to live" })).toBeVisible();
  const latest = structuredClone(updated);
  latest.transcripts[liveTurn].snapshot.state.message[sessionID].push({
    id: "newest-live-update",
    type: "assistant",
    content: [{ type: "text", text: "Most recent live update." }],
  });
  await screen.rerender({
    snapshot: latest,
    observation: new RunObservation(latest),
    revision: 3,
    sessionID,
  });
  await screen.getByRole("button", { name: "Return to live" }).click();
  await expect.element(card.getByText("Most recent live update.", { exact: true })).toBeVisible();
  expect(document.querySelector(".return-live")).toBeNull();
});

test("switching agents clears the previous agent's history selection", async () => {
  const snapshot = snapshotWithTranscript();
  const nextSessionID = Object.values(snapshot.sessions).find(
    (session) =>
      session.id !== sessionID &&
      Object.values(snapshot.turns).some((turn) => turn.session === session.id),
  )?.id;
  expect(nextSessionID).toBeTruthy();
  const nextTurn = Object.values(snapshot.turns).find((turn) => turn.session === nextSessionID)!;
  snapshot.transcripts[nextTurn.id] = {
    snapshot: {
      state: {
        info: {},
        family: {},
        active: {},
        pending: {},
        permission: {},
        form: {},
        message: {
          [nextSessionID!]: [
            {
              id: "other-agent-update",
              type: "assistant",
              content: [{ type: "text", text: "Other agent live update." }],
            },
          ],
        },
      },
    },
    provenance: {},
  };
  const screen = await render(Page, {
    snapshot,
    observation: new RunObservation(snapshot),
    revision: 1,
    sessionID,
  });
  await screen.getByRole("button", { name: /Turn 2/ }).click();
  await expect.element(screen.getByRole("button", { name: "Return to live" })).toBeVisible();
  await screen.rerender({
    snapshot,
    observation: new RunObservation(snapshot),
    revision: 2,
    sessionID: nextSessionID!,
  });
  await expect.element(screen.getByText("Other agent live update.", { exact: true })).toBeVisible();
  expect(document.querySelector(".return-live")).toBeNull();
});

test("a transcript without assistant prose says so explicitly", async () => {
  const snapshot = snapshotWithTranscript();
  for (const turn of [oldTurn, liveTurn]) {
    snapshot.transcripts[turn].snapshot.state.message[sessionID] = [
      {
        id: `${turn}-tools-only`,
        type: "assistant",
        content: [{ type: "tool", id: "tool", function: { name: "check" } }],
      },
    ];
  }
  const screen = await render(Page, {
    snapshot,
    observation: new RunObservation(snapshot),
    revision: 1,
    sessionID,
  });
  await expect
    .element(screen.getByText("No assistant prose recorded for this turn yet.", { exact: true }))
    .toBeVisible();
});
