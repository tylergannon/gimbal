import { expect, test } from "vite-plus/test";
import { render } from "vitest-browser-svelte";
import Payload from "./Payload.svelte";

test("a planner result reads as task cards, with raw JSON available on demand", async () => {
  const result = JSON.stringify({
    tasks: [
      {
        name: "Wire the browser to saved games",
        description: "Load the complete saved board and scores.",
        definition_of_done: "A fresh URL restores the game.",
      },
    ],
  });
  const screen = await render(Payload, { label: "result", text: result });

  await expect.element(screen.getByText("Wire the browser to saved games")).toBeVisible();
  await expect.element(screen.getByText("Load the complete saved board and scores.")).toBeVisible();
  await expect.element(screen.getByText("Definition of done")).toBeVisible();
  expect(document.querySelector(".scroller")?.textContent).not.toContain('"tasks"');

  await screen.getByRole("button", { name: "View JSON" }).click();
  await expect.element(screen.getByText(/"tasks"/)).toBeVisible();
  await screen.getByRole("button", { name: "Readable" }).click();
  await expect.element(screen.getByText("Wire the browser to saved games")).toBeVisible();
});

test("a quoted text result shows actual line breaks instead of JSON escapes", async () => {
  const screen = await render(Payload, {
    label: "result",
    text: JSON.stringify("Implemented the change.\n\nObserved it in the browser."),
    prose: true,
  });
  await expect
    .element(screen.getByText("Implemented the change.\n\nObserved it in the browser."))
    .toBeVisible();
  expect(document.querySelector(".scroller")?.textContent).not.toContain("\\n");
});
