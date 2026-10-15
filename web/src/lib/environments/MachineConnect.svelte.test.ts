import { beforeEach, expect, test, vi } from "vite-plus/test";
import { render } from "vitest-browser-svelte";
import MachineConnect from "./MachineConnect.svelte";
import type { ConnectionProgress } from "#lib/skgo/environments/types";

const remote = vi.hoisted(() => ({
  progress: undefined as ConnectionProgress | undefined,
  result: undefined as ConnectionProgress | undefined,
  submitted: [] as FormData[],
  fail: false,
}));
vi.mock("#lib/environments.remote", () => {
  const field = (name: string) => ({
    as: (type: string, value: string | number | boolean) => ({ name, type, value: String(value) }),
    value: () => undefined,
  });
  return {
    sshHosts: () => ({ current: ["build-box"] }),
    watchMachineConnection: () => ({
      current: remote.progress,
      error: undefined,
      reconnect: vi.fn(),
    }),
    saveMachine: {
      for: () => ({
        pending: 0,
        result: remote.result,
        fields: {
          ...Object.fromEntries(
            [
              "revision",
              "id",
              "name",
              "kind",
              "host",
              "port",
              "daemon_port",
              "workspace_root",
              "address",
              "executable",
              "connect_anyway",
            ].map((name) => [name, field(name)]),
          ),
          allIssues: () => [],
        },
        enhance: (handler: (submitted: unknown) => Promise<void>) => ({
          onsubmit: async (event: SubmitEvent) => {
            event.preventDefault();
            remote.submitted.push(
              new FormData(event.currentTarget as HTMLFormElement, event.submitter),
            );
            await handler({
              submit: async () => {
                if (remote.fail) throw new Error("SSH refused");
                return true;
              },
              result: remote.result,
            });
          },
        }),
      }),
    },
  };
});

const machine = {
  id: "build",
  name: "Build box",
  kind: "ssh",
  ssh: { host: "build-box", port: 0, daemon_port: 8081, workspace_root: "" },
};
const props = () => ({
  revision: "revision",
  machine,
  onclose: vi.fn(),
  onsaved: vi.fn(),
  onconfig: vi.fn(),
});
beforeEach(() => {
  remote.progress = undefined;
  remote.result = undefined;
  remote.submitted = [];
  remote.fail = false;
});

test("a failed recheck cannot reuse an earlier warning confirmation", async () => {
  remote.progress = {
    id: "build",
    complete: true,
    can_connect: true,
    saved: false,
    checks: [
      {
        id: "build",
        label: "Gimbal build",
        state: "warning",
        cause: "Builds differ",
        consequence: "Sessions are allowed.",
        fix: "Update Gimbal on Build box.",
        fatal: false,
      },
    ],
  };
  remote.result = remote.progress;
  const screen = await render(MachineConnect, props());
  await screen.getByRole("button", { name: "Connect", exact: true }).click();
  await expect
    .element(screen.getByRole("button", { name: "Connect anyway", exact: true }))
    .toBeVisible();
  remote.fail = true;
  await screen.getByRole("button", { name: "Check again", exact: true }).click();
  await expect.element(screen.getByRole("alert")).toHaveTextContent("SSH refused");
  await expect
    .element(screen.getByRole("button", { name: "Connect anyway", exact: true }))
    .not.toBeInTheDocument();
});

test("fatal SSH failure ends the visible checks and offers its fix without Connect anyway", async () => {
  remote.progress = {
    id: "build",
    complete: true,
    can_connect: false,
    saved: false,
    checks: [
      {
        id: "sign-in",
        label: "SSH sign-in",
        state: "failed",
        cause: "Permission denied",
        consequence: "Gimbal cannot reach this machine. Nothing was saved.",
        fix: "Open in Terminal to verify SSH access and host trust.",
        fatal: true,
      },
    ],
  };
  const screen = await render(MachineConnect, props());
  await screen.getByRole("button", { name: "Connect", exact: true }).click();
  await expect.element(screen.getByText("Permission denied", { exact: true })).toBeVisible();
  await expect.element(screen.getByText(/Nothing was saved/)).toBeVisible();
  await expect
    .element(screen.getByRole("button", { name: "Open in Terminal", exact: true }))
    .toBeEnabled();
  await expect
    .element(screen.getByRole("button", { name: "Connect anyway", exact: true }))
    .not.toBeInTheDocument();
  await expect.element(screen.getByRole("checkbox")).not.toBeInTheDocument();
});

test("nonfatal build warning requires an explicit Connect anyway form submission", async () => {
  remote.progress = {
    id: "build",
    complete: true,
    can_connect: true,
    saved: false,
    checks: [
      {
        id: "build",
        label: "Gimbal build",
        state: "warning",
        cause: "Builds differ",
        consequence: "Sessions are allowed. Native workflow plugins may fail to load.",
        fix: "Update Gimbal on Build box before running workflows there.",
        fatal: false,
      },
    ],
  };
  remote.result = remote.progress;
  const callbacks = props();
  const screen = await render(MachineConnect, callbacks);
  await screen.getByRole("button", { name: "Connect", exact: true }).click();
  await expect.element(screen.getByText(/Sessions are allowed/)).toBeVisible();
  await screen.getByRole("button", { name: "Connect anyway", exact: true }).click();
  expect(remote.submitted[0].get("connect_anyway")).toBe("false");
  expect(remote.submitted[1].get("connect_anyway")).toBe("true");
  expect(callbacks.onsaved).not.toHaveBeenCalled();
  await screen.getByRole("button", { name: "Back", exact: true }).click();
  await screen.getByRole("combobox", { name: "Host", exact: true }).fill("different-host");
  await expect
    .element(screen.getByRole("button", { name: "Connect anyway", exact: true }))
    .not.toBeInTheDocument();
  await expect.element(screen.getByRole("button", { name: "Connect", exact: true })).toBeEnabled();
});
