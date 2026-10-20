import { expect, test } from "vite-plus/test";
import { render } from "vitest-browser-svelte";
import type { Conversation } from "#lib/skgo/conversation/types";
import type { SessionTurn as Turn } from "./sessionTurns";
import SessionTurn from "./SessionTurn.svelte";
import codex from "./fixtures/codex-turns.json";

const request = { ...codex.requests[0], id: "request", message: "Check the build" };
const shell = (id: string, command: string, status = "completed", output = "passed\n") => ({
  id,
  type: "tool",
  name: "Bash",
  state: { status, input: { command }, content: [{ type: "text", text: output }] },
});
const turn = (status = "completed"): Turn => ({
  request: { ...request, status },
  ordinal: 1,
  messages: [
    {
      id: "request/prompt",
      type: "user",
      text: request.message,
      requestID: request.id,
      time: { created: 1000 },
    },
    {
      id: "request/narration",
      type: "assistant",
      content: [{ type: "text", text: "I’ll check the build and test results." }],
    },
    {
      id: "request/tools",
      type: "assistant",
      content: [shell("build", "go test ./..."), shell("status", "git status --short")],
    },
    {
      id: "request/more",
      type: "assistant",
      content: [
        { type: "reasoning", text: "The build passed. I’ll inspect the remaining changes." },
      ],
    },
    { id: "request/check", type: "assistant", content: [shell("diff", "git diff --check")] },
    {
      id: "request/result",
      type: "assistant",
      time: { completed: 95000 },
      content: [
        {
          type: "text",
          text: "The build passed and the working tree is clean.",
          conversationDecision: true,
        },
      ],
    },
  ],
});
const record = { ...codex, requests: [request], active_request: "" } as Conversation;

test("activity discloses narration, groups, rows, and one terminal while retaining nested choices", async () => {
  const screen = await render(SessionTurn, { project: "project", record, turn: turn() });
  await expect
    .element(screen.getByText("The build passed and the working tree is clean."))
    .toBeVisible();
  await expect
    .element(screen.getByText("I’ll check the build and test results."))
    .not.toBeVisible();
  await screen.getByText("Worked for 1m 34s", { exact: true }).click();
  await expect.element(screen.getByText("I’ll check the build and test results.")).toBeVisible();
  await expect
    .element(screen.getByText("The build passed. I’ll inspect the remaining changes."))
    .toBeVisible();
  await expect.element(screen.getByText("Ran go test ./...", { exact: true })).not.toBeVisible();
  await screen.getByText("Ran commands", { exact: true }).first().click();
  await expect.element(screen.getByText("Ran go test ./...", { exact: true })).toBeVisible();
  await expect
    .element(
      screen.getByRole("region", { name: "Shell command and output", includeHidden: true }).first(),
    )
    .not.toBeVisible();
  await screen.getByText("Ran go test ./...", { exact: true }).click();
  const terminal = screen
    .getByRole("region", { name: "Shell command and output", includeHidden: true })
    .first();
  await expect.element(terminal).toBeVisible();
  expect(terminal.element().textContent).toBe("$ go test ./...\npassed\n");
  expect(terminal.element().closest(".terminal")?.querySelectorAll("pre")).toHaveLength(1);
  await screen.getByText("Worked for 1m 34s", { exact: true }).click();
  await screen.getByText("Worked for 1m 34s", { exact: true }).click();
  await expect.element(terminal).toBeVisible();
  await screen.getByText("Ran commands", { exact: true }).first().click();
  await screen.getByText("Ran commands", { exact: true }).first().click();
  await expect.element(terminal).toBeVisible();
  await screen.getByText("Ran go test ./...", { exact: true }).click();
  await expect.element(terminal).not.toBeVisible();
});

test("streaming updates preserve disclosure through completion and surface error/stopped state", async () => {
  const current = turn("delivered");
  current.messages = current.messages.slice(0, 3);
  current.messages[2].content = [shell("build", "go test ./...", "running", "starting\n")];
  const props = {
    project: "project",
    record: { ...record, active_request: "request" },
    turn: current,
  };
  const screen = await render(SessionTurn, props);
  await screen.getByText(/^Working for/).click();
  await screen.getByText("Running commands", { exact: true }).click();
  await screen.getByText("Running go test ./...", { exact: true }).click();
  const terminal = screen.getByRole("region", { name: "Shell command and output" }).first();
  await expect.element(terminal).toBeVisible();
  const updated = structuredClone(current);
  updated.messages[2].content = [
    shell("build", "go test ./...", "running", "starting\nnext chunk\n"),
    shell("second", "git diff --check"),
  ];
  await screen.rerender({ ...props, turn: updated, record: { ...props.record, revision: 2 } });
  await expect.element(terminal).toBeVisible();
  expect(terminal.element().textContent).toContain("next chunk");
  await expect.element(screen.getByText("Ran git diff --check", { exact: true })).toBeVisible();
  await screen.getByText("Running go test ./...", { exact: true }).click();
  const completed = structuredClone(updated);
  completed.request.status = "completed";
  completed.messages[2].content[0].state.status = "completed";
  completed.messages.push({
    id: "request/result",
    type: "assistant",
    time: { completed: 95000 },
    content: [{ type: "text", text: "Everything passed.", conversationDecision: true }],
  });
  await screen.rerender({ ...props, turn: completed, record: { ...record, revision: 3 } });
  await expect
    .element(
      screen.getByRole("region", { name: "Shell command and output", includeHidden: true }).first(),
    )
    .not.toBeVisible();
  await expect.element(screen.getByText("Everything passed.", { exact: true })).toBeVisible();
  await screen.getByText("Ran go test ./...", { exact: true }).click();
  await expect.element(terminal).toBeVisible();
  updated.request.status = "canceled";
  updated.messages[2].content[0].state.status = "interrupted";
  await screen.rerender({
    ...props,
    turn: structuredClone(updated),
    record: { ...record, revision: 3 },
  });
  await expect.element(terminal).toBeVisible();
  await expect.element(screen.getByText("Stopped go test ./...", { exact: true })).toBeVisible();
  await expect.element(screen.getByText("interrupted", { exact: true })).toBeVisible();
  const failed = structuredClone(updated);
  failed.request.status = "failed";
  failed.request.error = "Build could not finish";
  failed.messages[2].content[0].state.status = "error";
  await screen.rerender({ ...props, turn: failed, record: { ...record, revision: 4 } });
  await expect.element(screen.getByText("1 failed", { exact: true })).toBeVisible();
  await expect.element(screen.getByRole("alert")).toHaveTextContent("Build could not finish");
});
