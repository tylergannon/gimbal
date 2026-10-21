import { expect, test, vi } from "vite-plus/test";
import { render } from "vitest-browser-svelte";
import type { Conversation } from "#lib/skgo/conversation/types";
import ConversationView from "./ConversationView.svelte";
import claudeCompleted from "./fixtures/claude-completed.json";
import claudeWaiting from "./fixtures/claude-waiting.json";
import claudeNarrated from "./fixtures/claude-narrated.json";
import codexTurns from "./fixtures/codex-turns.json";
import steeredTurns from "./fixtures/steered-turns.json";

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

// These render the conversation exactly as the Go manager produced it for a
// recorded run, so the reply has to be visible through the real page.
for (const [name, fixture] of [
  ["completed", claudeCompleted],
  ["waiting", claudeWaiting],
] as const)
  test(`a Claude ${name} StructuredOutput reply is visible as the chat reply`, async () => {
    const screen = await render(ConversationView, {
      project: "project",
      current: fixture as unknown as Conversation,
      active: true,
    });
    await expect.element(screen.getByText(/Hello there\s+friend/)).toBeVisible();
    await expect
      .element(screen.getByText(/StructuredOutput|"decision"|"state"/))
      .not.toBeInTheDocument();
  });

test("Claude narration and the StructuredOutput reply both show", async () => {
  const screen = await render(ConversationView, {
    project: "project",
    current: claudeNarrated as unknown as Conversation,
    active: true,
  });
  await expect.element(screen.getByText("I checked the files.", { exact: true })).toBeVisible();
  await expect.element(screen.getByText("The answer is 42.", { exact: true })).toBeVisible();
  await expect
    .element(screen.getByText(/StructuredOutput|"decision"|"state"/))
    .not.toBeInTheDocument();
});

test("Codex replies show once each, per turn, with no queued prompt", async () => {
  const screen = await render(ConversationView, {
    project: "project",
    current: codexTurns as unknown as Conversation,
    active: true,
  });
  await expect.element(screen.getByText("one", { exact: true })).toBeVisible();
  await expect.element(screen.getByText("two", { exact: true })).toBeVisible();
  await expect.element(screen.getByText(/"decision"/)).not.toBeInTheDocument();
  await expect.element(screen.getByText(/Queued/)).not.toBeInTheDocument();
  await expect.element(screen.getByRole("region", { name: "Turn 2", exact: true })).toBeVisible();
});

test("a steered prompt is not queued and the second turn is the running one", async () => {
  const screen = await render(ConversationView, {
    project: "project",
    current: steeredTurns as unknown as Conversation,
    active: true,
  });
  await expect.element(screen.getByText(/Queued/)).not.toBeInTheDocument();
  await expect.element(screen.getByText("turn 2", { exact: true })).toBeVisible();
  const first = screen.getByRole("region", { name: "Turn 1", exact: true });
  await expect.element(first.getByText("go on", { exact: true })).toBeVisible();
  await expect.element(first.getByText("On it", { exact: true })).toBeVisible();
  const second = screen.getByRole("region", { name: "Turn 2", exact: true });
  await expect.element(second.getByText("go on", { exact: true })).toBeVisible();
  expect(screen.getByText("go on", { exact: true }).elements()).toHaveLength(2);
});

const failedRecord = (error: string, requestError: string, delivery: string) =>
  ({
    ...record("failed"),
    live: false,
    status: "error",
    error,
    state: JSON.stringify({
      message: { failed: [{ id: "user", type: "user", requestID: "request", text: "hello" }] },
    }),
    requests: [
      {
        id: "request",
        created: 1,
        kind: "prompt",
        message: "hello",
        status: "failed",
        error: requestError,
        delivery,
      },
    ],
  }) as unknown as Conversation;
const visible = (text: string) => (document.body.textContent?.split(text).length ?? 1) - 1;

// The same failure is copied to the conversation and its request; either way it is read once.
for (const delivery of ["turn", "durable"]) {
  test(`a failed ${delivery} request shows its error once`, async () => {
    const error = "gimbal: agentic-dialogues.1: codex: model is not supported";
    const current = failedRecord(error, error, delivery);
    const screen = await render(ConversationView, { project: "project", current, active: true });
    await expect.element(screen.getByText(error, { exact: true })).toBeVisible();
    expect(visible(error)).toBe(1);
  });
}

test("distinct conversation and request errors are each shown", async () => {
  const current = failedRecord("run ended: first cause", "turn failed: second cause", "turn");
  const screen = await render(ConversationView, { project: "project", current, active: true });
  await expect.element(screen.getByText("turn failed: second cause")).toBeVisible();
  expect(visible("run ended: first cause")).toBe(1);
  expect(visible("turn failed: second cause")).toBe(1);
});
