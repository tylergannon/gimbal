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
