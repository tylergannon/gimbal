import { expect, test } from "vitest";
import { render } from "vitest-browser-svelte";
import Transcript from "./ConversationTranscript.svelte";

test("a stopped partial reply remains marked after the next reply", async () => {
  const screen = await render(Transcript, {
    provider: "Codex",
    model: "gpt-5.6-luna",
    location: "This Mac",
    requests: [
      {
        id: "stopped",
        created: 1,
        kind: "prompt",
        message: "Start",
        target: "",
        file_id: "",
        status: "canceled",
        delivery: "turn",
        error: "",
      },
    ],
    messages: [
      {
        id: "stopped/step",
        type: "assistant",
        content: [{ type: "text", text: "Partial reply retained." }],
      },
      { id: "steer", type: "user", text: "Keep it concise", steered: true, requestID: "steer" },
      { id: "next/step", type: "assistant", content: [{ type: "text", text: "The next reply." }] },
    ],
  });
  await expect.element(screen.getByText("Partial reply retained.")).toBeVisible();
  await expect.element(screen.getByText("Stopped", { exact: true })).toBeVisible();
  await expect.element(screen.getByText("The next reply.")).toBeVisible();
  await expect.element(screen.getByText("Sent while Codex was working")).toBeVisible();
});

test("chat hides completion tools and their empty assistant rows but keeps real tool activity", async () => {
  const screen = await render(Transcript, {
    provider: "Claude Code",
    model: "Haiku 4.5",
    location: "This Mac",
    messages: [
      { id: "reply", type: "assistant", content: [{ type: "text", text: "Ready to help." }] },
      {
        id: "completion",
        type: "assistant",
        content: [
          { type: "reasoning", text: "Return the structured result." },
          {
            type: "tool",
            name: "StructuredOutput",
            state: { status: "success", input: { state: "completed", value: "Ready to help." } },
          },
        ],
      },
      {
        id: "mixed",
        type: "assistant",
        content: [
          { type: "text", text: "Read the file." },
          {
            type: "tool",
            name: "Read",
            state: { status: "success", input: { file_path: "/tmp/example.txt" } },
          },
          {
            type: "tool",
            name: "StructuredOutput",
            state: { status: "success", input: { state: "completed", value: "Read the file." } },
          },
        ],
      },
    ],
  });
  await expect.element(screen.getByText("Ready to help.", { exact: true })).toBeVisible();
  await expect.element(screen.getByText("Read", { exact: true }).first()).toBeVisible();
  await expect
    .element(screen.getByText("StructuredOutput", { exact: true }))
    .not.toBeInTheDocument();
  await expect.element(screen.getByText("Thinking", { exact: true })).not.toBeInTheDocument();
  expect(document.querySelectorAll("article.assistant")).toHaveLength(2);
});
