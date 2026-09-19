import { expect, test } from "vite-plus/test";
import { render } from "vitest-browser-svelte";
import { mismatchedHistoryFixture } from "./fixtures/index.js";
import HistoryLanes from "./HistoryLanes.svelte";

test("interrupted history records are ended while a genuine command failure remains failed", async () => {
  const snapshot = structuredClone(mismatchedHistoryFixture.snapshot);
  snapshot.turns["coding.1/turn.4"].interrupted = true;
  snapshot.turns["coding.1/turn.4"].error = "turn stopped";
  snapshot.commands["implementation.1/task.1/task-check.1"].interrupted = true;
  snapshot.commands["implementation.1/task.1/task-check.1"].error = "command stopped";

  await render(HistoryLanes, { snapshot, reason: mismatchedHistoryFixture.graphNotice });

  const lane = (name: string) => {
    const row = [...document.querySelectorAll("button.lane")].find((item) =>
      item.textContent?.includes(name),
    );
    expect(row, `history lane ${name} should be rendered`).not.toBeUndefined();
    if (!row) throw new Error(`history lane ${name} is missing`);
    return row;
  };
  expect(lane("coding").querySelector('[data-state="ended"]')).not.toBeNull();
  expect(lane("task-check").querySelector('[data-state="ended"]')).not.toBeNull();
  expect(lane("test").querySelector('[data-state="failed"]')).not.toBeNull();
});
