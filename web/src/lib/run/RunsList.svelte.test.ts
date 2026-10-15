import { expect, test, vi } from "vite-plus/test";
import { render } from "vitest-browser-svelte";
import "../../app.css";
import RunsList from "./RunsList.svelte";
import { runsListFixture } from "./fixtures/index.js";
const remote = vi.hoisted(() => ({
  cancel: vi.fn(async (_request: unknown) => ({ accepted: true })),
}));
vi.mock("../../routes/control.remote", () => ({ cancelRun: remote.cancel }));

test("run rows expose real summaries and small cost and open the selected run", async () => {
  const opened: string[] = [];
  const items = structuredClone(runsListFixture.items);
  items[0].cost = "<$0.01";
  const screen = await render(RunsList, {
    items,
    attention: runsListFixture.attention,
    onopenrun: (run) => opened.push(run.id),
  });
  const row = screen.getByRole("button", { name: `Open ${items[0].run.name} ${items[0].run.id}` });
  await expect.element(row).toMatchTextContent("Needs you");
  await expect.element(row).toMatchTextContent("<$0.01");
  await expect.element(row).toMatchTextContent("3 sessions");
  await row.click();
  expect(opened).toEqual([items[0].run.id]);
  await expect.element(screen.getByRole("searchbox")).not.toBeInTheDocument();
});

test("four filters and revealed search preserve failed and cancelled history", async () => {
  const screen = await render(RunsList, {
    items: runsListFixture.items,
    attention: runsListFixture.attention,
  });
  await screen.getByRole("button", { name: "Ended", exact: true }).click();
  await expect
    .element(
      screen.getByRole("button", { name: "Open implement-interview 01M2QZ7PB3N6D0R9X2G5HKW8VY" }),
    )
    .toBeVisible();
  await expect
    .element(screen.getByRole("button", { name: "Open plan-trip 01M2RWXNFFYQA4HHQWMQ252CYF" }))
    .not.toBeInTheDocument();
  await screen.getByRole("button", { name: "All", exact: true }).click();
  await screen.getByRole("button", { name: "Search runs", exact: true }).click();
  await screen.getByRole("searchbox").fill("Q1HD4");
  await expect
    .element(screen.getByRole("button", { name: "Open plan-trip 01M2Q1HD4T8KJ2M7P5S0B9XNAE" }))
    .toBeVisible();
  await screen.getByRole("searchbox").fill("");
  await screen.getByRole("button", { name: "Needs you", exact: true }).click();
  await expect
    .element(screen.getByRole("button", { name: "Open plan-trip 01M2RWXNFFYQA4HHQWMQ252CYF" }))
    .toBeVisible();
});

test("row cancellation requires confirmation and leaves the run record authoritative", async () => {
  remote.cancel.mockClear();
  const screen = await render(RunsList, {
    items: runsListFixture.items,
    attention: runsListFixture.attention,
  });
  await screen
    .getByRole("button", { name: "Actions for plan-trip 01M2RWXNFFYQA4HHQWMQ252CYF" })
    .click();
  await screen.getByRole("button", { name: "Cancel run…" }).click();
  expect(remote.cancel).not.toHaveBeenCalled();
  await screen.getByRole("button", { name: "Cancel run", exact: true }).click();
  expect(remote.cancel).toHaveBeenCalledWith({ run: "01M2RWXNFFYQA4HHQWMQ252CYF" });
  await expect
    .element(screen.getByText("Cancellation accepted. Waiting for the run record to update."))
    .toBeVisible();
});

test("inactive Runs removes menus and cancellation dialogs without reviving them", async () => {
  const props = { items: runsListFixture.items, attention: runsListFixture.attention };
  const screen = await render(RunsList, { ...props, active: true });
  const menu = () =>
    screen.getByRole("button", { name: "Actions for plan-trip 01M2RWXNFFYQA4HHQWMQ252CYF" });
  await menu().click();
  await screen.rerender({ ...props, active: false });
  await expect.element(screen.getByRole("button", { name: "Cancel run…" })).not.toBeInTheDocument();
  await screen.rerender({ ...props, active: true });
  await menu().click();
  await screen.getByRole("button", { name: "Cancel run…" }).click();
  await expect.element(screen.getByRole("alertdialog")).toBeVisible();
  await screen.rerender({ ...props, active: false });
  await expect.element(screen.getByRole("alertdialog")).not.toBeInTheDocument();
  await screen.rerender({ ...props, active: true });
  await expect.element(screen.getByRole("alertdialog")).not.toBeInTheDocument();
});

test("run confirmation is opaque even when animation timelines cannot advance", async () => {
  const paused = document.createElement("style");
  paused.textContent =
    "[data-slot=popover-content], [data-slot=alert-dialog-content], [data-slot=alert-dialog-overlay] { animation-play-state: paused !important; }";
  document.head.append(paused);
  try {
    const screen = await render(RunsList, {
      items: runsListFixture.items,
      attention: runsListFixture.attention,
    });
    await screen
      .getByRole("button", { name: "Actions for plan-trip 01M2RWXNFFYQA4HHQWMQ252CYF" })
      .click();
    const menu = document.querySelector('[data-slot="popover-content"]')!;
    expect(getComputedStyle(menu).opacity).toBe("1");
    await screen.getByRole("button", { name: "Cancel run…" }).click();
    const dialog = screen.getByRole("alertdialog").element();
    expect(getComputedStyle(dialog).opacity).toBe("1");
    const overlay = document.querySelector('[data-slot="alert-dialog-overlay"]')!;
    expect(getComputedStyle(overlay).opacity).toBe("1");
  } finally {
    paused.remove();
  }
});
