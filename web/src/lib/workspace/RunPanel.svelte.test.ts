import { afterEach, beforeEach, expect, test, vi } from "vite-plus/test";
import { page, userEvent } from "vitest/browser";
import { render } from "vitest-browser-svelte";
import { planTripFixture } from "#lib/run/fixtures/index.js";
import RetainedRunTabs from "./RetainedRunTabs.svelte";

const navigation = vi.hoisted(() => ({ callbacks: [] as (() => void)[] }));
vi.mock("$app/navigation", () => ({
  goto: vi.fn(async () => {}),
  afterNavigate: (callback: () => void) => {
    navigation.callbacks.push(callback);
  },
}));
const remote = vi.hoisted(() => ({
  cancelRun: vi.fn(async (_: { run: string }) => ({ accepted: true })),
  stopTurn: vi.fn(async () => ({ accepted: true })),
}));
vi.mock("$app/state", () => ({ page: { params: {}, data: { authEnabled: false } } }));
vi.mock("../../routes/auth.remote", () => ({
  currentUser: vi.fn(),
  logout: vi.fn(),
  beginLogin: vi.fn(),
}));
vi.mock("../../routes/control.remote.js", () => remote);
vi.mock("#lib/runs.remote", () => ({ viewRun: () => ({ current: undefined, error: undefined }) }));

class SilentEventSource {
  static instances: SilentEventSource[] = [];
  closed = false;
  constructor(readonly url: string) {
    SilentEventSource.instances.push(this);
  }
  addEventListener() {}
  close() {
    this.closed = true;
  }
}
const originalEventSource = globalThis.EventSource;

beforeEach(() => {
  navigation.callbacks = [];
  remote.cancelRun.mockClear();
  SilentEventSource.instances = [];
  globalThis.EventSource = SilentEventSource as unknown as typeof EventSource;
  // WebKit delivers no animation frames to a desktop window it considers hidden,
  // such as one behind another window. The guard must not wait for one to close.
  vi.spyOn(window, "requestAnimationFrame").mockImplementation(() => 0);
});

afterEach(() => {
  globalThis.EventSource = originalEventSource;
  vi.restoreAllMocks();
});

const guard = () => page.getByRole("alertdialog");
const overlay = () => document.querySelector("[data-slot=alert-dialog-overlay]");

async function openCancelGuard(panelID = "workspace-panel-run") {
  const panel = document.getElementById(panelID);
  const menu = panel?.querySelector<HTMLDetailsElement>("details.more");
  if (!panel || panel.hidden || !menu) throw new Error(`active run panel ${panelID} is missing`);
  if (!menu.open) {
    const summary = menu.querySelector("summary");
    if (!summary) throw new Error(`run panel ${panelID} has no actions menu`);
    summary.click();
  }
  const action = menu.querySelector<HTMLButtonElement>(".menu button");
  if (action?.textContent?.trim() !== "Cancel run…") {
    throw new Error(`run panel ${panelID} has no Cancel run… action`);
  }
  action.click();
  await expect.element(guard()).toBeVisible();
}

async function openRunTabs() {
  const run = {
    snapshot: structuredClone(planTripFixture.snapshot),
    graph: JSON.stringify(planTripFixture.graph),
  };
  const screen = await render(RetainedRunTabs, { project: "retained-project", run });
  return { screen, run, openGuard: () => openCancelGuard() };
}

test("switching away from a run removes its cancellation guard and leaves the destination usable", async () => {
  const { screen, run, openGuard } = await openRunTabs();
  await openGuard();
  await screen.rerender({ project: "retained-project", run, selected: "draft" });
  navigation.callbacks.forEach((callback) => callback());
  await expect.element(guard()).not.toBeInTheDocument();
  expect(overlay()).toBeNull();
  const draft = screen.getByRole("textbox", { name: "Draft" });
  await draft.click();
  await userEvent.keyboard("still typing");
  await expect.element(draft).toHaveValue("still typing");

  await screen.rerender({ project: "retained-project", run, selected: "run" });
  await openCancelGuard();
  await page.getByRole("button", { name: "Keep running", exact: true }).click();
  await expect.element(guard()).not.toBeInTheDocument();
  expect(remote.cancelRun).not.toHaveBeenCalled();
});

test("Keep running and Escape dismiss the guard without cancelling", async () => {
  const { openGuard } = await openRunTabs();
  await openGuard();
  await page.getByRole("button", { name: "Keep running" }).click();
  await expect.element(guard()).not.toBeInTheDocument();
  expect(overlay()).toBeNull();
  await openGuard();
  await userEvent.keyboard("{Escape}");
  await expect.element(guard()).not.toBeInTheDocument();
  expect(overlay()).toBeNull();
  expect(remote.cancelRun).not.toHaveBeenCalled();
});

test("confirming cancels exactly this run and removes the guard", async () => {
  const { run, openGuard } = await openRunTabs();
  await openGuard();
  await guard().getByRole("button", { name: "Cancel run", exact: true }).click();
  await expect.element(guard()).not.toBeInTheDocument();
  expect(overlay()).toBeNull();
  expect(remote.cancelRun.mock.calls).toEqual([[{ run: run.snapshot.run.id }]]);
});

test("closing the run's tab removes its guard without cancelling the run", async () => {
  const { screen, run, openGuard } = await openRunTabs();
  await openGuard();
  await screen.rerender({ project: "retained-project", run, selected: "draft", shown: ["draft"] });
  await expect.element(guard()).not.toBeInTheDocument();
  expect(overlay()).toBeNull();
  await expect.element(screen.getByRole("textbox", { name: "Draft" })).toBeVisible();
  expect(remote.cancelRun).not.toHaveBeenCalled();
});

test("eight retained live runs use one observation stream and cancellation still targets the active run", async () => {
  const runs = Object.fromEntries(
    Array.from({ length: 8 }, (_, index) => {
      const snapshot = structuredClone(planTripFixture.snapshot);
      snapshot.run.id = `retained-${index}`;
      return [`run-${index}`, { snapshot, graph: JSON.stringify(planTripFixture.graph) }];
    }),
  );
  const ids = Object.keys(runs);
  const props = { project: "retained-project", run: runs[ids[0]], runs, shown: [...ids, "draft"] };
  const screen = await render(RetainedRunTabs, { ...props, selected: ids[0] });
  await expect
    .poll(() => SilentEventSource.instances.filter((item) => !item.closed).length)
    .toBe(1);
  for (const id of ids.slice(1)) {
    await screen.rerender({ ...props, selected: id });
    await expect
      .poll(() => SilentEventSource.instances.filter((item) => !item.closed).length)
      .toBe(1);
    expect(SilentEventSource.instances.at(-1)?.url).toContain(runs[id].snapshot.run.id);
  }
  await openCancelGuard(`workspace-panel-${ids.at(-1)!}`);
  await guard().getByRole("button", { name: "Cancel run", exact: true }).click();
  expect(remote.cancelRun.mock.calls).toEqual([[{ run: runs[ids.at(-1)!].snapshot.run.id }]]);
  await screen.rerender({ ...props, selected: "draft" });
  await expect
    .poll(() => SilentEventSource.instances.filter((item) => !item.closed).length)
    .toBe(0);
  const draft = screen.getByRole("textbox", { name: "Draft" });
  await draft.fill("Keep this draft");
  await screen.rerender({ ...props, selected: ids[0] });
  await expect
    .poll(() => SilentEventSource.instances.filter((item) => !item.closed).length)
    .toBe(1);
  await screen.rerender({ ...props, selected: "draft" });
  await expect.element(draft).toHaveValue("Keep this draft");
});
