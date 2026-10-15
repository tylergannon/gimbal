import { expect, test, vi } from "vite-plus/test";
import { render } from "vitest-browser-svelte";
import type { View, Entry } from "#lib/skgo/promises/types";
import type { Header } from "#lib/skgo/transcript/types";
import Ledger from "./PromiseLedger.svelte";
const remote = vi.hoisted(() => ({
  view: { entries: [], errors: [], working: false, version: 1 } as View,
  save: vi.fn(async (_request: unknown) => true),
}));
vi.mock("../workspace.remote", () => ({
  watchPromises: () => ({ current: remote.view, connected: true, error: undefined }),
  setPromiseWithdrawn: remote.save,
}));
const recording: Header = {
  id: "recording",
  created: 1,
  updated: 1,
  sequence: 1,
  status: "ended",
  requirements: true,
  source: "microphone",
  durationMs: 100,
};
const entry = (id: number): Entry => ({
  id,
  title: `Promise ${id}`,
  details: "A visible promise",
  tags: ["reliability"],
  withdrawn: false,
  created: 1,
  updated: 1,
  evidence: [],
});
test("empty states distinguish an unmarked project from waiting on marked speech", async () => {
  remote.view = { entries: [], errors: [], working: false, version: 1 };
  const screen = await render(Ledger, { project: "project", recordings: [] });
  await expect
    .element(
      screen.getByText("Mark a recording as a requirements discussion; its promises appear here."),
    )
    .toBeVisible();
  await expect.element(screen.getByRole("group", { name: "Sort" })).not.toBeInTheDocument();
  await screen.rerender({ project: "project", recordings: [recording] });
  await expect
    .element(
      screen.getByText(
        "Promises from requirements discussions appear here 10–30 seconds after each pause in speech.",
      ),
    )
    .toBeVisible();
  await expect
    .element(screen.getByText(/Recording ▸ Record Requirements Discussion/))
    .not.toBeInTheDocument();
});
test("no quote, transcript count and per-recording failure preserve withdraw controls", async () => {
  remote.view = {
    entries: [
      entry(1),
      {
        ...entry(2),
        evidence: [{ recording: "recording", chunk: 0, quote: "Always preserve progress." }],
      },
    ],
    errors: [{ recording: "recording", message: "named server failure" }],
    working: true,
    version: 2,
  };
  remote.save.mockRejectedValueOnce(new Error("Disk is full"));
  const screen = await render(Ledger, { project: "project", recordings: [recording] });
  await expect.element(screen.getByText("Reading new speech for promises…")).toBeVisible();
  await expect.element(screen.getByText(/Nothing from it was applied/)).toBeVisible();
  await expect.element(screen.getByText("named server failure")).toBeVisible();
  await screen.getByText("Promise 1", { exact: true }).click();
  await expect.element(screen.getByText("No quote found in the transcript.")).toBeVisible();
  await screen.getByRole("button", { name: "Withdraw", exact: true }).click();
  await expect.element(screen.getByText("Could not withdraw #1: Disk is full")).toBeVisible();
  await screen.getByText("Promise 2", { exact: true }).click();
  await expect.element(screen.getByText("From the transcript (1)", { exact: true })).toBeVisible();
  await expect
    .element(screen.getByRole("button", { name: "Withdraw", exact: true }).nth(1))
    .toBeEnabled();
});
