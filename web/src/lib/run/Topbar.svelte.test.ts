import { expect, test, vi } from "vite-plus/test";
import { render } from "vitest-browser-svelte";
import Topbar from "./Topbar.svelte";
import { planTripFixture } from "./fixtures/index.js";

vi.mock("$app/state", () => ({ page: { params: { project: "selected-project" } } }));

test("Runs breadcrumb targets the current project run list for live and recorded runs", async () => {
  const run = structuredClone(planTripFixture.snapshot.run);
  const screen = await render(Topbar, { run, connection: "live" });
  await expect
    .element(screen.getByRole("link", { name: "Runs", exact: true }))
    .toHaveAttribute("href", "/projects/selected-project/runs");
  await screen.rerender({
    run: { ...run, status: "completed" },
    connection: "recorded",
    project: "other-project",
    sessionName: "conversation",
  });
  await expect
    .element(screen.getByRole("link", { name: "Runs", exact: true }))
    .toHaveAttribute("href", "/projects/other-project/runs");
});
