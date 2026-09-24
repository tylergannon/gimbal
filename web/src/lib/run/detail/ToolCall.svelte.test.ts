import { expect, test } from "vite-plus/test";
import { render } from "vitest-browser-svelte";
import ToolCall from "./ToolCall.svelte";

test("a nested string field's real newline survives as a line break, not literal backslash-n text", async () => {
  // `edits` is an object, so ToolCall's kv-value goes through
  // JSON.stringify to render it, which escapes an embedded real newline to
  // the two characters "\" and "n". The pane is for reading, so that escape
  // must be undone before display.
  const part = {
    id: "call_1",
    type: "tool",
    name: "Edit",
    state: {
      status: "completed",
      input: {
        edits: { note: "line one\nline two" },
      },
    },
  };

  const screen = await render(ToolCall, { part });
  await screen.getByRole("button", { name: /Edit/ }).click();
  await expect.element(screen.getByText("Edits", { exact: true })).toBeVisible();
  expect(document.body.textContent).not.toContain("line one\\nline two");
  expect(document.body.textContent).toContain("line one");
  expect(document.body.textContent).toContain("line two");
});
