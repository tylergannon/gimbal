import { beforeEach, expect, test, vi } from "vite-plus/test";
import { render } from "vitest-browser-svelte";
import { createSubscriber } from "svelte/reactivity";
import { createRawSnippet } from "svelte";
import type { Conversation } from "#lib/skgo/conversation/types";
import Workspace from "./ProjectWorkspace.svelte";
import type { RunsData } from "../runs.remote";

const state = vi.hoisted(() => ({
  page: {} as any,
  after: [] as (() => void)[],
  before: [] as (() => void)[],
  setRuns: (_current: RunsData | undefined, _error?: Error) => {},
  reconnect: vi.fn(),
  inventory: [] as Conversation[],
  navigate: undefined as undefined | ((path: string) => void),
  archive: vi.fn(),
  cleanup: vi.fn(async () => true),
  stop: vi.fn(async () => ({})),
  send: vi.fn(async (_input: FormData) => true),
}));
vi.mock("$app/state", async () => {
  const { createSubscriber } = await import("svelte/reactivity");
  let pathname = "/projects/project/conversations/pi";
  let update = () => {};
  const subscribe = createSubscriber((notify) => {
    update = notify;
  });
  state.navigate = (path) => {
    pathname = path;
    update();
  };
  state.page = {
    status: 200,
    params: { project: "project" },
    get url() {
      subscribe();
      return new URL(pathname, "http://localhost");
    },
    get data() {
      return { selected: state.inventory[0] };
    },
  };
  return { page: state.page };
});
vi.mock("$app/navigation", () => ({
  goto: async (path: string) => {
    state.before.forEach((fn) => fn());
    state.navigate?.(path);
    state.after.forEach((fn) => fn());
  },
  afterNavigate: (fn: () => void) => {
    state.after.push(fn);
  },
  beforeNavigate: (fn: () => void) => {
    state.before.push(fn);
  },
  refreshAll: vi.fn(),
}));

// A fresh form per mount deliberately models remote form eviction on teardown.
// Its input/restore/reset behavior still uses a real DOM form and FormData.
function form() {
  let message = "";
  let notify = () => {};
  let element: HTMLTextAreaElement | null = null;
  let formElement: HTMLFormElement | null = null;
  const subscribe = createSubscriber((update) => {
    notify = update;
  });
  const fields = Object.fromEntries(
    ["project", "conversation", "id", "created", "message", "path", "file"].map((name) => [
      name,
      {
        as: (type: string, value?: unknown) =>
          name === "message"
            ? {
                name,
                get value() {
                  subscribe();
                  return message;
                },
              }
            : { name, type, ...(value === undefined ? {} : { value }) },
        value: () => {
          subscribe();
          return message;
        },
        set: (value: string | number) => {
          if (name === "message") {
            message = String(value);
            if (element) element.value = message;
            notify();
          } else {
            const input = formElement?.elements.namedItem(name);
            if (input instanceof HTMLInputElement) input.value = String(value);
          }
        },
      },
    ]),
  );
  return {
    pending: 0,
    fields: { ...fields, allIssues: () => [] },
    enhance: (handler: (submitted: any) => Promise<void>) => ({
      oninput: (event: Event) => {
        element = (event.currentTarget as HTMLFormElement).querySelector("textarea");
        message = element?.value ?? "";
        notify();
      },
      onreset: () => {
        message = "";
        notify();
      },
      onsubmit: async (event: SubmitEvent) => {
        event.preventDefault();
        const target = event.currentTarget as HTMLFormElement;
        formElement = target;
        await handler({
          element: target,
          fields,
          submit: () => ({ updates: () => state.send(new FormData(target)) }),
        });
      },
    }),
  };
}
vi.mock("../../routes/conversation.remote", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../routes/conversation.remote")>()),
  projectConversations: () => ({
    current: state.inventory,
    refresh: async () => {},
    catch: async () => {},
  }),
  archiveConversation: state.archive,
  cleanupConversation: state.cleanup,
  stopConversationRequest: state.stop,
  sendConversationMessage: { for: () => form() },
  renameConversation: { for: () => form() },
  conversationSetup: () => ({ current: undefined }),
  configureConversationModel: vi.fn(),
}));
vi.mock("../workspace.remote", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../workspace.remote")>()),
  projectFiles: () => ({ current: [], catch: async () => {} }),
  watchPromises: () => ({ current: { entries: [] } }),
  uploadFile: { for: () => form() },
  createProject: { for: () => form() },
}));
vi.mock("../runs.remote", async (importOriginal) => {
  const { createSubscriber } = await import("svelte/reactivity");
  let current: RunsData | undefined;
  let error: Error | undefined;
  let notify = () => {};
  const subscribe = createSubscriber((update) => {
    notify = update;
  });
  state.setRuns = (next, failure) => {
    current = next;
    error = failure;
    notify();
  };
  const query = {
    get current() {
      subscribe();
      return current;
    },
    get error() {
      subscribe();
      return error;
    },
    reconnect: state.reconnect,
  };
  return { ...(await importOriginal<typeof import("../runs.remote")>()), watchRuns: () => query };
});
vi.mock("../../routes/workflow.remote", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../routes/workflow.remote")>()),
  workflowCatalog: () => ({ current: { workflows: [] } }),
}));
vi.mock("../transcript.remote", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../transcript.remote")>()),
  recordingHistory: () => ({
    current: { items: [], next: "" },
    refresh: async () => {},
    catch: async () => {},
  }),
}));

