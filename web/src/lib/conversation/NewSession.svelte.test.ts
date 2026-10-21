import { beforeEach, expect, test, vi } from "vite-plus/test";
import { render } from "vitest-browser-svelte";
import NewSession from "./NewSession.svelte";
import type { Conversation } from "#lib/skgo/conversation/types";

const remote = vi.hoisted(() => ({
  setup: {
    harnesses: [
      {
        id: "claude",
        default: "claude-haiku-4-5",
        models: ["claude-haiku-4-5"],
        efforts: [] as { model: string; levels: string[] }[],
      },
    ],
    directory: "~/project",
    git: true,
    unborn: false,
    branch: "working",
    base: "main",
    bases: ["main", "release"],
    environment: "local",
    machine: "This Mac",
    capabilities: { steer: true, fork: true, worktree: true, browse: true, clone: true },
    harness_states: [] as { id: string; state: string }[],
  },
  inventory: {
    project: "project",
    config_path: "/catalog/environments.json",
    revision: "1",
    issues: [],
    sessions: [],
    machines: [
      {
        id: "local",
        name: "This Mac",
        kind: "local",
        state: "connected",
        since: 0,
        detail: "",
        issues: [],
      },
      {
        id: "remote",
        name: "Build box",
        kind: "ssh",
        state: "connected",
        since: 0,
        detail: "",
        issues: [],
      },
      {
        id: "offline",
        name: "Offline box",
        kind: "address",
        state: "unreachable",
        since: 1,
        detail: "No response",
        issues: [],
      },
    ],
    environments: [
      {
        environment: { id: "local", project: "project", machine: "local", directory: "~/project" },
        directory: { git: true, branch: "working" },
        capabilities: { steer: true },
        last_used: 0,
        observed: 1,
        live_sessions: 0,
        issues: [],
      },
      {
        environment: {
          id: "build",
          project: "project",
          machine: "remote",
          directory: "/srv/project",
        },
        directory: { git: true, branch: "main" },
        capabilities: { steer: false },
        last_used: 1,
        observed: 1,
        live_sessions: 2,
        issues: [],
      },
      {
        environment: {
          id: "offline-env",
          project: "project",
          machine: "offline",
          directory: "/offline/project",
        },
        directory: { git: true, branch: "main" },
        capabilities: { steer: false },
        last_used: 0,
        observed: 1,
        live_sessions: 0,
        issues: [],
      },
    ],
  },
  submitted: [] as FormData[],
  recent: [] as Conversation[],
  notifyRecent: () => {},
  defaultsLoaded: undefined as Promise<void> | undefined,
  fail: false,
}));
vi.mock("#lib/environments.remote", () => ({
  environmentSnapshot: () => ({ current: remote.inventory, ready: false, refresh: async () => {} }),
}));
vi.mock("$app/navigation", () => ({ goto: vi.fn() }));
vi.mock("../../routes/conversation.remote", async () => {
  const { createSubscriber } = await import("svelte/reactivity");
  let notify = () => {};
  const subscribe = createSubscriber((update) => {
    notify = update;
    return () => {
      notify = () => {};
    };
  });
  remote.notifyRecent = () => notify();
  const field = (name: string) => ({
    as: (type: string, value?: string | boolean) => ({
      name,
      type,
      ...(value === undefined ? {} : { value: String(value) }),
    }),
    value: () => "",
  });
  const fields = Object.fromEntries(
    [
      "provider",
      "model",
      "effort",
      "file_id",
      "message",
      "environment",
      "shared_checkout",
      "branch",
      "base",
    ].map((name) => [name, field(name)]),
  );
  return {
    conversationSetup: ({ environment }: { environment: string }) => ({
      current: {
        ...remote.setup,
        environment,
        machine: environment === "build" ? "Build box" : "This Mac",
        directory: environment === "build" ? "/srv/project" : remote.setup.directory,
        environments: remote.inventory,
      },
    }),
    projectConversations: () => ({
      get current() {
        subscribe();
        return remote.recent;
      },
      ready: true,
      then: (resolve: (value: Conversation[]) => unknown, reject: (error: unknown) => unknown) =>
        (remote.defaultsLoaded ?? Promise.resolve()).then(() => resolve(remote.recent), reject),
    }),
    createConversation: {
      for: () => ({
        pending: 0,
        fields: { ...fields, allIssues: () => [] },
        enhance: (handler: (submitted: unknown) => Promise<void>) => ({
          onsubmit: async (event: SubmitEvent) => {
            event.preventDefault();
            remote.submitted.push(new FormData(event.currentTarget as HTMLFormElement));
            await handler({
              submit: () => ({
                updates: async () => {
                  if (remote.fail) throw new Error("Branch already exists; choose a new branch.");
                  return false;
                },
              }),
            });
          },
        }),
      }),
    },
  };
});

