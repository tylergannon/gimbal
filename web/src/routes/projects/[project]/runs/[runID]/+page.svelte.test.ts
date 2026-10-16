import { afterEach, beforeEach, expect, test, vi } from "vite-plus/test";
import { error as httpError } from "@sveltejs/kit";
import { render } from "vitest-browser-svelte";
import { implementInterviewFixture, planTripFixture } from "#lib/run/fixtures/index.js";
import type { ObservationDelta, RunSnapshot } from "#lib/observation/index.js";
import type { Graph } from "#lib/workflow/types.js";
import Page from "./+page.svelte";
import RunView from "#lib/run/RunView.svelte";
import RunPanel from "#lib/workspace/RunPanel.svelte";

const auth = vi.hoisted(() => ({
  page: { params: { project: "test-project" }, data: { authEnabled: false } },
  currentUser: vi.fn(),
  viewRun: vi.fn(),
  goto: vi.fn(() => Promise.resolve()),
  answer: vi.fn(),
  after: undefined as undefined | (() => void),
}));
vi.mock("$app/state", () => ({ page: auth.page }));
vi.mock("$app/navigation", () => ({
  goto: auth.goto,
  invalidateAll: vi.fn(),
  afterNavigate: (callback: () => void) => {
    auth.after = callback;
  },
  beforeNavigate: vi.fn(),
  onNavigate: vi.fn(),
  invalidate: vi.fn(),
  refreshAll: vi.fn(),
  preloadData: vi.fn(),
  preloadCode: vi.fn(),
  disableScrollHandling: vi.fn(),
  pushState: vi.fn(),
  replaceState: vi.fn(),
}));
vi.mock("../../../../auth.remote", () => ({
  currentUser: auth.currentUser,
  logout: vi.fn(),
  beginLogin: vi.fn(),
}));

vi.mock("#lib/runs.remote", () => ({ viewRun: auth.viewRun }));
vi.mock("#lib/environments.remote", () => ({
  environmentSnapshot: () => ({ current: undefined }),
}));

vi.mock("../../../../interview.remote", () => ({
  answerInterview: {
    for: () => ({
      pending: 0,
      fields: {
        ...Object.fromEntries(
          ["run", "question_id", "answer"].map((name) => [
            name,
            {
              as: (type: string, value?: string) => ({ name, type, value }),
              value: () => "",
              set: () => {},
            },
          ]),
        ),
        allIssues: () => [],
      },
      enhance: (handler: (value: unknown) => Promise<void>) => ({
        onsubmit: async (event: SubmitEvent) => {
          event.preventDefault();
          auth.answer(Object.fromEntries(new FormData(event.currentTarget as HTMLFormElement)));
          await handler({ submit: async () => true, result: { accepted: true } });
        },
      }),
    }),
  },
}));

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

  closed = false;
  close() {
    this.closed = true;
  }

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
  auth.page.data.authEnabled = false;
  auth.currentUser.mockReset();
  auth.viewRun.mockReturnValue({ current: undefined, error: undefined });
  auth.goto.mockClear();
  auth.answer.mockClear();
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
  await screen.getByRole("button", { name: "Map", exact: true }).first().click();
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

function mapRegion(screen: { getByRole: (role: "region", options: { name: RegExp }) => unknown }) {
  return screen.getByRole("region", { name: /workflow map/ }) as never;
}

function step(name: string) {
  return document.querySelector<HTMLButtonElement>(`.svelte-flow button[aria-label^="${name},"]`);
}

async function stepButton(name: string) {
  await expect.poll(() => step(name)).not.toBeNull();
  return step(name)!;
}

async function press(label: string) {
  const find = () => document.querySelector<HTMLButtonElement>(`button[aria-label="${label}"]`);
  await expect.poll(find).not.toBeNull();
  find()!.click();
}

