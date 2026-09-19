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

test("instance selection works when runtime scope names include ordinals", async () => {
  const snapshot = structuredClone(implementInterviewFixture.snapshot);
  for (const scope of Object.values(snapshot.scopes)) {
    if (scope.key) scope.name = scope.key.split("/").at(-1) ?? scope.name;
  }
  const screen = await render(Map, { graph: implementInterviewFixture.graph, snapshot });

  const trigger = screen.getByRole("button", { name: "Select task.3 instance" });
  await trigger.click();
  await screen.getByRole("option", { name: "task 2 of 3" }).click();

  await expect
    .element(screen.getByRole("button", { name: "Select task.2 instance" }))
    .toHaveTextContent("task 2 of 3");
  expect(document.querySelector('[data-scope="implementation.1/task.2"]')).not.toBeNull();
  await expect
    .element(screen.getByRole("button", { name: "Select task-check" }))
    .toHaveTextContent("exit 1");
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

test("interrupted map records are ended while a genuine command failure remains failed", async () => {
  const snapshot = structuredClone(implementInterviewFixture.snapshot);
  snapshot.turns["coding.1/turn.2"].interrupted = true;
  snapshot.turns["coding.1/turn.2"].error = "turn stopped";
  snapshot.commands["implementation.1/task.2/task-check.1"].interrupted = true;
  snapshot.commands["implementation.1/task.2/task-check.1"].error = "command stopped";
  snapshot.commands["implementation.1/task.2/build.1"].exit_code = 2;
  snapshot.commands["implementation.1/task.2/build.1"].error = "build failed";

  const screen = await render(Map, { graph: implementInterviewFixture.graph, snapshot });
  const trigger = screen.getByRole("button", { name: "Select task instance" });
  await trigger.click();
  await screen.getByRole("option", { name: "task 2 of 3" }).click();

  expect(
    document.querySelector('button[aria-label="Select coding"] [data-state="ended"]'),
  ).not.toBeNull();
  expect(
    document.querySelector('button[aria-label="Select task-check"] [data-state="ended"]'),
  ).not.toBeNull();
  expect(
    document.querySelector('button[aria-label="Select build"] [data-state="failed"]'),
  ).not.toBeNull();
});

test("a folded live scope advances its visible elapsed time while idle", async () => {
  const snapshot = structuredClone(planTripFixture.snapshot);
  const now = Date.now();
  snapshot.run.started = now - 75_000;
  snapshot.scopes["research.1"].began = now - 65_000;

  const screen = await render(Map, { graph: planTripFixture.graph, snapshot });
  await screen.getByRole("button", { name: "Fold research" }).click();
  const elapsed = (): Element => {
    const element = document.querySelector('[data-scope="research.1"] .fold-summary .elapsed');
    expect(element, "folded live elapsed duration should be rendered").not.toBeNull();
    if (!element) throw new Error("folded live elapsed duration is missing");
    return element;
  };
  const before = elapsed().textContent;
  expect(before).toMatch(/^\d+m \d{2}s$/);

  await new Promise((resolve) => setTimeout(resolve, 1_250));

  expect(elapsed().textContent).not.toBe(before);
});

test("a folded live scope advances while snapshots arrive faster than its clock", async () => {
  const snapshot = structuredClone(planTripFixture.snapshot);
  const now = Date.now();
  snapshot.run.started = now - 75_000;
  snapshot.scopes["research.1"].began = now - 65_000;

  const screen = await render(Map, { graph: planTripFixture.graph, snapshot });
  await screen.getByRole("button", { name: "Fold research" }).click();
  const elapsed = (): Element => {
    const element = document.querySelector('[data-scope="research.1"] .fold-summary .elapsed');
    expect(element, "folded live elapsed duration should be rendered").not.toBeNull();
    if (!element) throw new Error("folded live elapsed duration is missing");
    return element;
  };
  const before = elapsed().textContent;

  for (let update = 1; update <= 6; update++) {
    await new Promise((resolve) => setTimeout(resolve, 250));
    const refreshed = structuredClone(snapshot);
    refreshed.position += update;
    await screen.rerender({ graph: planTripFixture.graph, snapshot: refreshed });
  }

  expect(elapsed().textContent).not.toBe(before);
});

test("a folded ended or recorded scope keeps its visible elapsed time fixed", async () => {
  const snapshot = structuredClone(planTripFixture.snapshot);
  const now = Date.now();
  snapshot.run.started = now - 75_000;
  snapshot.scopes["research.1"].began = now - 65_000;
  snapshot.scopes["research.1"].ended = now - 20_000;

  const screen = await render(Map, { graph: planTripFixture.graph, snapshot });
  await screen.getByRole("button", { name: "Fold research" }).click();
  const elapsed = (): Element => {
    const element = document.querySelector('[data-scope="research.1"] .fold-summary .elapsed');
    expect(element, "folded elapsed duration should be rendered").not.toBeNull();
    if (!element) throw new Error("folded elapsed duration is missing");
    return element;
  };
  const endedBefore = elapsed().textContent;
  expect(endedBefore).toBe("0m 45s");

  await new Promise((resolve) => setTimeout(resolve, 1_250));
  expect(elapsed().textContent).toBe(endedBefore);

  const recorded = structuredClone(snapshot);
  recorded.run.status = "completed";
  recorded.run.ended = now - 1_000;
  recorded.scopes["research.1"].ended = 0;
  for (const scope of Object.values(recorded.scopes)) scope.began = now - 70_000;
  recorded.scopes["research.1"].began = now - 65_000;
  await screen.rerender({ graph: planTripFixture.graph, snapshot: recorded });
  const recordedBefore = elapsed().textContent;
  expect(recordedBefore).toBe("1m 04s");

  await new Promise((resolve) => setTimeout(resolve, 1_250));
  expect(elapsed().textContent).toBe(recordedBefore);
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
