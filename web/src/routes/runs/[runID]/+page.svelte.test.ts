import { afterEach, beforeEach, expect, test } from "vite-plus/test";
import { render } from "vitest-browser-svelte";
import { implementInterviewFixture, planTripFixture } from "#lib/run/fixtures/index.js";
import type { ObservationDelta, RunSnapshot } from "#lib/observation/index.js";
import Page from "./+page.svelte";

class TestEventSource {
  static instances: TestEventSource[] = [];

  readonly url: string;
  onopen: ((event: Event) => void) | null = null;
  onerror: ((event: Event) => void) | null = null;
  private listeners = new Map<string, EventListener[]>();

  constructor(url: string | URL) {
    this.url = String(url);
    TestEventSource.instances.push(this);
  }

  addEventListener(type: string, listener: EventListener) {
    const listeners = this.listeners.get(type) ?? [];
    listeners.push(listener);
    this.listeners.set(type, listeners);
  }

  close() {}

  open() {
    this.onopen?.(new Event("open"));
  }

  fail() {
    this.onerror?.(new Event("error"));
  }

  send(type: "delta" | "snapshot", data: ObservationDelta | RunSnapshot) {
    const event = new MessageEvent(type, { data: JSON.stringify(data) });
    for (const listener of this.listeners.get(type) ?? []) listener(event);
  }
}

const originalEventSource = globalThis.EventSource;

beforeEach(() => {
  TestEventSource.instances = [];
  globalThis.EventSource = TestEventSource as unknown as typeof EventSource;
});

afterEach(() => {
  globalThis.EventSource = originalEventSource;
});

async function renderRunningPage(
  fixture: typeof planTripFixture | typeof implementInterviewFixture = planTripFixture,
) {
  const snapshot = structuredClone(fixture.snapshot);
  const screen = await render(Page, {
    data: { snapshot, graph: JSON.stringify(fixture.graph) },
  });
  await expect.poll(() => TestEventSource.instances.length).toBe(1);
  return { screen, snapshot, stream: TestEventSource.instances[0] };
}

test("topbar navigation reveals folded current activity and keeps map and detail coordinated", async () => {
  const { screen } = await renderRunningPage();
  await screen.getByRole("button", { name: "Fold research" }).click();
  await expect.element(screen.getByRole("button", { name: "Open research" })).toBeVisible();

  await screen.getByRole("button", { name: "2 questions waiting" }).click();

  await expect.element(screen.getByRole("button", { name: "Fold research" })).toBeVisible();
  const selected = document.querySelector(
    'button[aria-label="Select preferences"][aria-pressed="true"]',
  );
  expect(selected).not.toBeNull();
  expect(selected?.closest("[data-selection-key]")?.getAttribute("data-selection-key")).toContain(
    "research.1/transport.1",
  );
  await expect.element(screen.getByRole("heading", { name: "preferences" })).toBeVisible();
});

test("search selects an old turn through a folded loop instance", async () => {
  const { screen } = await renderRunningPage(implementInterviewFixture);
  await screen.getByRole("button", { name: "Fold implementation" }).click();
  await expect.element(screen.getByRole("button", { name: "Open implementation" })).toBeVisible();

  const search = screen.getByRole("textbox", { name: "Find a scope, turn, or command" });
  await search.fill("coding turn 1");
  await screen.getByRole("option", { name: /coding · turn 1/ }).click();

  await expect.element(screen.getByRole("button", { name: "Fold implementation" })).toBeVisible();
  const selected = screen.getByRole("button", { name: "Select coding", pressed: true });
  await expect.element(selected).toBeVisible();
  expect(
    selected.element().closest("[data-selection-key]")?.getAttribute("data-selection-key"),
  ).toContain("implementation.1/task.1");
  await expect.element(screen.getByRole("heading", { name: "coding" })).toBeVisible();
});