beforeEach(() => {
  remote.setup.git = true;
  remote.setup.unborn = false;
  remote.submitted = [];
  remote.fail = false;
  remote.recent = [];
  remote.defaultsLoaded = undefined;
  remote.setup.harness_states = [];
  remote.setup.harnesses = [
    { id: "claude", default: "claude-haiku-4-5", models: ["claude-haiku-4-5"], efforts: [] },
  ];
});

test("checkout choices survive popover dismissal and a rejected submission", async () => {
  remote.fail = true;
  const screen = await render(NewSession, {
    project: "project",
    name: "Project",
    oncreated: vi.fn(),
  });
  await screen.getByRole("textbox", { name: "First message" }).fill("Keep my draft");
  await screen.getByRole("button", { name: "Worktree", exact: true }).click();
  await screen.getByRole("textbox", { name: "New branch" }).fill("feature/edited");
  await screen.getByRole("combobox", { name: "Base branch" }).selectOptions("release");
  await screen.getByRole("button", { name: "Worktree", exact: true }).click();
  await screen.getByRole("button", { name: "Start session" }).click();
  await expect
    .element(screen.getByRole("alert"))
    .toHaveTextContent("Branch already exists; choose a new branch.");
  expect(remote.submitted[0].get("branch")).toBe("feature/edited");
  expect(remote.submitted[0].get("base")).toBe("release");
  expect(remote.submitted[0].get("shared_checkout")).toBe("false");
  await expect
    .element(screen.getByRole("textbox", { name: "First message" }))
    .toHaveValue("Keep my draft");
  await screen.getByRole("button", { name: "Worktree", exact: true }).click();
  await expect
    .element(screen.getByRole("textbox", { name: "New branch" }))
    .toHaveValue("feature/edited");
  await expect
    .element(screen.getByRole("combobox", { name: "Base branch" }))
    .toHaveValue("release");
  await screen.getByRole("switch", { name: "Create a worktree" }).click();
  await expect
    .element(
      screen.getByText("You and other sessions there can change the same files.", { exact: false }),
    )
    .toBeVisible();
  await screen.getByRole("button", { name: "Worktree", exact: true }).click();
  await screen.getByRole("button", { name: "Start session" }).click();
  expect(remote.submitted[1].get("shared_checkout")).toBe("true");
  expect(remote.submitted[1].get("branch")).toBe("");
  await screen.getByRole("button", { name: "Worktree", exact: true }).click();
  await screen.getByRole("switch", { name: "Create a worktree" }).click();
  await expect
    .element(screen.getByRole("textbox", { name: "New branch" }))
    .toHaveValue("feature/edited");
});

test("unborn repository explains why worktrees are unavailable", async () => {
  remote.setup.git = false;
  remote.setup.unborn = true;
  const screen = await render(NewSession, {
    project: "project",
    name: "Project",
    oncreated: vi.fn(),
  });
  await screen.getByRole("button", { name: "Worktree", exact: true }).click();
  await expect.element(screen.getByRole("switch", { name: "Create a worktree" })).toBeDisabled();
  await expect.element(screen.getByText("has no Git commit yet", { exact: false })).toBeVisible();
});