function openSelectedSession() {
  document.body.dispatchEvent(
    new KeyboardEvent("keydown", { key: "o", bubbles: true, cancelable: true }),
  );
}

test("a run opens as a transcript and retains its workflow map, drawn with the designer's shapes", async () => {
  const snapshot = multiAgentSnapshot();
  const screen = await render(Page, {
    data: { snapshot, graph: JSON.stringify(planTripFixture.graph) },
  });
  await screen.getByRole("button", { name: "Map", exact: true }).first().click();
  await expect.element(mapRegion(screen)).toBeVisible();
  expect(document.querySelector('nav[aria-label="Run views"]')).toBeNull();
  const preferences = await stepButton("preferences");
  expect(preferences.closest(".node")?.getAttribute("data-selection-key")).toContain(
    "research.1/lodging.1",
  );
});

test("transcript opens the waiting session, shows actual invocation and answers inline", async () => {
  const snapshot = multiAgentSnapshot();
  const session = Object.values(snapshot.sessions)[0];
  const question = Object.values(snapshot.interviews).find((row) => row.status === "pending")!;
  question.session = session.id;
  snapshot.scopes[""].values.goal = { value: "Choose a quiet cabin with reliable Wi-Fi." };
  session.steers = [
    {
      message: "Keep the cabin comparison focused.",
      source: "person",
      landed: true,
      created: snapshot.run.started + 1,
    },
  ];
  const screen = await render(Page, {
    data: { snapshot, graph: JSON.stringify(planTripFixture.graph) },
  });
  await expect.element(screen.getByRole("region", { name: "Invocation" })).toBeVisible();
  await expect.element(screen.getByText("Choose a quiet cabin with reliable Wi-Fi.")).toBeVisible();
  await expect.element(screen.getByText(`Answer ${session.name}`, { exact: true })).toBeVisible();
  await expect
    .element(screen.getByRole("textbox", { name: "Answer the waiting question" }))
    .toBeVisible();
  expect(document.querySelector('.switcher button[aria-pressed="true"]')?.textContent).toContain(
    session.name,
  );
  expect(document.body.textContent).toContain("Keep the cabin comparison focused.");
  expect(document.body.textContent).not.toContain("*claude.adapter");
  await screen
    .getByRole("textbox", { name: "Answer the waiting question" })
    .fill("Choose the cabin.");
  await screen.getByRole("button", { name: "Send answer", exact: true }).click();
  expect(auth.answer).toHaveBeenCalledWith({
    run: snapshot.run.id,
    question_id: question.question_id,
    answer: "Choose the cabin.",
  });
  await expect.element(screen.getByText("Answer accepted.", { exact: true })).toBeVisible();
  await expect.element(screen.getByText(`Answer ${session.name}`, { exact: true })).toBeVisible();
  expect(document.querySelector('.run-head .pip[data-state="needs-you"]')).not.toBeNull();
  await expect.poll(() => TestEventSource.instances.length).toBe(1);
  const remaining = Object.values(snapshot.interviews)
    .filter((row) => row.status === "pending")
    .map((row) => ({
      ...row,
      status: "answered" as const,
      answer: "Choose the cabin.",
      answered: Date.now(),
    }));
  TestEventSource.instances[0].send("delta", {
    stream: snapshot.stream,
    position: snapshot.position + 1,
    frames: remaining.map((row) => ({
      type: "row" as const,
      data: { table: "interviews" as const, key: row.question_id, row },
    })),
  });
  await expect
    .element(screen.getByRole("textbox", { name: "Answer the waiting question" }))
    .not.toBeInTheDocument();
  await expect
    .poll(() => document.querySelector('.run-head .pip[data-state="needs-you"]'))
    .toBeNull();
});

