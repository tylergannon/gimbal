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
  fail: false,
}));
vi.mock("#lib/environments.remote", () => ({
  environmentSnapshot: () => ({ current: remote.inventory, ready: false, refresh: async () => {} }),
}));
vi.mock("$app/navigation", () => ({ goto: vi.fn() }));
vi.mock("../../routes/conversation.remote", () => {
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
    projectConversations: () => ({ current: remote.recent }),
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
    .element(screen.getByRole("button", { name: /Codex Not installed on This Mac/ }))
    .toBeDisabled();
});

test("last session and Start like restore environment and checkout setup", async () => {
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
  await screen.rerender({
    project: "project",
    name: "Project",
    oncreated: vi.fn(),
    startLike: "earlier",
  });
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
  await screen.getByRole("button", { name: "Harness and model" }).click();
  await screen.getByRole("textbox", { name: "First message" }).fill("Use this effort");
  await screen.getByRole("button", { name: "Start session" }).click();
  expect(remote.submitted[0].get("effort")).toBe("high");
});
