import { expect, test, vi } from "vite-plus/test";
import { render } from "vitest-browser-svelte";
import type { Conversation } from "#lib/skgo/conversation/types";
import ConversationView from "./ConversationView.svelte";

const remote = vi.hoisted(() => ({
  watch: vi.fn(() => {
    throw new Error("Retained chats must use their shared workspace inventory");
  }),
  refresh: vi.fn(async () => {}),
  stop: vi.fn(async (_input: unknown) => ({})),
  cleanup: vi.fn(async () => true),
}));
vi.mock("../../routes/conversation.remote", () => ({
  watchConversation: remote.watch,
  conversationSetup: () => ({ current: undefined }),
  configureConversationModel: vi.fn(),
  renameConversation: {
    for: () => ({
      fields: {
        ...Object.fromEntries(
          ["project", "conversation", "title"].map((name) => [
            name,
            { as: (type: string, value?: unknown) => ({ name, type, value }) },
          ]),
        ),
        allIssues: () => [],
      },
      enhance: () => ({ onsubmit: (event: SubmitEvent) => event.preventDefault() }),
    }),
  },
  projectConversations: () => ({
    error: undefined,
    refresh: remote.refresh,
    catch: async () => {},
  }),
  stopConversationRequest: remote.stop,
  cleanupConversation: remote.cleanup,
  sendConversationMessage: {
    for: () => ({
      pending: 0,
      fields: {
        ...Object.fromEntries(
          ["conversation", "id", "created", "message"].map((name) => [
            name,
            {
              as: (type: string, value?: unknown) => ({
                name,
                type,
                ...(value === undefined ? {} : { value }),
              }),
              value: () => "",
            },
          ]),
        ),
        allIssues: () => [],
      },
      enhance: () => ({ onsubmit: (event: SubmitEvent) => event.preventDefault() }),
    }),
  },
}));

const record = (id: string): Conversation => ({
  id,
  title: `Chat ${id}`,
  provider: "claude",
  model: "haiku",
  effort: "",
  shared_checkout: true,
  base: "",
  branch: "",
  worktree: "/project",
  workflow_run: "workflow-run",
  native_session: "",
  live: true,
  status: "idle",
  error: "",
  created: 1,
  updated: 1,
  messages: [],
  runs: [],
  requests: [],
  active_request: "",
  state: "",
  revision: 1,
  environment: "local",
  placement: null,
  contact: "connected",
  capabilities: { steer: true, fork: true, worktree: true, browse: true, clone: true },
  cleanup_pending: false,
});
test("eight retained chats add no live connections and Stop refreshes the shared owner inventory", async () => {
  remote.watch.mockClear();
  remote.refresh.mockClear();
  remote.stop.mockClear();
  const screens = [];
  for (let i = 0; i < 8; i++)
    screens.push(
      await render(ConversationView, {
        project: "project",
        current: record(String(i)),
        active: false,
      }),
    );
  expect(remote.watch).not.toHaveBeenCalled();
  const current = { ...record("7"), active_request: "owned-request", status: "working" };
  await screens[7].rerender({ project: "project", current, active: true });
  await screens[7].getByRole("button", { name: "Stop response", exact: true }).click();
  expect(remote.stop).toHaveBeenCalledWith(
    expect.objectContaining({ conversation: "7", target: "owned-request" }),
  );
  expect(remote.refresh).toHaveBeenCalledOnce();
  expect(remote.watch).not.toHaveBeenCalled();
  for (const screen of screens) await screen.unmount();
});

test("leaving a chat destroys Rename without losing its message draft", async () => {
  const current = record("rename");
  const screen = await render(ConversationView, { project: "project", current, active: true });
  await screen.getByRole("textbox", { name: "Message Chat rename" }).fill("Unsent followup");
  window.dispatchEvent(
    new CustomEvent("gimbal-workspace-command", { detail: { command: "renameTab" } }),
  );
  await expect.element(screen.getByRole("dialog")).toBeVisible();
  await screen.rerender({ project: "project", current, active: false });
  await expect.element(screen.getByRole("dialog")).not.toBeInTheDocument();
  await screen.rerender({ project: "project", current, active: true });
  await expect.element(screen.getByRole("dialog")).not.toBeInTheDocument();
  await expect
    .element(screen.getByRole("textbox", { name: "Message Chat rename" }))
    .toHaveValue("Unsent followup");
});

// getByRole is strict: a second alert with the same error fails this lookup.
test("a failed run shows its error once", async () => {
  const error = "gimbal: agentic-dialogues.1: codex: model is not supported";
  const current = {
    ...record("failed"),
    live: false,
    status: "error",
    error,
    requests: [
      {
        id: "request",
        created: 1,
        kind: "prompt",
        message: "hello",
        status: "failed",
        error,
        delivery: "durable",
      },
    ],
  } as Conversation;
  const screen = await render(ConversationView, { project: "project", current, active: true });
  await expect.element(screen.getByRole("alert")).toHaveTextContent(error);
});
