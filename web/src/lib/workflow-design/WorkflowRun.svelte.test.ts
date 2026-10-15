import { expect, test, vi } from "vite-plus/test";
import { render } from "vitest-browser-svelte";
import WorkflowRun from "./WorkflowRun.svelte";
const remote = vi.hoisted(() => ({ submitted: [] as FormData[] }));
vi.mock("../../routes/launch.remote", () => {
  const field = (name: string) => ({
    as: (type: string, value?: unknown) => ({
      name,
      type,
      ...(value === undefined ? {} : { value: String(value) }),
    }),
    issues: () => [],
  });
  return {
    launchWorkflow: {
      for: () => ({
        pending: 0,
        fields: {
          workflow: field("workflow"),
          project: field("project"),
          workdir: field("workdir"),
          inputs: [
            {
              name: field("inputs[0].name"),
              value: field("inputs[0].value"),
              include: field("inputs[0].include"),
            },
          ],
          roles: [{ name: field("roles[0].name"), value: field("roles[0].value") }],
          allIssues: () => [],
        },
        enhance: (handler: (submitted: unknown) => Promise<void>) => ({
          onsubmit: async (event: SubmitEvent) => {
            event.preventDefault();
            remote.submitted.push(new FormData(event.currentTarget as HTMLFormElement));
            await handler({
              submit: async () => true,
              result: { project_id: "current-project", run_id: "admitted-run" },
            });
          },
        }),
      }),
    },
  };
});
test("workspace launch locks its project and submits the real goal and role model before opening admission", async () => {
  const opened: unknown[] = [];
  const guide = vi.fn();
  remote.submitted = [];
  const screen = await render(WorkflowRun, {
    workflow: {
      name: "implement",
      summary: "Make a reviewed change",
      help: "",
      fields: [
        {
          name: "goal",
          kind: "string",
          description: "What should change?",
          optional: false,
          nullable: false,
        },
      ],
      roles: [{ name: "coder", default: "haiku" }],
    },
    project: "/work/current-project",
    onstarted: (result) => opened.push(result),
    onguide: guide,
  });
  await expect.element(screen.getByRole("combobox", { name: "Project" })).not.toBeInTheDocument();
  await screen.getByRole("textbox", { name: "Goal", exact: true }).fill("Make the run legible");
  await screen.getByText("Model override", { exact: false }).click();
  await screen.getByRole("textbox", { name: "coder", exact: true }).fill("gpt-5.6-luna");
  await screen.getByRole("button", { name: "Read the guide" }).click();
  expect(guide).toHaveBeenCalledOnce();
  await screen.getByRole("button", { name: "Start run" }).click();
  expect(Object.fromEntries(remote.submitted[0])).toMatchObject({
    project: "/work/current-project",
    workflow: "implement",
    "inputs[0].name": "goal",
    "inputs[0].value": "Make the run legible",
    "roles[0].name": "coder",
    "roles[0].value": "gpt-5.6-luna",
  });
  expect(opened).toEqual([{ project_id: "current-project", run_id: "admitted-run" }]);
});