test("a saved source page draws the designer's titles over the run", async () => {
  const snapshot = structuredClone(implementInterviewFixture.snapshot);
  const { sourceFromGraph } = await import("#lib/run/runSource.js");
  const source = sourceFromGraph(implementInterviewFixture.graph);
  const coding = JSON.stringify(source).replace('"title":"coding"', '"title":"Implement the task"');
  const screen = await render(Page, {
    data: { snapshot, graph: JSON.stringify(implementInterviewFixture.graph), source: coding },
  });
  await screen.getByRole("button", { name: "Map", exact: true }).first().click();
  await expect.element(mapRegion(screen)).toBeVisible();
  const button = await stepButton("Implement the task");
  button.click();
  await expect.element(screen.getByRole("region", { name: "Agent chat" })).toBeVisible();
});

test("a command step in an earlier task instance selects into the detail pane", async () => {
  const { screen } = await renderRunningPage(implementInterviewFixture);
  const find = () =>
    document.querySelector<HTMLSelectElement>('select[aria-label="implementation instance"]');
  await expect.poll(find).not.toBeNull();
  const instances = find()!;
  instances.value = instances.options[0].value;
  instances.dispatchEvent(new Event("change", { bubbles: true }));
  (await stepButton("task-check")).click();
  await expect.element(screen.getByRole("heading", { name: "task-check" })).toBeVisible();
});

test("Map runtime selection rebinds across snapshots and clears when its row disappears", async () => {
  const { screen, stream } = await renderRunningPage(implementInterviewFixture);
  (await stepButton("coding")).click();
  await expect.element(screen.getByRole("region", { name: "Agent chat" })).toBeVisible();
  await screen.getByRole("button", { name: "Close" }).first().click();
  await expect.poll(() => step("coding")?.getAttribute("aria-pressed")).toBe("true");

  const refreshed = structuredClone(implementInterviewFixture.snapshot);
  refreshed.position++;
  stream.send("snapshot", refreshed);
  await expect.poll(() => step("coding")?.getAttribute("aria-pressed")).toBe("true");

  const missing = structuredClone(refreshed);
  missing.position++;
  for (const id of Object.keys(missing.turns).filter((id) => id.startsWith("coding.1/")))
    delete missing.turns[id];
  stream.send("snapshot", missing);
  await expect.poll(() => step("coding")?.getAttribute("aria-pressed")).toBe("false");
});

test("a missing saved graph reports the missing run data", async () => {
  const snapshot = structuredClone(planTripFixture.snapshot);
  const screen = await render(Page, { data: { snapshot, graph: "" } });

  await screen.getByRole("button", { name: "Map", exact: true }).first().click();
  await expect.element(screen.getByText("Workflow graph required", { exact: true })).toBeVisible();
  await expect
    .element(screen.getByText(`No workflow graph was saved for ${snapshot.run.name}`))
    .toBeVisible();
  await expect
    .element(screen.getByRole("link", { name: "Back to runs" }))
    .toHaveAttribute("href", "/projects/test-project");
  expect(document.querySelector('[aria-label="Recorded run history"]')).toBeNull();
  expect(document.querySelector('[aria-label$="workflow map"]')).toBeNull();
});

test("an incompatible saved graph reports the mismatch", async () => {
  const snapshot = structuredClone(planTripFixture.snapshot);
  const screen = await render(Page, {
    data: { snapshot, graph: JSON.stringify(implementInterviewFixture.graph) },
  });

  await screen.getByRole("button", { name: "Map", exact: true }).first().click();
  await expect
    .element(screen.getByText("The saved graph does not match this run", { exact: true }))
    .toBeVisible();
  await expect
    .element(
      screen.getByText(
        "The graph must come from this run. Restarting the server cannot replace it.",
        { exact: true },
      ),
    )
    .toBeVisible();
  expect(document.querySelector('[aria-label="Recorded run history"]')).toBeNull();
  expect(document.querySelector('[aria-label$="workflow map"]')).toBeNull();
});