test("selection survives same-run replacement and resets when its runtime disappears", async () => {
  const { screen, snapshot } = await renderRunningPage(implementInterviewFixture);
  await screen.getByRole("button", { name: "Select coding" }).click();
  await expect
    .element(screen.getByRole("button", { name: "Select coding", pressed: true }))
    .toBeVisible();

  const refreshed = structuredClone(snapshot);
  refreshed.position++;
  refreshed.scopes["implementation.1/task.3"].values.refresh = { value: "fresh data" };
  await screen.rerender({
    data: { snapshot: refreshed, graph: JSON.stringify(implementInterviewFixture.graph) },
  });
  await expect
    .element(screen.getByRole("button", { name: "Select coding", pressed: true }))
    .toBeVisible();
  expect(document.querySelector("aside .empty-selection")).toBeNull();

  const missing = structuredClone(refreshed);
  missing.position++;
  delete missing.turns["coding.1/turn.3"];
  await screen.rerender({
    data: { snapshot: missing, graph: JSON.stringify(implementInterviewFixture.graph) },
  });
  await expect
    .element(screen.getByRole("button", { name: "Select coding" }))
    .toHaveAttribute("aria-pressed", "false");
  expect(document.querySelector("aside .empty-selection")).not.toBeNull();
});

test("live elapsed time advances without events and fixes at the recorded end", async () => {
  const now = Date.now();
  const fixture = structuredClone(planTripFixture);
  fixture.snapshot.run.started = now - 65_000;
  const { screen, snapshot, stream } = await renderRunningPage(fixture);
  const elapsed = document.querySelector(".topbar .elapsed");
  expect(elapsed).not.toBeNull();
  const before = elapsed?.textContent;
  await new Promise((resolve) => setTimeout(resolve, 1_100));
  expect(elapsed?.textContent).not.toBe(before);

  const recorded = structuredClone(snapshot);
  recorded.position++;
  recorded.run.status = "completed";
  recorded.run.ended = now;
  stream.send("snapshot", recorded);
  await expect.element(screen.getByText("Recorded run · nothing here is live")).toBeVisible();
  const fixed = elapsed?.textContent;
  await new Promise((resolve) => setTimeout(resolve, 1_100));
  expect(elapsed?.textContent).toBe(fixed);
});

test("an open connection followed by one changed delta renders that update", async () => {
  const { screen, snapshot, stream } = await renderRunningPage();
  await expect.element(screen.getByText("Connecting", { exact: true })).toBeVisible();

  stream.open();
  await expect.element(screen.getByText("Live", { exact: true })).toBeVisible();

  const key = "01M2RWXNFFYQA4HHQWMQ252CYF";
  const answered = structuredClone(snapshot.interviews[key]);
  answered.status = "answered";
  answered.answer = "A single streamed answer.";
  answered.answered = Date.now();
  stream.send("delta", {
    stream: snapshot.stream,
    position: snapshot.position + 1,
    frames: [{ type: "row", data: { table: "interviews", key, row: answered } }],
  });

  await expect
    .poll(() => document.querySelector(".current")?.textContent?.trim())
    .toBe("1 question waiting");
});

test("an outage keeps aging across retry, recovers, and one replacement becomes recorded", async () => {
  const { screen, snapshot, stream } = await renderRunningPage();
  stream.open();
  await expect.element(screen.getByText("Live", { exact: true })).toBeVisible();

  stream.fail();
  await expect.element(screen.getByText("Disconnected 0 s", { exact: true })).toBeVisible();
  await expect.poll(() => TestEventSource.instances.length).toBe(2);
  await new Promise((resolve) => setTimeout(resolve, 1_100));
  await expect.element(screen.getByText(/Disconnected [1-9]\d* s/)).toBeVisible();

  const retry = TestEventSource.instances[1];
  retry.open();
  await expect.element(screen.getByText("Live", { exact: true })).toBeVisible();

  const recorded = structuredClone(snapshot);
  recorded.position++;
  recorded.run.status = "completed";
  recorded.run.ended = Date.now();
  retry.send("snapshot", recorded);

  await expect.element(screen.getByText("Completed", { exact: true })).toBeVisible();
  await expect.element(screen.getByText("Recorded run · nothing here is live")).toBeVisible();
});
