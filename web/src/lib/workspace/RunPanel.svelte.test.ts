import { afterEach, beforeEach, expect, test, vi } from "vite-plus/test";
import { page, userEvent } from "vitest/browser";
import { render } from "vitest-browser-svelte";
import { planTripFixture } from "#lib/run/fixtures/index.js";
import RetainedRunTabs from "./RetainedRunTabs.svelte";

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
  addEventListener() {}
  close() {}
}
const originalEventSource = globalThis.EventSource;

beforeEach(() => {
  remote.cancelRun.mockClear();
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

async function openRunTabs() {
  const run = {
    snapshot: structuredClone(planTripFixture.snapshot),
    graph: JSON.stringify(planTripFixture.graph),
  };
  const screen = await render(RetainedRunTabs, { project: "retained-project", run });
  const openGuard = async () => {
    await screen.getByRole("button", { name: "Cancel run", exact: true }).click();
    await expect.element(guard()).toBeVisible();
  };
  return { screen, run, openGuard };
}

test("switching away from a run removes its cancellation guard and leaves the destination usable", async () => {
  const { screen, run, openGuard } = await openRunTabs();
  await openGuard();
  await screen.rerender({ project: "retained-project", run, selected: "draft" });
  await expect.element(guard()).not.toBeInTheDocument();
  expect(overlay()).toBeNull();
  const draft = screen.getByRole("textbox", { name: "Draft" });
  await draft.click();
  await userEvent.keyboard("still typing");
  await expect.element(draft).toHaveValue("still typing");

  await screen.rerender({ project: "retained-project", run, selected: "run" });
  await expect
    .element(screen.getByRole("button", { name: "Cancel run", exact: true }))
    .toBeVisible();
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