test("environment selection preserves each checkout draft and does not submit a session", async () => {
  const screen = await render(NewSession, {
    project: "project",
    name: "Project",
    oncreated: vi.fn(),
  });
  await screen.getByRole("textbox", { name: "First message" }).fill("Keep the same message");
  await screen.getByRole("button", { name: "Worktree", exact: true }).click();
  await screen.getByRole("textbox", { name: "New branch" }).fill("feature/local");
  await screen.getByRole("combobox", { name: "Base branch" }).selectOptions("release");
  await screen.getByRole("button", { name: "Worktree", exact: true }).click();
  await screen.getByRole("button", { name: "Environment", exact: true }).click();
  await expect.element(screen.getByRole("button", { name: /\/offline\/project/ })).toBeDisabled();
  await screen.getByRole("button", { name: /\/srv\/project/ }).click();
  await screen.getByRole("button", { name: "Worktree", exact: true }).click();
  await screen.getByRole("textbox", { name: "New branch" }).fill("feature/remote");
  await screen.getByRole("switch", { name: "Create a worktree" }).click();
  await screen.getByRole("button", { name: "Worktree", exact: true }).click();
  await screen.getByRole("button", { name: "Environment", exact: true }).click();
  await screen.getByRole("button", { name: /~\/project/ }).click();
  await screen.getByRole("button", { name: "Worktree", exact: true }).click();
  await expect
    .element(screen.getByRole("textbox", { name: "New branch" }))
    .toHaveValue("feature/local");
  await expect
    .element(screen.getByRole("combobox", { name: "Base branch" }))
    .toHaveValue("release");
  await screen.getByRole("button", { name: "Worktree", exact: true }).click();
  await screen.getByRole("button", { name: "Environment", exact: true }).click();
  await screen.getByRole("button", { name: /\/srv\/project/ }).click();
  await screen.getByRole("button", { name: "Worktree", exact: true }).click();
  await expect.element(screen.getByRole("switch", { name: "Create a worktree" })).not.toBeChecked();
  await expect
    .element(screen.getByRole("textbox", { name: "First message" }))
    .toHaveValue("Keep the same message");
  expect(remote.submitted).toEqual([]);
});

test("Discuss adds removable file context without replacing an existing draft", async () => {
  const oncreated = vi.fn();
  const oncontextremoved = vi.fn();
  const screen = await render(NewSession, {
    project: "project",
    name: "Project",
    oncreated,
    oncontextremoved,
  });
  await screen.getByRole("textbox", { name: "First message" }).fill("Keep this question");
  await screen.rerender({
    project: "project",
    name: "Project",
    oncreated,
    oncontextremoved,
    fileContext: { id: "file-id", name: "Brief.pdf" },
  });
  await expect
    .element(screen.getByRole("textbox", { name: "First message" }))
    .toHaveValue("Keep this question");
  await screen.getByRole("button", { name: "Start session" }).click();
  expect(remote.submitted[0].get("file_id")).toBe("file-id");
  expect(remote.submitted[0].get("message")).toBe("Keep this question");
  await screen.getByRole("button", { name: "Remove file context" }).click();
  expect(oncontextremoved).toHaveBeenCalledOnce();
});

test("unavailable harnesses stay visible with their machine-specific reason", async () => {
  remote.setup.harnesses.push({
    id: "codex",
    default: "gpt-6-luna",
    models: ["gpt-6-luna"],
    efforts: [{ model: "gpt-6-luna", levels: ["low", "medium", "high"] }],
  });
  remote.setup.harness_states = [{ id: "codex", state: "missing" }];
  const screen = await render(NewSession, {
    project: "project",
    name: "Project",
    oncreated: vi.fn(),
  });
  await screen.getByRole("button", { name: "Harness and model" }).click();
  await expect
    .element(screen.getByRole("button", { name: /Codex Not installed · This Mac/ }))
    .toBeDisabled();
});

test("last session restores setup and the environment picker changes it", async () => {
  remote.recent = [
    {
      id: "last",
      title: "Prior setup",
      provider: "claude",
      model: "claude-haiku-4-5",
      effort: "",
      environment: "build",
      shared_checkout: true,
      base: "release",
    } as Conversation,
  ];
  const screen = await render(NewSession, {
    project: "project",
    name: "Project",
    oncreated: vi.fn(),
  });
  await expect
    .element(screen.getByRole("button", { name: "Environment" }))
    .toHaveTextContent("Build box · /srv/project");
  await expect
    .element(screen.getByRole("button", { name: "Worktree", exact: true }))
    .toHaveTextContent("Shared checkout");
  remote.recent.push({
    id: "earlier",
    title: "Local setup",
    provider: "claude",
    model: "claude-haiku-4-5",
    effort: "",
    environment: "local",
    shared_checkout: false,
    base: "main",
  } as Conversation);
  await screen.getByRole("button", { name: "Environment", exact: true }).click();
  await screen.getByRole("button", { name: /~\/project.*Git/ }).click();
  await expect
    .element(screen.getByRole("button", { name: "Environment" }))
    .toHaveTextContent("This Mac · ~/project");
  await expect
    .element(screen.getByRole("button", { name: "Worktree", exact: true }))
    .toHaveTextContent("New worktree");
});

