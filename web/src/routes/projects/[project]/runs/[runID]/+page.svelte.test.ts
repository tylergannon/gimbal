import { afterEach, beforeEach, expect, test, vi } from "vite-plus/test";
import { render } from "vitest-browser-svelte";
import { implementInterviewFixture, planTripFixture } from "#lib/run/fixtures/index.js";
import type { ObservationDelta, RunSnapshot } from "#lib/observation/index.js";
import type { Graph } from "#lib/workflow/types.js";
import Page from "./+page.svelte";

vi.mock("$app/state", () => ({ page: { params: { project: "test-project" } } }));

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
  fixture: { graph: Graph; snapshot: RunSnapshot } = planTripFixture,
) {
  const snapshot = structuredClone(fixture.snapshot);
  const screen = await render(Page, {
    data: { snapshot, graph: JSON.stringify(fixture.graph) },
  });
  await expect.poll(() => TestEventSource.instances.length).toBe(1);
  return { screen, snapshot, stream: TestEventSource.instances[0] };
}

function multiAgentSnapshot() {
  const snapshot = structuredClone(planTripFixture.snapshot);
  const sessions = Object.values(snapshot.sessions);
  for (const [index, session] of sessions.entries()) {
    const id = `${session.id}/turn.1`;
    snapshot.turns[id] = {
      run: snapshot.run.id,
      id,
      session: session.id,
      scope: session.scope,
      prompt: `Review active task ${index + 1}.`,
      output_type: "text",
      result: "",
      error: "",
      interrupted: false,
      started: snapshot.run.started + index + 1,
      ended: 0,
      duration: 0,
    };
  }
  const firstTurn = Object.values(snapshot.turns)[0];
  const firstSession = snapshot.sessions[firstTurn.session];
  snapshot.transcripts[firstTurn.id] = {
    snapshot: {
      state: {
        info: {},
        family: {},
        active: {},
        message: {
          [firstSession.id]: [
            {
              id: "recorded-current-update",
              type: "assistant",
              content: [{ type: "text", text: "I compared the recorded lodging choices." }],
            },
          ],
        },
        pending: {},
        permission: {},
        form: {},
      },
    },
    provenance: {},
  };
  return snapshot;
}

test("multiple running agents default to Watchboard and remain visible without a graph", async () => {
  const snapshot = multiAgentSnapshot();
  const screen = await render(Page, { data: { snapshot, graph: "" } });
  await expect
    .element(
      screen
        .getByRole("navigation", { name: "Run views" })
        .getByRole("button", { name: "Watchboard" }),
    )
    .toHaveAttribute("aria-current", "page");
  await expect
    .element(screen.getByRole("button", { name: "Focus interviewer" }).first())
    .toBeVisible();
  await expect
    .element(screen.getByRole("button", { name: "Focus interviewer" }).last())
    .toBeVisible();
  await expect.element(screen.getByText("I compared the recorded lodging choices.")).toBeVisible();
  await expect.element(screen.getByText("No assistant prose recorded yet.")).toBeVisible();
  expect(document.body.textContent).not.toContain("Review active task 1.");
  await expect
    .element(screen.getByText("Workflow map unavailable; running agents remain visible above."))
    .toBeVisible();

  await screen.getByRole("button", { name: "Focus interviewer" }).last().click();
  await expect.element(screen.getByRole("region", { name: "Focus card" })).toBeVisible();
  await expect
    .element(screen.getByText("No assistant prose recorded for this turn yet."))
    .toBeVisible();
});

test("one running agent defaults to Focus and manual view choice survives live snapshots", async () => {
  const snapshot = multiAgentSnapshot();
  const turns = Object.values(snapshot.turns);
  delete snapshot.turns[turns[1].id];
  const { screen, stream } = await renderRunningPage({ graph: planTripFixture.graph, snapshot });
  const tabs = screen.getByRole("navigation", { name: "Run views" });
  await expect
    .element(tabs.getByRole("button", { name: "Focus" }))
    .toHaveAttribute("aria-current", "page");
  await tabs.getByRole("button", { name: "Map", exact: true }).click();
  const replacement = structuredClone(snapshot);
  replacement.position++;
  replacement.turns[turns[1].id] = { ...turns[1], started: turns[1].started + 2 };
  stream.send("snapshot", replacement);
  await expect
    .element(tabs.getByRole("button", { name: "Map", exact: true }))
    .toHaveAttribute("aria-current", "page");
});

test("Focus gives a clear no-session state when the run has no agent sessions", async () => {
  const snapshot = structuredClone(planTripFixture.snapshot);
  snapshot.sessions = {};
  snapshot.turns = {};
  snapshot.transcripts = {};
  const screen = await render(Page, {
    data: { snapshot, graph: JSON.stringify(planTripFixture.graph) },
  });
  await screen.getByRole("button", { name: "Focus" }).click();
  await expect.element(screen.getByRole("region", { name: "Focus has no session" })).toBeVisible();
  await expect
    .element(screen.getByRole("heading", { name: "No agent session selected" }))
    .toBeVisible();
  await expect
    .element(screen.getByText("No agent sessions have been recorded in this run."))
    .toBeVisible();
});

