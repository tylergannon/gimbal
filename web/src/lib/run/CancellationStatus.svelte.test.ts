import { expect, test } from "vite-plus/test";
import { render } from "vitest-browser-svelte";
import CancellationStatus from "./CancellationStatus.svelte";

test("cancellation request remains visible on a recorded run", async () => {
  const screen = await render(CancellationStatus, {
    cancellation: {
      by: "person",
      reason: "cancelled",
    },
  });
  await expect.element(screen.getByText("Cancellation requested", { exact: true })).toBeVisible();
  await screen.getByText("Cancellation requested", { exact: true }).click();
  await expect.element(screen.getByText("Requested by person: cancelled")).toBeVisible();
});