function conversation(id: string): Conversation {
  return {
    id,
    title: `Session ${id}`,
    provider: "pi",
    model: "flash",
    effort: "",
    shared_checkout: true,
    base: "",
    branch: "",
    worktree: "/project",
    workflow_run: "",
    native_session: "",
    live: true,
    status: "waiting",
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
  };
}
beforeEach(() => {
  state.after = [];
  state.before = [];
  state.setRuns({ items: [], attention: [], now: 1 });
  state.reconnect.mockClear();
  state.inventory = [conversation("pi"), conversation("other")];
  state.navigate?.("/projects/project/conversations/pi");
  state.archive.mockClear();
  state.stop.mockClear();
  state.cleanup.mockClear();
  state.send.mockReset().mockResolvedValue(true);
});
const props = {
  project: "project",
  name: "Project",
  projects: [{ id: "project", path: "/project" }],
  sidebarPreference: true,
  children: createRawSnippet(() => ({ render: () => "<div></div>" })),
};

for (const running of [false, true])
  test(`Close view preserves a ${running ? "running" : "ready"} session and its draft on sidebar reopen`, async () => {
    if (running) Object.assign(state.inventory[0], { active_request: "turn", status: "working" });
    const screen = await render(Workspace, props);
    const composer = () => screen.getByRole("textbox", { name: "Message Session pi" });
    await composer().fill("Unsent Pi follow-up");
    const sidebar = screen.getByRole("complementary", { name: "Project sidebar" });
    await sidebar.getByRole("button", { name: /Session other/ }).click();
    await sidebar.getByRole("button", { name: /Session pi/ }).click();
    await expect.element(composer()).toHaveValue("Unsent Pi follow-up");
    await screen.getByRole("button", { name: "Close Session pi", exact: true }).click();
    await expect.element(composer()).not.toBeInTheDocument();
    expect(state.archive).not.toHaveBeenCalled();
    expect(state.cleanup).not.toHaveBeenCalled();
    expect(state.stop).not.toHaveBeenCalled();
    await sidebar.getByRole("button", { name: /Session pi/ }).click();
    await expect.element(composer()).toBeEnabled();
    await expect.element(composer()).toHaveValue("Unsent Pi follow-up");
    if (running)
      await expect.element(screen.getByRole("button", { name: "Stop response" })).toBeEnabled();
    await composer().fill("Edited after reopen");
    await screen.getByRole("button", { name: "Send message" }).click();
    expect(state.send).toHaveBeenCalledOnce();
    expect(state.send.mock.calls[0][0].get("message")).toBe("Edited after reopen");
    await screen.getByRole("button", { name: "Close Session pi", exact: true }).click();
    await sidebar.getByRole("button", { name: /Session pi/ }).click();
    await expect.element(composer()).toHaveValue("");
  });

test("native Close Tab closes only the view, and ended history stays read-only on reopen", async () => {
  Object.assign(state.inventory[0], { live: false, status: "completed" });
  const screen = await render(Workspace, props);
  window.dispatchEvent(
    new CustomEvent("gimbal-workspace-command", { detail: { command: "closeTab" } }),
  );
  await expect.element(screen.getByRole("tab", { name: /Session pi/ })).not.toBeInTheDocument();
  await screen
    .getByRole("complementary", { name: "Project sidebar" })
    .getByRole("button", { name: /Session pi/ })
    .click();
  await expect.element(screen.getByRole("textbox", { name: "Message Session pi" })).toBeDisabled();
  await expect.element(screen.getByRole("button", { name: "Send message" })).toBeDisabled();
  await expect
    .element(screen.getByRole("link", { name: "Start a new session", exact: true }))
    .toBeVisible();
  expect(state.archive).not.toHaveBeenCalled();
  expect(state.cleanup).not.toHaveBeenCalled();
});

test("local End session requires confirmation and Stop turn remains separate from closing", async () => {
  Object.assign(state.inventory[0], { active_request: "turn", status: "working" });
  const screen = await render(Workspace, props);
  await screen.getByRole("button", { name: "Stop response", exact: true }).click();
  expect(state.stop).toHaveBeenCalledWith(
    expect.objectContaining({ conversation: "pi", target: "turn" }),
  );
  expect(state.cleanup).not.toHaveBeenCalled();
  await screen.getByLabelText("More session actions").click();
  await screen.getByRole("button", { name: "End session…", exact: true }).click();
  await expect.element(screen.getByRole("dialog")).toBeVisible();
  expect(state.cleanup).not.toHaveBeenCalled();
  await screen.getByRole("button", { name: "End session", exact: true }).click();
  expect(state.cleanup).toHaveBeenCalledWith({ project: "project", conversation: "pi" });
  await expect.element(screen.getByRole("dialog")).not.toBeInTheDocument();
});

