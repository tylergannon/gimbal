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
  renamed: [] as FormData[],
}));
vi.mock("../../routes/conversation.remote", () => ({
  watchConversation: remote.watch,
  projectConversations: () => ({ error: undefined, refresh: remote.refresh }),
  stopConversationRequest: remote.stop,
  cleanupConversation: remote.cleanup,
  renameConversation: {
    for: () => ({
      pending: 0,
      fields: {
        ...Object.fromEntries(
          ["project", "conversation", "title"].map((name) => [
            name,
            { as: (type: string, value?: string) => ({ name, type, value }) },
          ]),
        ),
        allIssues: () => [],
      },
      enhance: (handler: (submitted: unknown) => Promise<void>) => ({
        onsubmit: async (event: SubmitEvent) => {
          event.preventDefault();
          remote.renamed.push(new FormData(event.currentTarget as HTMLFormElement));
          await handler({ submit: () => ({ updates: async () => true }) });
        },
      }),
    }),
  },
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
  shared_checkout: false,
  base: "",
  branch: "",
  worktree: "/project",
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

test("Rename targets the active chat and preserves its identity", async () => {
  remote.renamed = [];
  const screen = await render(ConversationView, {
    project: "project",
    current: record("active"),
    active: true,
  });
  window.dispatchEvent(
    new CustomEvent("gimbal-workspace-command", { detail: { command: "renameTab" } }),
  );
  await screen.getByRole("textbox", { name: "Name", exact: true }).fill("A useful name");
  await screen.getByRole("button", { name: "Rename", exact: true }).click();
  expect(remote.renamed[0].get("conversation")).toBe("active");
  expect(remote.renamed[0].get("title")).toBe("A useful name");
  await expect.element(screen.getByRole("dialog")).not.toBeInTheDocument();
});
