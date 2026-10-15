import { expect, test, vi } from "vite-plus/test";
import { render } from "vitest-browser-svelte";
import RecorderBridge from "./RecorderBridge.svelte";
import type { Recording } from "#lib/skgo/transcript/types";
import type { NativeStatus } from "./native";

const mocks = vi.hoisted(() => ({
  before: undefined as undefined | ((navigation: any) => void),
  after: undefined as undefined | (() => void),
  goto: vi.fn(async () => {}),
  watch: vi.fn(() => ({
    connected: true,
    current: undefined as Recording | undefined,
    error: undefined,
    done: false,
  })),
}));
vi.mock("$app/navigation", () => ({
  beforeNavigate: (callback: any) => {
    mocks.before = callback;
  },
  afterNavigate: (callback: () => void) => {
    mocks.after = callback;
  },
  goto: mocks.goto,
  refreshAll: vi.fn(async () => {}),
}));
vi.mock("$app/state", () => ({ page: { params: { project: "a" } } }));
vi.mock("../transcript.remote", () => ({ watchRecording: mocks.watch }));
vi.mock("../window.remote", () => ({ openProject: vi.fn() }));

function navigation(project: string, path = `/projects/${project}/recordings`) {
  return { to: { url: new URL(path, "http://localhost"), params: { project } }, cancel: vi.fn() };
}

test("cross-project links wait for native approval; same-project views remain usable", async () => {
  const messages: unknown[] = [];
  const postMessage = (value: unknown) => messages.push(value);
  window.webkit = {
    messageHandlers: {
      bridgeReady: { postMessage },
      requestProject: { postMessage },
      selectProject: { postMessage },
      displayAvailability: { postMessage },
      projectNavigationComplete: { postMessage },
    },
  };
  window.voiceNotesLatestStatus = {
    sessionID: "recording-a",
    sequence: 0,
    pending: false,
    state: "paused",
    source: "synthetic",
    project: "a",
  } satisfies NativeStatus;
  const screen = await render(RecorderBridge, { dock: false });
  const own = navigation("a", "/projects/a/promises");
  mocks.before!(own);
  expect(own.cancel).not.toHaveBeenCalled();
  const other = navigation("b");
  mocks.before!(other);
  expect(other.cancel).toHaveBeenCalledOnce();
  expect(messages).toContainEqual({ path: "/projects/b/recordings" });
  await screen.unmount();
  delete window.webkit;
  delete window.voiceNotesLatestStatus;
});

test("destination selection receipt is independent of the old transcript connection", async () => {
  const messages: unknown[] = [];
  const postMessage = (value: unknown) => messages.push(value);
  const complete = vi.fn(postMessage);
  window.webkit = {
    messageHandlers: {
      bridgeReady: { postMessage },
      selectProject: { postMessage },
      displayAvailability: { postMessage },
      projectNavigationComplete: { postMessage: complete },
    },
  };
  window.voiceNotesLatestStatus = {
    sessionID: "closed-old",
    sequence: 3,
    pending: false,
    state: "stopped",
    source: "synthetic",
    project: "old",
    switching: true,
  };
  const screen = await render(RecorderBridge, { dock: false });
  expect(messages).toContainEqual({ project: "a" });
  expect(complete).not.toHaveBeenCalled();
  window.dispatchEvent(
    new CustomEvent("gimbal-project-navigation", { detail: { path: "/projects/a/recordings" } }),
  );
  await vi.waitFor(() => expect(complete).toHaveBeenCalledWith({ project: "a" }));
  expect(messages).toContainEqual({
    available: false,
    error: "Transcript disconnected or waiting for saved text",
  });
  await screen.unmount();
  delete window.webkit;
  delete window.voiceNotesLatestStatus;
});

test("a finite saved-recording stream still acknowledges the final native display receipt", async () => {
  const messages: unknown[] = [];
  const postMessage = (value: unknown) => messages.push(value);
  mocks.watch.mockReturnValue({
    connected: false,
    done: true,
    error: undefined,
    current: {
      id: "saved",
      sequence: 4,
      status: "stopped",
      durationMs: null,
      source: "synthetic",
      created: 1,
      updated: 1,
      requirements: false,
      finalized: [],
      provisional: [],
      last: "",
    },
  });
  window.webkit = {
    messageHandlers: {
      bridgeReady: { postMessage },
      selectProject: { postMessage },
      displayAvailability: { postMessage },
      snapshotApplied: { postMessage },
    },
  };
  window.voiceNotesLatestStatus = {
    sessionID: "saved",
    sequence: 4,
    pending: false,
    state: "stopped",
    source: "synthetic",
    project: "a",
  };
  const screen = await render(RecorderBridge, { dock: true });
  await vi.waitFor(() => expect(messages).toContainEqual({ available: true, error: undefined }));
  await vi.waitFor(() =>
    expect(messages).toContainEqual({ sessionID: "saved", sequence: 4, closed: true }),
  );
  await expect.element(screen.getByText("Saved on server", { exact: true })).toBeVisible();
  await expect.element(screen.getByRole("button", { name: "Reconnect" })).not.toBeInTheDocument();
  messages.length = 0;
  window.voiceNotesNativeStatus?.({
    sessionID: "new-recording",
    sequence: 0,
    pending: false,
    state: "recording",
    source: "synthetic",
    project: "a",
  });
  await vi.waitFor(() =>
    expect(messages).toContainEqual({
      available: false,
      error: "Transcript disconnected or waiting for saved text",
    }),
  );
  await screen.unmount();
  delete window.webkit;
  delete window.voiceNotesLatestStatus;
});
