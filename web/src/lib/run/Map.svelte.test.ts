import { expect, test } from "vite-plus/test";
import { userEvent } from "vitest/browser";
import { render } from "vitest-browser-svelte";
import Map, { type MapSelection } from "./Map.svelte";
import { implementInterviewFixture, planTripFixture } from "./fixtures/index.js";

test("pointer and keyboard instance selection replace the rendered runtime facts", async () => {
  const selections: MapSelection[] = [];
  const screen = await render(Map, {
    ...implementInterviewFixture,
    onselect: (selection) => selections.push(selection),
  });
  const trigger = screen.getByRole("button", { name: "Select task instance" });

  await trigger.click();
  await screen.getByRole("option", { name: "task 2 of 3" }).click();
  await expect.element(trigger).toHaveTextContent("task 2 of 3");
  expect(
    document.querySelector('button[aria-label="Select coding"] [data-state="ended"]'),
  ).not.toBeNull();
  await expect
    .element(screen.getByRole("button", { name: "Select task-check" }))
    .toHaveTextContent("exit 1");
  expect(selections.at(-1)?.kind).toBe("instance");

  await trigger.click();
  await userEvent.keyboard("{ArrowDown}{Enter}");
  await expect.element(trigger).toHaveTextContent("task 3 of 3");
  await expect
    .element(screen.getByRole("button", { name: "Select task-check" }))
    .toHaveTextContent("not started");

  const refreshed = structuredClone(implementInterviewFixture.snapshot);
  refreshed.turns["coding.1/turn.3"].ended = refreshed.turns["coding.1/turn.3"].started + 120_000;
  refreshed.turns["coding.1/turn.3"].duration = 120_000;
  await screen.rerender({
    ...implementInterviewFixture,
    snapshot: refreshed,
    onselect: (selection) => selections.push(selection),
  });
  expect(
    document.querySelector('button[aria-label="Select coding"] [data-state="ended"]'),
  ).not.toBeNull();
});

test("nodes, watchers, and sheets report selection and folded summaries stay truthful", async () => {
  const selections: MapSelection[] = [];
  const screen = await render(Map, {
    ...implementInterviewFixture,
    onselect: (selection) => selections.push(selection),
  });

  await screen.getByRole("button", { name: "Select coding" }).click();
  expect(selections.at(-1)?.kind).toBe("node");
  const watcher = screen.getByRole("button", {
    name: "Select watcher architectural-critique",
  });
  await watcher.click();
  expect(selections.at(-1)?.kind).toBe("watcher");
  await expect.element(watcher).toHaveAttribute("aria-pressed", "true");
  expect(
    document.querySelector('[data-scope="implementation.1/task.3"].path.selected'),
  ).not.toBeNull();

  const taskTrigger = screen.getByRole("button", { name: "Select task instance" });
  await taskTrigger.click();
  await screen.getByRole("option", { name: "task 2 of 3" }).click();
  await screen.getByRole("button", { name: "Fold task" }).click();

  const taskSheet = document.querySelector('[data-scope="implementation.1/task.2"]');
  expect(taskSheet).not.toBeNull();
  expect(taskSheet?.querySelectorAll('[data-state="failed"]')).toHaveLength(1);
  await screen.getByRole("button", { name: "Open task" }).click();
  await expect.element(screen.getByRole("button", { name: "Select coding" })).toBeVisible();
  expect(selections.at(-1)?.kind).toBe("sheet");
});

test("opening a sheet folds its sibling while the root remains permanent", async () => {
  const screen = await render(Map, planTripFixture);

  await screen.getByRole("button", { name: "Fold lodging" }).click();
  await screen.getByRole("button", { name: "Open lodging" }).click();
  await expect.element(screen.getByRole("button", { name: "Open transport" })).toBeVisible();
  expect(
    document.querySelector(`button[aria-label="Fold ${planTripFixture.graph.name}"]`),
  ).toBeNull();
});
