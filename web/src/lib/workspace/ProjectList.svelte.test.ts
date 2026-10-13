import { beforeEach, expect, test, vi } from "vite-plus/test";
import { userEvent } from "vitest/browser";
import { render } from "vitest-browser-svelte";
import ProjectList from "./ProjectList.svelte";

const remote = vi.hoisted(() => ({ reorder: vi.fn(), invalidate: vi.fn() }));
vi.mock("$app/navigation", () => ({ invalidateAll: remote.invalidate, goto: vi.fn() }));
vi.mock("#lib/workspace.remote", () => ({ reorderProjects: remote.reorder }));

const projects = [
  { id: "a", path: "/src/alpha" },
  { id: "b", path: "/src/beta" },
  { id: "c", path: "/src/gamma" },
];
const href = (project: { id: string }) => `/projects/${project.id}`;
const names = (screen: Awaited<ReturnType<typeof render>>) =>
  screen
    .getByRole("listitem")
    .elements()
    .map((item) => item.querySelector(".font-medium")?.textContent);

beforeEach(() => {
  remote.reorder.mockReset().mockResolvedValue(true);
  remote.invalidate.mockReset().mockResolvedValue(undefined);
});

test("arrow keys on a handle move the project and save the new order", async () => {
  const screen = await render(ProjectList, { projects, href });
  (screen.getByRole("button", { name: "Move gamma" }).element() as HTMLElement).focus();
  await userEvent.keyboard("{ArrowUp}");
  await expect.poll(() => names(screen)).toEqual(["alpha", "gamma", "beta"]);
  expect(remote.reorder).toHaveBeenCalledWith({ ids: ["a", "c", "b"] });
  expect(remote.invalidate).toHaveBeenCalled();
});

test("a rejected save puts the projects back and says why", async () => {
  remote.reorder.mockRejectedValue(new Error("The project list changed. Reload and try again."));
  const screen = await render(ProjectList, { projects, href });
  (screen.getByRole("button", { name: "Move alpha" }).element() as HTMLElement).focus();
  await userEvent.keyboard("{ArrowDown}");
  await expect
    .element(screen.getByRole("alert"))
    .toHaveTextContent("The project list changed. Reload and try again.");
  expect(names(screen)).toEqual(["alpha", "beta", "gamma"]);
});
