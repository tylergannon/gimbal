import { expect, test } from "vite-plus/test";
import { render } from "vitest-browser-svelte";
import CancellationStatus from "./CancellationStatus.svelte";

test("failed local cleanup remain visible on a retained run", async () => {
  const screen = await render(CancellationStatus, {
    cancellation: {
      by: "person",
      reason: "cancelled",
      cleanup: "unconfirmed",
      cleanup_error: "session close failed",
    },
  });
  await expect
    .element(screen.getByText("Local cleanup unconfirmed", { exact: true }))
    .toBeVisible();
  await screen.getByText("Local cleanup unconfirmed", { exact: true }).click();
  await expect.element(screen.getByText("Local cleanup: session close failed")).toBeVisible();
});