test("rejected submission and closing a background tab preserve each session's own draft", async () => {
  const screen = await render(Workspace, props);
  const sidebar = screen.getByRole("complementary", { name: "Project sidebar" });
  await screen.getByRole("textbox", { name: "Message Session pi" }).fill("Pi draft");
  state.send.mockResolvedValueOnce(false);
  await screen.getByRole("button", { name: "Send message" }).click();
  await sidebar.getByRole("button", { name: /Session other/ }).click();
  await screen.getByRole("textbox", { name: "Message Session other" }).fill("Other draft");
  await screen.getByRole("button", { name: "Close Session pi", exact: true }).click();
  await expect
    .element(screen.getByRole("textbox", { name: "Message Session other" }))
    .toHaveValue("Other draft");
  await sidebar.getByRole("button", { name: /Session pi/ }).click();
  await expect
    .element(screen.getByRole("textbox", { name: "Message Session pi" }))
    .toHaveValue("Pi draft");
  expect(state.archive).not.toHaveBeenCalled();
});

const runInventory: RunsData = {
  items: [
    {
      run: {
        id: "inventory-run",
        name: "Inventory workflow",
        status: "running",
        error: "",
        recording_error: "",
        started: 100,
        ended: 0,
        workflow_environment: "local",
        task_environment: "local",
      },
      session_title: "",
      summary: "Actual inventory arrived",
      elapsed: "1m",
      activity_at: 200,
      cost: "$0.01",
      session_count: 1,
      turn_count: 1,
      instruction: "",
    },
  ],
  attention: [],
  now: 200,
};
for (const route of ["", "/runs"])
  test(`project ${route || "home"} loads without route inventory and updates from the live query`, async () => {
    state.inventory = [];
    state.setRuns(undefined);
    state.navigate?.(`/projects/project${route}`);
    const screen = await render(Workspace, props);
    const sidebar = screen.getByRole("complementary", { name: "Project sidebar" });
    const view = screen.getByRole("region", { name: route ? "Project runs" : "Project home" });
    await expect.element(sidebar.getByRole("status")).toHaveTextContent("Loading runs…");
    await expect.element(view.getByRole("status")).toHaveTextContent("Loading runs…");
    await expect.element(screen.getByText("Could not load this view.")).not.toBeInTheDocument();
    await expect
      .element(screen.getByText("No activity yet. Start a session or run a workflow."))
      .not.toBeInTheDocument();
    await expect.element(screen.getByText("No runs yet", { exact: true })).not.toBeInTheDocument();
    state.setRuns(runInventory);
    await expect.element(sidebar.getByRole("status")).not.toBeInTheDocument();
    await expect.element(sidebar.getByRole("button", { name: /All runs 1/ })).toBeVisible();
    await expect.element(sidebar.getByRole("button", { name: /Inventory workflow/ })).toBeVisible();
    await expect.element(view.getByText(/Inventory workflow/).first()).toBeVisible();
    if (!route) await sidebar.getByRole("button", { name: /All runs 1/ }).click();
    await expect
      .element(screen.getByRole("button", { name: "Open Inventory workflow inventory-run" }))
      .toBeVisible();
  });

for (const route of ["", "/runs"])
  test(`project ${route || "home"} exposes inventory failures and retries without reporting an empty project`, async () => {
    state.inventory = [];
    state.setRuns(undefined);
    state.navigate?.(`/projects/project${route}`);
    const screen = await render(Workspace, props);
    const sidebar = screen.getByRole("complementary", { name: "Project sidebar" });
    const view = screen.getByRole("region", { name: route ? "Project runs" : "Project home" });
    state.setRuns(undefined, new Error("Inventory API unavailable"));
    await expect
      .element(sidebar.getByRole("alert"))
      .toHaveTextContent("Could not read runs. Retry");
    await expect
      .element(view.getByRole("alert"))
      .toHaveTextContent(route ? "Could not read runs. Retry" : "Could not read run status. Retry");
    await expect.element(view.getByRole("status")).not.toBeInTheDocument();
    await expect.element(view.getByText("No runs yet", { exact: true })).not.toBeInTheDocument();
    await sidebar.getByRole("button", { name: "Retry" }).click();
    await view.getByRole("button", { name: "Retry" }).click();
    expect(state.reconnect).toHaveBeenCalledTimes(2);
    state.setRuns(runInventory);
    await expect.element(view.getByRole("alert")).not.toBeInTheDocument();
    state.setRuns(runInventory, new Error("Inventory connection lost"));
    await expect.element(view.getByRole("alert")).toBeVisible();
    await expect.element(sidebar.getByRole("alert")).toBeVisible();
  });
