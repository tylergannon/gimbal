import { afterEach, beforeEach, expect, test } from "vite-plus/test";
import { render } from "vitest-browser-svelte";
import { planTripFixture } from "#lib/run/fixtures/index.js";
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

async function renderRunningPage() {
  const snapshot = structuredClone(planTripFixture.snapshot);
  const screen = await render(Page, {
    data: { snapshot, graph: JSON.stringify(planTripFixture.graph) },
  });
  await expect.poll(() => TestEventSource.instances.length).toBe(1);
  return { screen, snapshot, stream: TestEventSource.instances[0] };
}

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
