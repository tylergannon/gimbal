import { expect, test } from "vite-plus/test";
import { render } from "vitest-browser-svelte";
import MessageRow from "./MessageRow.svelte";

test("paused recovery exposes an actionable connection message", async () => {
  const screen = await render(MessageRow, {
    message: {
      id: "connection.1",
      type: "connection",
      state: "paused",
      attempt: 3,
      message: "Recovery paused. Inspect partial work, then send resume or cancel the run.",
      time: { created: 1 },
    },
    revision: 1,
  });
  await expect.element(screen.getByText("paused", { exact: true })).toBeVisible();
  await expect
    .element(
      screen.getByText(
        "Recovery paused. Inspect partial work, then send resume or cancel the run.",
      ),
    )
    .toBeVisible();
});
