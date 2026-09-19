import { expect, test } from "vite-plus/test";
import { render } from "vitest-browser-svelte";
import RunsLoading from "./RunsLoading.svelte";

test("announces Runs navigation and keeps card-shaped placeholders", async () => {
  const screen = await render(RunsLoading);

  await expect.element(screen.getByRole("heading", { name: "Runs" })).toBeVisible();
  await expect.element(screen.getByRole("status")).toHaveTextContent("Loading runs…");
  await expect
    .element(screen.getByRole("region", { name: "Runs" }))
    .toHaveAttribute("aria-busy", "true");
  expect(document.querySelectorAll("[data-loading-card]")).toHaveLength(2);
});