test("explicit effort stays with the submitted session setup", async () => {
  remote.setup.harnesses = [
    {
      id: "codex",
      default: "gpt-6-luna",
      models: ["gpt-6-luna"],
      efforts: [{ model: "gpt-6-luna", levels: ["low", "medium", "high"] }],
    },
  ];
  const screen = await render(NewSession, {
    project: "project",
    name: "Project",
    oncreated: vi.fn(),
  });
  await screen.getByRole("button", { name: "Harness and model" }).click();
  await screen.getByRole("combobox", { name: "Effort" }).selectOptions("high");
  // Re-selecting the current model closes the picker without resetting explicit effort.
  await screen.getByRole("button", { name: /Luna 6.*gpt-6-luna/ }).click();
  await screen.getByRole("textbox", { name: "First message" }).fill("Use this effort");
  await screen.getByRole("button", { name: "Start session" }).click();
  expect(remote.submitted[0].get("effort")).toBe("high");
});

test.each(["Harness and model", "Environment", "Worktree"])(
  "deactivating New session unmounts the open %s picker without an animation frame",
  async (picker) => {
    const oncreated = vi.fn();
    const props = { project: "project", name: "Project", oncreated, active: true };
    const screen = await render(NewSession, props);
    await screen
      .getByRole("textbox", { name: "First message" })
      .fill("Keep the question while another tab is open");
    await screen.getByRole("button", { name: "Worktree", exact: true }).click();
    await screen.getByRole("textbox", { name: "New branch" }).fill("feature/retained-draft");
    await screen.getByRole("combobox", { name: "Base branch" }).selectOptions("release");
    await screen.getByRole("button", { name: "Worktree", exact: true }).click();
    await screen.getByRole("button", { name: picker, exact: true }).click();
    expect(document.querySelectorAll("[data-popover-content][data-state='open']")).toHaveLength(1);

    // WKWebView may stop frame callbacks for an inactive view. Removal must
    // happen on component teardown, not wait for a close animation frame.
    const frames = vi.fn(() => 0);
    vi.stubGlobal("requestAnimationFrame", frames);
    try {
      await screen.rerender({ ...props, active: false });
      expect(document.querySelectorAll("[data-popover-content]")).toHaveLength(0);
    } finally {
      vi.unstubAllGlobals();
    }

    await screen.rerender(props);
    expect(document.querySelectorAll("[data-popover-content]")).toHaveLength(0);
    await expect
      .element(screen.getByRole("textbox", { name: "First message" }))
      .toHaveValue("Keep the question while another tab is open");
    await screen.getByRole("button", { name: "Worktree", exact: true }).click();
    await expect
      .element(screen.getByRole("textbox", { name: "New branch" }))
      .toHaveValue("feature/retained-draft");
    await expect
      .element(screen.getByRole("combobox", { name: "Base branch" }))
      .toHaveValue("release");
    expect(remote.submitted).toEqual([]);
  },
);

test("opening a file unmounts setup popups while retaining the draft and choices", async () => {
  const oncreated = vi.fn();
  const screen = await render(NewSession, {
    project: "project",
    name: "Project",
    oncreated,
    active: true,
  });
  await screen.getByRole("textbox", { name: "First message" }).fill("Read the attached file");
  await screen.getByRole("button", { name: "Worktree", exact: true }).click();
  await screen.getByRole("textbox", { name: "New branch" }).fill("codex/retained-file-draft");
  const noFrames = vi.spyOn(window, "requestAnimationFrame").mockImplementation(() => 0);
  try {
    await screen.rerender({ project: "project", name: "Project", oncreated, active: false });
    await expect
      .element(screen.getByRole("textbox", { name: "New branch" }))
      .not.toBeInTheDocument();
    await screen.rerender({
      project: "project",
      name: "Project",
      oncreated,
      active: true,
      fileContext: { id: "file", name: "notes.md" },
    });
    await expect
      .element(screen.getByRole("textbox", { name: "New branch" }))
      .not.toBeInTheDocument();
    await expect
      .element(screen.getByRole("textbox", { name: "First message" }))
      .toHaveValue("Read the attached file");
  } finally {
    noFrames.mockRestore();
  }
  await screen.getByRole("button", { name: "Worktree", exact: true }).click();
  await expect
    .element(screen.getByRole("textbox", { name: "New branch" }))
    .toHaveValue("codex/retained-file-draft");
});