test("Watchboard non-agent map selection reveals the run Map detail", async () => {
  const { screen } = await renderRunningPage(implementInterviewFixture);
  await screen.getByRole("button", { name: "Watchboard" }).click();
  await expect.element(screen.getByRole("region", { name: "Watchboard" })).toBeVisible();
  await screen.getByRole("button", { name: "Select task-check" }).click();
  await expect
    .element(screen.getByRole("button", { name: "Map", exact: true }))
    .toHaveAttribute("aria-current", "page");
  await expect.element(screen.getByRole("heading", { name: "task-check" })).toBeVisible();
});

test("Map runtime selection rebinds across snapshots and clears when its row disappears", async () => {
  const { screen, stream } = await renderRunningPage(implementInterviewFixture);
  await screen.getByRole("button", { name: "Map", exact: true }).click();
  await screen.getByRole("button", { name: "Select coding" }).click();
  await expect.element(screen.getByRole("region", { name: "Focus card" })).toBeVisible();
  await screen.getByRole("button", { name: "Map", exact: true }).click();
  await expect
    .element(screen.getByRole("button", { name: "Select coding", pressed: true }))
    .toBeVisible();

  const refreshed = structuredClone(implementInterviewFixture.snapshot);
  refreshed.position++;
  stream.send("snapshot", refreshed);
  await expect
    .element(screen.getByRole("button", { name: "Select coding", pressed: true }))
    .toBeVisible();

  const missing = structuredClone(refreshed);
  missing.position++;
  for (const id of Object.keys(missing.turns).filter((id) => id.startsWith("coding.1/")))
    delete missing.turns[id];
  stream.send("snapshot", missing);
  await expect
    .element(screen.getByRole("button", { name: "Select coding" }))
    .toHaveAttribute("aria-pressed", "false");
});

test("a missing graph requires generation and a rebuilt serving binary", async () => {
  const snapshot = structuredClone(planTripFixture.snapshot);
  const screen = await render(Page, { data: { snapshot, graph: "" } });

  await expect.element(screen.getByText("Workflow graph required", { exact: true })).toBeVisible();
  await expect
    .element(screen.getByText(`No generated graph is registered for ${snapshot.run.name}`))
    .toBeVisible();
  await expect.element(screen.getByText("go generate ./...", { exact: true })).toBeVisible();
  await expect.element(screen.getByText("just build", { exact: true })).toBeVisible();
  await expect
    .element(screen.getByRole("link", { name: "Back to runs" }))
    .toHaveAttribute("href", "/projects/test-project");
  expect(document.querySelector('[aria-label="Recorded run history"]')).toBeNull();
  expect(document.querySelector('[aria-label$="workflow map"]')).toBeNull();
});

test("an incompatible registered graph requires regeneration and rebuild", async () => {
  const snapshot = structuredClone(planTripFixture.snapshot);
  const screen = await render(Page, {
    data: { snapshot, graph: JSON.stringify(implementInterviewFixture.graph) },
  });

  await expect
    .element(screen.getByText("The registered graph does not match this run", { exact: true }))
    .toBeVisible();
  await expect
    .element(
      screen.getByText(
        "Regenerate the workflow, then rebuild and restart the binary serving this project.",
        { exact: true },
      ),
    )
    .toBeVisible();
  expect(document.querySelector('[aria-label="Recorded run history"]')).toBeNull();
  expect(document.querySelector('[aria-label$="workflow map"]')).toBeNull();
});

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
  await screen.getByRole("button", { name: "Map", exact: true }).click();
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

test("Map agent selection opens Focus and remains selected through live snapshots", async () => {
  const { screen, snapshot } = await renderRunningPage(implementInterviewFixture);
  await screen.getByRole("button", { name: "Map", exact: true }).click();
  await screen.getByRole("button", { name: "Select coding" }).click();
  await expect.element(screen.getByRole("region", { name: "Focus card" })).toBeVisible();

  const refreshed = structuredClone(snapshot);
  refreshed.position++;
  refreshed.scopes["implementation.1/task.3"].values.refresh = { value: "fresh data" };
  await screen.rerender({
    data: { snapshot: refreshed, graph: JSON.stringify(implementInterviewFixture.graph) },
  });
  await expect.element(screen.getByRole("region", { name: "Focus card" })).toBeVisible();

  const missing = structuredClone(refreshed);
  missing.position++;
  delete missing.turns["coding.1/turn.3"];
  await screen.rerender({
    data: { snapshot: missing, graph: JSON.stringify(implementInterviewFixture.graph) },
  });
  await expect.element(screen.getByRole("region", { name: "Focus card" })).toBeVisible();
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
