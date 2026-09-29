import { expect, test } from "vite-plus/test";
import { render } from "vitest-browser-svelte";
import CancellationStatus from "./CancellationStatus.svelte";

test("backend delivery stays distinct from local cleanup and retains failed attempts", async () => {
  const screen = await render(CancellationStatus, {
    cancellation: {
      by: "person",
      reason: "cancelled from the run workspace",
      cleanup: "pending",
      deliveries: [
        { at: 1, status: "unconfirmed", error: "context deadline exceeded" },
        { at: 2, status: "accepted" },
      ],
    },
  });
  await expect
    .element(screen.getByText("Backend cancellation accepted", { exact: true }))
    .toBeVisible();
  await expect.element(screen.getByText("Local cleanup pending", { exact: true })).toBeVisible();
  await screen.getByText("Backend cancellation accepted", { exact: true }).click();
  await expect
    .element(screen.getByText("Delivery 1: unconfirmed — context deadline exceeded"))
    .toBeVisible();
  await expect.element(screen.getByText("Delivery 2: accepted")).toBeVisible();
  await expect
    .element(screen.getByText("Backend acceptance confirms the request, not backend cleanup."))
    .toBeVisible();
});

test("unconfirmed backend delivery and failed local cleanup remain visible on a retained run", async () => {
  const screen = await render(CancellationStatus, {
    cancellation: {
      by: "person",
      reason: "cancelled",
      cleanup: "unconfirmed",
      cleanup_error: "session close failed",
      deliveries: [{ at: 1, status: "unconfirmed", error: "controller unavailable" }],
    },
  });
  await expect
    .element(screen.getByText("Backend cancellation unconfirmed", { exact: true }))
    .toBeVisible();
  await expect
    .element(screen.getByText("Local cleanup unconfirmed", { exact: true }))
    .toBeVisible();
  await screen.getByText("Backend cancellation unconfirmed", { exact: true }).click();
  await expect.element(screen.getByText("Local cleanup: session close failed")).toBeVisible();
});
