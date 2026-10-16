import { expect, test, vi } from "vite-plus/test";
import { render } from "vitest-browser-svelte";
import { userEvent } from "vitest/browser";
import TabStrip from "./TabStrip.svelte";
import type { WorkspaceTab } from "./types";

test.each(["failed", "needs-you"] as const)(
  "unseen is independent from a persistent %s pip",
  async (status) => {
    const label = status === "failed" ? "Failed" : "Needs you";
    const select = vi.fn();
    const tab: WorkspaceTab = { id: "run", title: "Review", kind: "run", status, unseen: true };
    const screen = await render(TabStrip, {
      tabs: [tab],
      active: "other",
      onselect: select,
      onclose: vi.fn(),
    });
    const button = screen.getByRole("tab", { name: `${label} Review Unseen changes` });
    await expect.element(button).toBeVisible();
    await button.click();
    expect(select).toHaveBeenCalledWith("run");
    await screen.rerender({
      tabs: [{ ...tab, unseen: false }],
      active: "run",
      onselect: select,
      onclose: vi.fn(),
    });
    await expect
      .element(screen.getByRole("tab", { name: `${label} Review` }))
      .toHaveAttribute("aria-selected", "true");
    expect(document.querySelector(`.pip[data-state="${status}"]`)).not.toBeNull();
    expect(document.querySelector(".unseen-mark")).toBeNull();
  },
);
test("group menu selects real members by keyboard and can unfold views", async () => {
  const select = vi.fn();
  const close = vi.fn();
  const tabs: WorkspaceTab[] = [
    { id: "parent", title: "Build checkout", kind: "chat" },
    ...(["running", "failed", "needs-you"] as const).map((status, index) => ({
      id: `run-${index}`,
      title: `Review ${index}`,
      kind: "run" as const,
      status,
      unseen: index === 1,
      workflow: "review",
      origin: { id: "parent", title: "Build checkout" },
    })),
  ];
  const screen = await render(TabStrip, {
    tabs,
    active: "parent",
    onselect: select,
    onclose: close,
  });
  const group = screen.getByRole("tab", {
    name: "Needs you runs · Build checkout Unseen changes 3",
  });
  expect(group.element().parentElement!.getBoundingClientRect().height).toBeGreaterThanOrEqual(30);
  await group.click();
  const menu = screen.getByRole("menu");
  expect(menu.element().textContent?.indexOf("Review 2")).toBeLessThan(
    menu.element().textContent?.indexOf("Review 1") ?? -1,
  );
  await screen.getByRole("menuitem", { name: /Review 2/ }).click();
  expect(select).toHaveBeenCalledWith("run-2");
  await group.click();
  await userEvent.keyboard("{ArrowDown}{Enter}");
  expect(select).toHaveBeenCalledWith("run-2");
  await group.click();
  await screen.getByRole("menuitem", { name: "Open each as its own tab" }).click();
  expect(document.querySelectorAll('[role="tab"]')).toHaveLength(4);
  expect(close).not.toHaveBeenCalled();
});