test("a background reply cannot replace the setup owned by an existing draft", async () => {
  remote.recent = [
    {
      id: "last",
      title: "Last local",
      provider: "claude",
      model: "claude-haiku-4-5",
      effort: "",
      environment: "local",
      shared_checkout: true,
      base: "main",
    } as Conversation,
  ];
  const screen = await render(NewSession, {
    project: "project",
    name: "Project",
    oncreated: vi.fn(),
  });
  await screen.getByRole("textbox", { name: "First message" }).fill("Keep my draft");
  remote.recent = [
    {
      id: "background",
      title: "Background",
      provider: "codex",
      model: "gpt-6-luna",
      effort: "high",
      environment: "build",
      shared_checkout: false,
      base: "release",
    } as Conversation,
    ...remote.recent,
  ];
  remote.notifyRecent();
  await expect
    .element(screen.getByRole("button", { name: "Environment", exact: true }))
    .toHaveTextContent("This Mac · ~/project");
  await expect
    .element(screen.getByRole("button", { name: "Harness and model" }))
    .toHaveTextContent("C Claude Code · Haiku 4.5");
  await expect
    .element(screen.getByRole("button", { name: "Worktree", exact: true }))
    .toHaveTextContent("Shared checkout");
  await screen.getByRole("button", { name: "Start session" }).click();
  expect(remote.submitted[0].get("environment")).toBe("local");
  expect(remote.submitted[0].get("message")).toBe("Keep my draft");
});

test("a saved setup stays recoverable when the selected machine reports no harnesses", async () => {
  remote.setup.harnesses = [];
  remote.recent = [
    {
      id: "saved",
      title: "Saved setup",
      provider: "claude",
      model: "claude-haiku-4-5",
      effort: "",
      environment: "local",
      shared_checkout: true,
      base: "main",
    } as Conversation,
  ];
  const screen = await render(NewSession, {
    project: "project",
    name: "Project",
    oncreated: vi.fn(),
  });
  await screen.getByRole("textbox", { name: "First message" }).fill("Keep my work");
  await screen.getByRole("button", { name: "Harness and model" }).click();
  await expect
    .element(screen.getByRole("textbox", { name: "First message" }))
    .toHaveValue("Keep my work");
  await expect
    .element(screen.getByRole("button", { name: "Environment", exact: true }))
    .toBeEnabled();
});

test("late defaults cannot overwrite checkout choices made while history loads", async () => {
  let release!: () => void;
  remote.defaultsLoaded = new Promise<void>((resolve) => {
    release = resolve;
  });
  remote.recent = [
    {
      id: "prior",
      title: "Prior remote",
      provider: "claude",
      model: "claude-haiku-4-5",
      environment: "build",
      shared_checkout: true,
      base: "main",
    } as Conversation,
  ];
  const screen = await render(NewSession, {
    project: "project",
    name: "Project",
    oncreated: vi.fn(),
  });
  await screen.getByRole("textbox", { name: "First message" }).fill("My chosen setup");
  await screen.getByRole("button", { name: "Worktree", exact: true }).click();
  await screen.getByRole("textbox", { name: "New branch" }).fill("feature/my-choice");
  await screen.getByRole("combobox", { name: "Base branch" }).selectOptions("release");
  release();
  await remote.defaultsLoaded;
  await screen.getByRole("button", { name: "Worktree", exact: true }).click();
  await expect
    .element(screen.getByRole("button", { name: "Environment", exact: true }))
    .toHaveTextContent("This Mac · ~/project");
  await screen.getByRole("button", { name: "Start session" }).click();
  expect(remote.submitted[0].get("branch")).toBe("feature/my-choice");
  expect(remote.submitted[0].get("base")).toBe("release");
  expect(remote.submitted[0].get("message")).toBe("My chosen setup");
});

test("setup pickers switch without waiting for an exit animation", async () => {
  const screen = await render(NewSession, {
    project: "project",
    name: "Project",
    oncreated: vi.fn(),
  });
  await screen.getByRole("button", { name: "Harness and model" }).click();
  await expect.element(screen.getByRole("group", { name: "Harness", exact: true })).toBeVisible();
  const noFrames = vi.spyOn(window, "requestAnimationFrame").mockImplementation(() => 0);
  try {
    await screen.getByRole("button", { name: "Worktree", exact: true }).click();
    await expect
      .element(screen.getByRole("group", { name: "Harness", exact: true }))
      .not.toBeInTheDocument();
    await expect.element(screen.getByRole("textbox", { name: "New branch" })).toBeInTheDocument();
  } finally {
    noFrames.mockRestore();
  }
});
