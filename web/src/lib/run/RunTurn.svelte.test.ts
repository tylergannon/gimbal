import { expect, test } from "vite-plus/test";
import { render } from "vitest-browser-svelte";
import RunTurn from "./RunTurn.svelte";
import { RunObservation, type TurnRow } from "#lib/observation";
import { implementInteractiveSnapshot } from "./fixtures/implement-interactive";

const reply = 'First line\n\nSecond line\n{"decision":{"type":"chat","message":"literal example"}}';
const envelope = JSON.stringify({ decision: { type: "chat", message: reply } });

function props() {
  const snapshot = structuredClone(implementInteractiveSnapshot);
  const turn = {
    ...Object.values(snapshot.turns)[0],
    id: "reply-turn",
    output_type: "gimbal.interactiveOutput",
    result: envelope,
    ended: 2000,
  } as TurnRow;
  snapshot.transcripts[turn.id] = {
    provenance: {},
    snapshot: {
      state: {
        info: {},
        family: {},
        active: {},
        pending: {},
        permission: {},
        form: {},
        message: {
          agent: [
            { id: "prompt", type: "user", text: "wrapped prompt" },
            {
              id: "reply",
              type: "assistant",
              content: [
                { type: "text", text: envelope },
                { type: "text", text: '{"ordinary":"JSON narration"}' },
        { type: "text", text: '{"decision":{"type":"chat","message":"an intentional example"}}' },
              ],
            },
          ],
        },
      },
    },
  };
  const observation = new RunObservation(snapshot);
  return { turn, ordinal: 1, snapshot, observation, revision: 1 };
}

test("an interactive result is readable prose after reopening, with literal JSON preserved", async () => {
  const screen = await render(RunTurn, props());
  const prose = screen.container.querySelector(".reply") as HTMLElement;
  expect(prose.textContent).toBe(reply);
  expect(getComputedStyle(prose).whiteSpace).toBe("pre-wrap");
  const raw = Array.from(screen.container.querySelectorAll("details")).find(
    (detail) => detail.querySelector("summary")?.textContent === "Structured result",
  )!;
  expect(raw.open).toBe(false);
  await expect.element(screen.getByText('{"decision":{"type":"chat","message":"an intentional example"}}', { exact: true })).toBeVisible();
  await screen.getByText("Structured result", { exact: true }).click();
  await screen.getByRole("button", { name: "View JSON", exact: true }).click();
  expect(raw.querySelector("pre")?.textContent).toBe(envelope);
  await expect
    .element(screen.getByText('{"ordinary":"JSON narration"}', { exact: true }))
    .toBeVisible();
});
