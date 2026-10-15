import { expect, test, vi } from "vite-plus/test";
import { render } from "vitest-browser-svelte";
import RecordingTranscript from "./RecordingTranscript.svelte";

const remote = vi.hoisted(() => ({
  refresh: vi.fn(async () => {}),
  watch: vi.fn(() => {
    throw new Error("Retained recordings must not open live connections");
  }),
  mark: vi.fn(async () => {}),
}));
vi.mock("../transcript.remote", () => ({
  watchRecording: remote.watch,
  markRequirements: remote.mark,
  recordingSnapshot: ({ id }: { id: string }) => ({
    ready: true,
    error: undefined,
    refresh: remote.refresh,
    current: {
      id,
      created: 1,
      status: "stopped",
      requirements: false,
      finalized: [{ startMs: 0, text: "Saved text" }],
      provisional: [],
    },
  }),
}));

test("eight retained recordings use finite reads and refresh on revisit and requirements changes", async () => {
  remote.refresh.mockClear();
  remote.watch.mockClear();
  remote.mark.mockClear();
  const screens = [];
  for (let i = 0; i < 8; i++)
    screens.push(
      await render(RecordingTranscript, { project: "project", id: `saved-${i}`, active: false }),
    );
  expect(remote.watch).not.toHaveBeenCalled();
  expect(remote.refresh).not.toHaveBeenCalled();
  await screens[7].rerender({ project: "project", id: "saved-7", active: true });
  await expect.poll(() => remote.refresh.mock.calls.length).toBe(1);
  const beforeMark = remote.refresh.mock.calls.length;
  await screens[7].getByRole("button", { name: "Treat as requirements discussion" }).nth(7).click();
  expect(remote.mark.mock.calls).toEqual([[{ project: "project", id: "saved-7" }]]);
  expect(remote.refresh.mock.calls.length).toBeGreaterThan(beforeMark);
  const beforeRevisit = remote.refresh.mock.calls.length;
  await screens[7].rerender({ project: "project", id: "saved-7", active: false });
  await screens[7].rerender({ project: "project", id: "saved-7", active: true });
  await expect.poll(() => remote.refresh.mock.calls.length).toBeGreaterThan(beforeRevisit);
  expect(remote.watch).not.toHaveBeenCalled();
  for (const screen of screens) await screen.unmount();
});
