import { beforeEach, expect, test, vi } from "vite-plus/test";
import { render } from "vitest-browser-svelte";
import FileViewer from "./FileViewer.svelte";
const remote = vi.hoisted(() => ({
  refresh: vi.fn(),
  view: {
    file: {
      id: "file-1",
      name: "reader.md",
      mediaType: "text/markdown",
      size: 8000,
      bucket: "fixture",
      key: "reader.md",
    },
    html: "<h1>A complete document</h1><p>A retained reading position belongs to the file.</p>",
    text: "",
    url: "/original-file",
  },
}));
vi.mock("../workspace.remote", () => ({
  viewFile: () => ({ current: remote.view, refresh: remote.refresh }),
}));
beforeEach(() => {
  remote.refresh.mockReset();
  remote.refresh.mockResolvedValue(undefined);
});
test("discussion passes library identity and keeps secondary actions in the menu", async () => {
  const ondiscuss = vi.fn(),
    onmove = vi.fn();
  const screen = await render(FileViewer, { project: "project", id: "file-1", ondiscuss, onmove });
  await screen.getByRole("link", { name: "Discuss in chat" }).click();
  expect(ondiscuss).toHaveBeenCalledWith(remote.view.file);
  await expect
    .element(screen.getByRole("link", { name: "Open / download", exact: true }))
    .not.toBeInTheDocument();
  await screen.getByRole("button", { name: "File actions" }).click();
  await expect
    .element(screen.getByRole("link", { name: "Open / download", exact: true }))
    .toHaveAttribute("href", "/original-file");
  await screen.getByRole("button", { name: "Move tab to new window" }).click();
  expect(onmove).toHaveBeenCalledOnce();
});
test("a failed refresh leaves the document readable and permits retry", async () => {
  remote.refresh.mockRejectedValueOnce(new Error("File store unavailable"));
  const screen = await render(FileViewer, { project: "project", id: "file-1" });
  await screen.getByRole("button", { name: "File actions" }).click();
  await screen.getByRole("button", { name: "Refresh file", exact: true }).click();
  await expect
    .element(screen.getByRole("alert"))
    .toHaveTextContent("Could not refresh this file: File store unavailable Retry");
  await expect.element(screen.getByRole("heading", { name: "A complete document" })).toBeVisible();
  await screen.getByRole("button", { name: "File actions" }).click();
  await screen.getByRole("button", { name: "Retry", exact: true }).click();
  await expect.element(screen.getByRole("alert")).not.toBeInTheDocument();
  expect(remote.refresh).toHaveBeenCalledTimes(2);
});
test("leaving a retained file removes its open menu without an animation frame", async () => {
  const screen = await render(FileViewer, { project: "project", id: "file-1", active: true });
  await screen.getByRole("button", { name: "File actions" }).click();
  await expect
    .element(screen.getByRole("link", { name: "Open / download", exact: true }))
    .toBeVisible();
  const noFrames = vi.spyOn(window, "requestAnimationFrame").mockImplementation(() => 0);
  try {
    await screen.rerender({ project: "project", id: "file-1", active: false });
    await expect
      .element(screen.getByRole("link", { name: "Open / download", exact: true }))
      .not.toBeInTheDocument();
    await screen.rerender({ project: "project", id: "file-1", active: true });
    await expect
      .element(screen.getByRole("link", { name: "Open / download", exact: true }))
      .not.toBeInTheDocument();
  } finally {
    noFrames.mockRestore();
  }
});
