import { expect, test, vi } from "vite-plus/test";
import { render } from "vitest-browser-svelte";
import MachineDetail from "./MachineDetail.svelte";
import type { MachineState, Snapshot } from "#lib/skgo/environments/types";

const remote = vi.hoisted(() => ({
  remove: vi.fn(async () => true),
  reconnect: vi.fn(async () => ({})),
  cleanup: vi.fn(async () => {
    throw new Error("Cleanup is not confirmed; the request may still be running.");
  }),
}));
vi.mock("#lib/environments.remote", () => ({
  removeMachine: remote.remove,
  reconnectMachine: remote.reconnect,
  watchMachineConnection: () => ({ current: undefined, error: undefined, reconnect: vi.fn() }),
}));
vi.mock("../../routes/conversation.remote", () => ({ cleanupConversation: remote.cleanup }));

test("removing a machine names owned sessions and retains configuration on uncertain cleanup", async () => {
  remote.remove.mockClear();
  remote.cleanup.mockClear();
  const capabilities = { steer: false, fork: false, worktree: true, browse: true, clone: true };
  const machine: MachineState = {
    id: "build",
    name: "Build box",
    kind: "ssh",
    detail: "",
    state: "connected",
    since: 1,
    latency_ms: 3,
    build: "test",
    harnesses: [
      { id: "codex", state: "missing", default: "", models: [], detail: "Not installed" },
      { id: "claude", state: "unknown", default: "", models: [], detail: "Sign-in unverified" },
    ],
    capabilities,
    issues: [],
  };
  const snapshot: Snapshot = {
    project: "",
    config_path: "/catalog/environments.json",
    revision: "revision",
    machines: [machine],
    environments: [],
    sessions: [
      {
        project: "project-a",
        conversation: "session-a",
        title: "Fix the parser",
        machine: "build",
        environment: "remote",
        directory: "/srv/project",
        status: "unresolved",
        active: true,
        pending: true,
      },
    ],
    issues: [],
  };
  const screen = await render(MachineDetail, {
    machine,
    snapshot,
    onchanged: vi.fn(),
    onedit: vi.fn(),
    onadd: vi.fn(),
    oneditenvironment: vi.fn(),
  });
  await expect
    .element(screen.getByText("Install codex on Build box in Terminal.", { exact: true }))
    .toBeVisible();
  await expect
    .element(screen.getByText("Verify claude sign-in on Build box in Terminal.", { exact: true }))
    .toBeVisible();
  await expect
    .element(screen.getByRole("button", { name: /^(Install|Verify sign-in) on/ }))
    .not.toBeInTheDocument();
  await expect
    .element(screen.getByRole("button", { name: "Open in Terminal", exact: true }))
    .toBeEnabled();
  await screen.getByRole("button", { name: "Remove…", exact: true }).click();
  await expect
    .element(screen.getByRole("button", { name: "Remove machine", exact: true }))
    .toBeDisabled();
  await screen.getByRole("checkbox", { name: /Fix the parser/ }).click();
  await screen.getByRole("button", { name: "End selected sessions" }).click();
  expect(remote.cleanup.mock.calls).toEqual([
    [{ project: "project-a", conversation: "session-a" }],
  ]);
  await expect
    .element(screen.getByRole("alert"))
    .toHaveTextContent("Cleanup is not confirmed; the request may still be running.");
  await expect
    .element(screen.getByRole("button", { name: "Remove machine", exact: true }))
    .toBeDisabled();
  expect(remote.remove).not.toHaveBeenCalled();
});

test("a different build names the workflow consequence without disabling session actions", async () => {
  const capabilities = { steer: true, fork: true, worktree: true, browse: true, clone: true };
  const machine: MachineState = {
    id: "build",
    name: "Build box",
    kind: "ssh",
    detail: "",
    state: "connected",
    since: 0,
    latency_ms: 4,
    build: "older-build",
    harnesses: [],
    capabilities,
    issues: [],
  };
  const snapshot: Snapshot = {
    project: "",
    config_path: "/catalog/environments.json",
    revision: "revision",
    machines: [{ ...machine, id: "local", name: "This Mac", build: "current-build" }, machine],
    environments: [],
    sessions: [],
    issues: [],
  };
  const screen = await render(MachineDetail, {
    machine,
    snapshot,
    onchanged: vi.fn(),
    onedit: vi.fn(),
    onadd: vi.fn(),
    oneditenvironment: vi.fn(),
  });
  await expect
    .element(screen.getByText(/Sessions are allowed; native workflow plugins may fail to load/))
    .toBeVisible();
  await expect.element(screen.getByText(/Update Gimbal on Build box/)).toBeVisible();
  await expect
    .element(screen.getByRole("button", { name: "Add environment…", exact: true }))
    .toBeEnabled();
  await expect
    .element(screen.getByRole("button", { name: "Reconnect", exact: true }))
    .toBeEnabled();
});