test("topbar navigation reveals folded current activity and keeps map and detail coordinated", async () => {
  const { screen } = await renderRunningPage();
  await press("Collapse research");
  await expect
    .poll(() => document.querySelector('button[aria-label="Expand research"]'))
    .not.toBeNull();

  await screen.getByRole("button", { name: "2 questions waiting" }).click();

  await expect
    .poll(() => document.querySelector('button[aria-label="Collapse research"]'))
    .not.toBeNull();
  await expect
    .poll(() => document.querySelector(".node.chosen")?.getAttribute("data-selection-key"))
    .toContain("research.1/transport.1");
  await expect.element(screen.getByRole("heading", { name: "preferences" })).toBeVisible();
});

test("search selects an old turn through a collapsed loop instance", async () => {
  const { screen } = await renderRunningPage(implementInterviewFixture);
  await press("Collapse implementation");
  await expect
    .poll(() => document.querySelector('button[aria-label="Expand implementation"]'))
    .not.toBeNull();

  const search = screen.getByRole("textbox", { name: "Find a scope, turn, or command" });
  await search.fill("coding turn 1");
  await screen.getByRole("option", { name: /coding · turn 1/ }).click();

  await expect
    .poll(() => document.querySelector('button[aria-label="Collapse implementation"]'))
    .not.toBeNull();
  await expect.poll(() => step("coding")?.getAttribute("aria-pressed")).toBe("true");
  expect(step("coding")?.closest(".node")?.getAttribute("data-selection-key")).toContain(
    "implementation.1/task.1",
  );
  await expect.element(screen.getByRole("heading", { name: "coding" })).toBeVisible();
});

