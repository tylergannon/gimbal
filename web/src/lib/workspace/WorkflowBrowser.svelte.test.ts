import { expect, test, vi } from "vite-plus/test";
import { render } from "vitest-browser-svelte";
import Browser from "./WorkflowBrowser.svelte";

vi.mock("../../routes/workflow.remote", () => ({
  workflowCatalog: () => ({
    current: {
      workflows: [
        { name: "implement", summary: "Build outcomes" },
        { name: "review", summary: "Review a change" },
        { name: "validate-product", summary: "Test in parallel" },
      ],
    },
  }),
}));

test("every catalog entry opens its own workflow without a launch form", async () => {
  const onopen = vi.fn();
  const screen = await render(Browser, { project: "project", onopen });
  for (const name of ["implement", "review", "validate-product"]) {
    await screen.getByRole("button", { name: new RegExp(name) }).click();
    expect(onopen).toHaveBeenLastCalledWith(name);
  }
  await expect.element(screen.getByRole("button", { name: "Start run" })).not.toBeInTheDocument();
});
