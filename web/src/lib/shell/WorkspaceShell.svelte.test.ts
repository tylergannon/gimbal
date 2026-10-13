import { expect, test } from "vite-plus/test";
import { page, userEvent } from "vitest/browser";
import { render } from "vitest-browser-svelte";
import WorkspaceDemo from "./WorkspaceDemo.svelte";

test("switching mixed views preserves the chat draft and both reading positions", async () => {
  const screen = await render(WorkspaceDemo);
  const draft = screen.getByRole("textbox", { name: "Message for Workspace layout" });
  await draft.fill("Keep this draft while I read the file");
  const transcript = screen.getByLabelText("Workspace layout transcript").element() as HTMLElement;
  transcript.scrollTop = 240;
  await screen.getByRole("tab", { name: "web-app.md" }).click();
  const document = screen.getByLabelText("web-app.md document").element() as HTMLElement;
  document.scrollTop = 300;
  await screen.getByRole("tab", { name: "Workspace layout" }).click();
  await expect.element(draft).toHaveValue("Keep this draft while I read the file");
  expect(transcript.scrollTop).toBe(240);
  await screen.getByRole("tab", { name: "web-app.md" }).click();
  expect(document.scrollTop).toBe(300);
});

test("keyboard selection reveals overflowed tabs and Delete closes only the view", async () => {
  const screen = await render(WorkspaceDemo, { many: true });
  await screen.getByRole("tab", { name: "Workspace layout" }).click();
  await userEvent.keyboard("{End}");
  const last = screen.getByRole("tab", {
    name: "Workspace research and implementation notes 7.md",
  });
  await expect.element(last).toHaveAttribute("aria-selected", "true");
  await expect.element(last).toHaveFocus();
  // A long selected filename must not consume the file/chat distinction.
  const glyph = last.element().querySelector("svg")!.getBoundingClientRect();
  expect(glyph.width).toBe(14);
  expect(glyph.height).toBe(14);
  await userEvent.keyboard("{Home}");
  await userEvent.keyboard("{ArrowRight}");
  await expect
    .element(screen.getByRole("tab", { name: /Return navigation #455/ }))
    .toHaveAttribute("aria-selected", "true");
  await userEvent.keyboard("{Delete}");
  await expect
    .element(screen.getByRole("tab", { name: /Return navigation #455/ }))
    .not.toBeInTheDocument();
  await expect.element(screen.getByRole("status")).toHaveAccessibleName("recording: 1:12:08");
  await screen.getByRole("button", { name: "Show sidebar" }).click();
  const working = screen.getByRole("button", { name: "Return navigation #455 Working Codex" });
  await expect.element(working).toBeVisible();
  await working.click();
  await expect
    .element(screen.getByRole("tab", { name: /Return navigation #455/ }))
    .toHaveAttribute("aria-selected", "true");
});

test("the more disclosure selects a hidden view and dismisses with Escape", async () => {
  const screen = await render(WorkspaceDemo, { many: true });
  const more = screen.getByText(/^\d+ more$/);
  await more.click();
  const choice = screen.getByRole("button", {
    name: "Workspace research and implementation notes 7.md",
  });
  const label = choice.element().querySelector(".choice-title") as HTMLElement;
  // A clipped or ellipsized filename conceals its distinguishing ending.
  expect(label.scrollWidth).toBeLessThanOrEqual(label.clientWidth);
  expect(label.scrollHeight).toBeLessThanOrEqual(label.clientHeight);
  await choice.click();
  await expect
    .element(screen.getByRole("tab", { name: "Workspace research and implementation notes 7.md" }))
    .toHaveAttribute("aria-selected", "true");
  await more.click();
  await userEvent.keyboard("{Escape}");
  await expect.element(more).toHaveFocus();
});

test("sidebar toggling preserves selection and closing all views remains navigable", async () => {
  const screen = await render(WorkspaceDemo, { sidebar: true });
  await screen.getByRole("button", { name: "Hide sidebar" }).click();
  await expect
    .element(screen.getByRole("tab", { name: "Workspace layout" }))
    .toHaveAttribute("aria-selected", "true");
  for (const name of ["Workspace layout", "Return navigation #455", "web-app.md"]) {
    await screen.getByRole("button", { name: `Close ${name}`, exact: true }).click();
  }
  await expect.element(screen.getByText("Open a chat or file from the sidebar.")).toBeVisible();
  await screen.getByRole("button", { name: "Show sidebar" }).click();
  await screen.getByRole("button", { name: "web-app.md", exact: true }).click();
  await expect.element(screen.getByRole("tabpanel", { name: "web-app.md" })).toBeVisible();
});

test("below 1100 px the sidebar opens over the views instead of narrowing them", async () => {
  await page.viewport(1000, 600);
  try {
    const screen = await render(WorkspaceDemo, { sidebar: true });
    const views = screen.getByRole("region", { name: "Open views" }).element();
    await expect
      .element(screen.getByRole("complementary", { name: "Project sidebar" }))
      .toBeVisible();
    expect(views.getBoundingClientRect().width).toBe(1000);
    // Choosing a view or clicking past the overlay puts it away.
    await screen.getByRole("button", { name: "web-app.md", exact: true }).click();
    await expect.element(screen.getByRole("tabpanel", { name: "web-app.md" })).toBeVisible();
    await expect
      .element(screen.getByRole("complementary", { name: "Project sidebar" }))
      .not.toBeInTheDocument();
    await screen.getByRole("button", { name: "Show sidebar" }).click();
    await expect
      .element(screen.getByRole("complementary", { name: "Project sidebar" }))
      .toBeVisible();
    await userEvent.click(screen.getByRole("tabpanel", { name: "web-app.md" }));
    await expect
      .element(screen.getByRole("complementary", { name: "Project sidebar" }))
      .not.toBeInTheDocument();
  } finally {
    await page.viewport(1440, 900);
  }
});