test("Map agent selection opens the session and it stays open through live snapshots", async () => {
  const { screen, snapshot } = await renderRunningPage(implementInterviewFixture);
  (await stepButton("coding")).click();
  await expect.element(screen.getByRole("region", { name: "Agent chat" })).toBeVisible();

  const refreshed = structuredClone(snapshot);
  refreshed.position++;
  refreshed.scopes["implementation.1/task.3"].values.refresh = { value: "fresh data" };
  await screen.rerender({
    data: { snapshot: refreshed, graph: JSON.stringify(implementInterviewFixture.graph) },
  });
  await expect.element(screen.getByRole("region", { name: "Agent chat" })).toBeVisible();

  const missing = structuredClone(refreshed);
  missing.position++;
  delete missing.turns["coding.1/turn.3"];
  await screen.rerender({
    data: { snapshot: missing, graph: JSON.stringify(implementInterviewFixture.graph) },
  });
  await expect.element(screen.getByRole("region", { name: "Agent chat" })).toBeVisible();
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

test("an authenticated reconnect waits for a fresh session probe", async () => {
  auth.page.data.authEnabled = true;
  let accept!: () => void;
  const refresh = vi.fn(
    () =>
      new Promise<void>((resolve) => {
        accept = resolve;
      }),
  );
  auth.currentUser.mockReturnValue(
    Object.assign(Promise.resolve({ id: "tyler", name: "Tyler" }), { refresh }),
  );
  const { stream } = await renderRunningPage();
  stream.fail();
  await expect.poll(() => refresh.mock.calls.length).toBe(1);
  expect(TestEventSource.instances).toHaveLength(1);
  accept();
  await expect.poll(() => TestEventSource.instances.length).toBe(2);
  expect(auth.goto).not.toHaveBeenCalled();
});

test("a refused session probe returns to login and stops stream retries", async () => {
  auth.page.data.authEnabled = true;
  let expired: unknown;
  try {
    httpError(401, "Sign in to continue.");
  } catch (error) {
    expired = error;
  }
  const refresh = vi.fn().mockRejectedValue(expired);
  auth.currentUser.mockReturnValue({ refresh });
  const { stream } = await renderRunningPage();
  stream.fail();
  await expect.poll(() => auth.goto.mock.calls.length).toBe(1);
  expect(auth.goto).toHaveBeenCalledWith("/auth/login");
  await new Promise((resolve) => setTimeout(resolve, 400));
  expect(TestEventSource.instances).toHaveLength(1);
});

test("a finished session probe cannot restart a stream after page teardown", async () => {
  auth.page.data.authEnabled = true;
  let accept!: () => void;
  const refresh = vi.fn(
    () =>
      new Promise<void>((resolve) => {
        accept = resolve;
      }),
  );
  auth.currentUser.mockReturnValue(
    Object.assign(Promise.resolve({ id: "tyler", name: "Tyler" }), { refresh }),
  );
  const { screen, stream } = await renderRunningPage();
  stream.fail();
  await expect.poll(() => refresh.mock.calls.length).toBe(1);
  await screen.unmount();
  accept();
  await new Promise((resolve) => setTimeout(resolve, 400));
  expect(TestEventSource.instances).toHaveLength(1);
});

test("retained run panels close observation while inactive and preserve their selected view", async () => {
  const snapshot = structuredClone(planTripFixture.snapshot);
  const props = {
    project: "owner-project",
    data: { snapshot, graph: JSON.stringify(planTripFixture.graph) },
  };
  const screen = await render(RunView, { ...props, active: false });
  await screen.getByRole("button", { name: "Map", exact: true }).first().click();
  expect(TestEventSource.instances).toHaveLength(0);
  expect(auth.page.params.project).toBe("test-project");
  const event = new KeyboardEvent("keydown", { key: "/", bubbles: true, cancelable: true });
  window.dispatchEvent(event);
  expect(event.defaultPrevented).toBe(false);
  await screen.rerender({ ...props, active: true });
  await expect.poll(() => TestEventSource.instances.length).toBe(1);
  const stream = TestEventSource.instances[0];
  expect(stream.url).toContain(`/projects/owner-project/api/runs/${snapshot.run.id}/events`);
  await screen.rerender({ ...props, active: false });
  expect(stream.closed).toBe(true);
  await screen.rerender({ ...props, active: true });
  await expect.poll(() => TestEventSource.instances.length).toBe(2);
  expect(TestEventSource.instances[1].closed).toBe(false);
  await expect.element(screen.getByRole("region", { name: /workflow map/ })).toBeVisible();
  await screen.unmount();
  expect(TestEventSource.instances[1].closed).toBe(true);
});

test("switching between retained run panels observes the selected run identity", async () => {
  const first = structuredClone(planTripFixture.snapshot);
  const second = structuredClone(implementInterviewFixture.snapshot);
  const firstProps = {
    project: "retained-project",
    data: { snapshot: first, graph: JSON.stringify(planTripFixture.graph) },
  };
  const secondProps = {
    project: "retained-project",
    data: { snapshot: second, graph: JSON.stringify(implementInterviewFixture.graph) },
  };
  const one = await render(RunView, { ...firstProps, active: false });
  const two = await render(RunView, { ...secondProps, active: true });
  await expect.poll(() => TestEventSource.instances.length).toBe(1);
  expect(TestEventSource.instances[0].url).toContain(encodeURIComponent(second.run.id));
  await two.rerender({ ...secondProps, active: false });
  await one.rerender({ ...firstProps, active: true });
  await expect.poll(() => TestEventSource.instances.length).toBe(2);
  expect(TestEventSource.instances[0].closed).toBe(true);
  expect(TestEventSource.instances[1].url).toContain(encodeURIComponent(first.run.id));
  await one.unmount();
  expect(TestEventSource.instances[1].closed).toBe(true);
  await two.unmount();
});

test("a retained run panel renders a raw route preload without proxying its observation", async () => {
  const data = {
    snapshot: structuredClone(planTripFixture.snapshot),
    graph: JSON.stringify(planTripFixture.graph),
  };
  const screen = await render(RunPanel, {
    project: "retained-project",
    id: data.snapshot.run.id,
    initial: data,
    active: true,
  });
  await screen.getByRole("button", { name: "Map", exact: true }).first().click();
  await expect.element(mapRegion(screen)).toBeVisible();
  await screen.rerender({
    project: "retained-project",
    id: data.snapshot.run.id,
    initial: {
      snapshot: structuredClone(implementInterviewFixture.snapshot),
      graph: JSON.stringify(implementInterviewFixture.graph),
    },
    active: false,
  });
  await screen.rerender({
    project: "retained-project",
    id: data.snapshot.run.id,
    initial: data,
    active: true,
  });
  await expect.element(mapRegion(screen)).toBeVisible();
});

test("an unseeded retained run panel renders the query result", async () => {
  const data = {
    snapshot: structuredClone(planTripFixture.snapshot),
    graph: JSON.stringify(planTripFixture.graph),
  };
  auth.viewRun.mockReturnValue({ current: data, error: undefined });
  const screen = await render(RunPanel, {
    project: "retained-project",
    id: data.snapshot.run.id,
    active: true,
  });
  await screen.getByRole("button", { name: "Map", exact: true }).first().click();
  await expect.element(mapRegion(screen)).toBeVisible();
});

test("an open session hides the guarded run controls the map keeps", async () => {
  const snapshot = multiAgentSnapshot();
  const { screen } = await renderRunningPage({ graph: planTripFixture.graph, snapshot });
  await screen.getByRole("button", { name: "Cancel run", exact: true }).click();
  await expect.element(screen.getByRole("alertdialog")).toBeVisible();
  await screen.getByRole("button", { name: "Keep running", exact: true }).click();
  (await stepButton("preferences")).click();
  openSelectedSession();
  await expect.element(screen.getByRole("region", { name: "Agent chat" })).toBeVisible();
  await expect
    .element(screen.getByRole("button", { name: "Cancel run", exact: true }))
    .not.toBeInTheDocument();
  await expect
    .element(screen.getByRole("button", { name: "Stop turn", exact: true }))
    .not.toBeInTheDocument();
});

test("an open session leaves unconfirmed run cleanup controls in the run view", async () => {
  const snapshot = multiAgentSnapshot();
  snapshot.run.status = "cancelled";
  snapshot.run.cancellation = {
    by: "person",
    reason: "Cancelled",
    cleanup: "unconfirmed",
    cleanup_error: "No response",
  };
  for (const turn of Object.values(snapshot.turns)) turn.ended = turn.started + 1;
  const screen = await render(Page, {
    data: { snapshot, graph: JSON.stringify(planTripFixture.graph) },
  });
  await screen.getByRole("button", { name: "Map", exact: true }).first().click();
  await expect
    .element(screen.getByRole("button", { name: "Retry cleanup", exact: true }))
    .toBeVisible();
  (await stepButton("preferences")).click();
  openSelectedSession();
  await expect.element(screen.getByRole("region", { name: "Agent chat" })).toBeVisible();
  await expect
    .element(screen.getByRole("button", { name: "Retry cleanup", exact: true }))
    .not.toBeInTheDocument();
});

test("navigation closes a retained run confirmation without changing its view", async () => {
  const props = {
    project: "retained-project",
    data: { snapshot: multiAgentSnapshot(), graph: JSON.stringify(planTripFixture.graph) },
  };
  const screen = await render(RunView, { ...props, active: true });
  await screen.getByRole("button", { name: "Map", exact: true }).first().click();
  await screen.getByRole("button", { name: "Cancel run", exact: true }).click();
  await expect.element(screen.getByRole("alertdialog")).toBeVisible();
  await screen.rerender({ ...props, active: false });
  auth.after?.();
  await expect.element(screen.getByRole("alertdialog")).not.toBeInTheDocument();
  await screen.rerender({ ...props, active: true });
  await expect.element(screen.getByRole("alertdialog")).not.toBeInTheDocument();
  await expect.element(mapRegion(screen)).toBeVisible();
});
