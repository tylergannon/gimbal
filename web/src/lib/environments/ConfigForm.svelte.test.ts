import { beforeEach, expect, test, vi } from "vite-plus/test";
import { render } from "vitest-browser-svelte";
import ConfigForm from "./ConfigForm.svelte";

const remote = vi.hoisted(() => ({ submitted: [] as FormData[] }));
vi.mock("#lib/environments.remote", () => ({
  saveEnvironmentConfig: {
    for: () => ({
      pending: 0,
      fields: {
        revision: { as: (type: string, value: string) => ({ name: "revision", type, value }) },
        text: { as: (_type: string, value: string) => ({ name: "text", value }) },
        allIssues: () => [],
      },
      enhance: (handler: (submitted: unknown) => Promise<void>) => ({
        onsubmit: async (event: SubmitEvent) => {
          event.preventDefault();
          remote.submitted.push(new FormData(event.currentTarget as HTMLFormElement));
          await handler({
            submit: async () => {
              throw new Error("The file changed. Reload before saving.");
            },
          });
        },
      }),
    }),
  },
}));

beforeEach(() => {
  remote.submitted = [];
});

test("file refresh cannot silently advance the revision of unsaved edits", async () => {
  const onsaved = vi.fn();
  const original = {
    path: "/catalog/environments.json",
    revision: "old-revision",
    text: '{"version":1}',
    valid: true,
    issues: [],
  };
  const screen = await render(ConfigForm, { document: original, onsaved, onreload: vi.fn() });
  const editor = screen.getByRole("textbox", { name: "Environment configuration JSON" });
  await editor.fill('{"version":1,"machines":[]}');
  await screen.rerender({
    document: {
      ...original,
      revision: "new-revision",
      text: '{"version":1,"machines":[{"id":"external"}]}',
    },
    onsaved,
    onreload: vi.fn(),
  });
  await screen.getByRole("button", { name: "Save file" }).click();
  expect(remote.submitted[0].get("revision")).toBe("old-revision");
  expect(remote.submitted[0].get("text")).toBe('{"version":1,"machines":[]}');
  await expect.element(editor).toHaveValue('{"version":1,"machines":[]}');
  await expect
    .element(screen.getByRole("alert"))
    .toHaveTextContent("The file changed. Reload before saving.");
  expect(onsaved).not.toHaveBeenCalled();
});
