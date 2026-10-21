import { expect, test, vi } from "vite-plus/test";
import { render } from "vitest-browser-svelte";
import type { LoopMessage } from "../../routes/steer.remote.js";
import { RunObservation } from "../observation/index.js";
import DetailPane from "./DetailPane.svelte";
import {
  implementInteractiveFixture,
  issue325Finished,
  issue325Fixture,
  planTripFixture,
  serviceOwnershipFixture,
} from "./fixtures/index.js";

const steerRemote = vi.hoisted(() => ({ sent: vi.fn() }));
vi.mock("../../routes/steer.remote", () => ({
  steer: {
    for: () => ({
      pending: 0,
      fields: {
        ...Object.fromEntries(
          ["run", "session", "message"].map((name) => [
            name,
            {
              as: (type: string, value?: string) => ({
                name,
                type,
                value: value ?? (name === "message" ? "Draft steer" : ""),
              }),
              value: () => "Draft steer",
            },
          ]),
        ),
        allIssues: () => [],
      },
      enhance: () => ({
        onsubmit: (event: SubmitEvent) => {
          event.preventDefault();
          steerRemote.sent(
            Object.fromEntries(new FormData(event.currentTarget as HTMLFormElement)),
          );
        },
      }),
    }),
  },
}));

const interactiveRemote = vi.hoisted(() => ({
  sent: vi.fn(),
  finish: vi.fn(async () => ({ accepted: true })),
}));
vi.mock("../../routes/interactive.remote", () => ({
  finishInteractive: interactiveRemote.finish,
  answerInteractive: {
    for: () => ({
      pending: 0,
      fields: {
        ...Object.fromEntries(
          ["request_id", "run", "question_id", "answer"].map((name) => [
            name,
            { as: (type: string, value?: string) => ({ name, type, value }) },
          ]),
        ),
        allIssues: () => [],
      },
      enhance: (handler: (value: unknown) => Promise<void>) => ({
        onsubmit: async (event: SubmitEvent) => {
          event.preventDefault();
          interactiveRemote.sent(
            Object.fromEntries(new FormData(event.currentTarget as HTMLFormElement)),
          );
          await handler({ submit: async () => true, result: { accepted: true } });
        },
      }),
    }),
  },
}));
test("a selected interaction owns its durable message and finish controls", async () => {
  const operation = planTripFixture.graph.body[0];
  if (operation.kind !== "group") throw new Error("missing group");
  const interactive = operation.children[0].body.find((item) => item.kind === "interactive");
  if (!interactive) throw new Error("missing interaction");
  const selection = {
    kind: "node" as const,
    scope: planTripFixture.snapshot.scopes["research.1/lodging.1"],
    operation: interactive,
    runtime: planTripFixture.snapshot.interactions["01M2RWXNFFYQA4HHQWMQ252CYA"],
  };
  const screen = await render(DetailPane, {
    snapshot: planTripFixture.snapshot,
    observation: new RunObservation(planTripFixture.snapshot),
    selection,
  });
  await expect
    .element(screen.getByText("Would you trade reliable Wi-Fi for a more secluded cabin?"))
    .toBeVisible();
  await screen
    .getByRole("textbox", { name: "Answer the waiting question" })
    .fill("Yes, for a weekend.");
  await screen.getByRole("button", { name: "Send answer", exact: true }).click();
  expect(interactiveRemote.sent).toHaveBeenCalledWith(
    expect.objectContaining({
      run: planTripFixture.snapshot.run.id,
      question_id: "research.1/lodging.1/preferences",
      answer: "Yes, for a weekend.",
    }),
  );
  await screen.getByRole("button", { name: "Finish interaction", exact: true }).click();
  expect(interactiveRemote.finish).toHaveBeenCalledWith(
    expect.objectContaining({
      run: planTripFixture.snapshot.run.id,
      address: "research.1/lodging.1/preferences",
    }),
  );
});

test("exchange detail renders server Markdown and keeps user and code payloads literal", async () => {
  const snapshot = structuredClone(planTripFixture.snapshot);
  const operation = planTripFixture.graph.body[0];
  if (operation.kind !== "group") throw new Error("missing group");
  const interactive = operation.children[0].body.find((item) => item.kind === "interactive");
  if (!interactive) throw new Error("missing interaction");
  const runtime = snapshot.interactions["01M2RWXNFFYQA4HHQWMQ252CYA"];
  runtime.messages = [
    {
      role: "user",
      message: "**literal user** <script>literal</script>",
      html: "<b>must not render</b>",
    },
    {
      role: "assistant",
      message: "**Startup** uses `Application.swift`.",
      html: '<p><strong>Startup</strong> uses <code>Application.swift</code>. See <a href="https://example.com/docs">quickstart</a>.</p><pre><code>{"literal":"**text** &lt;tag&gt;"}</code></pre>',
    },
  ];
  const screen = await render(DetailPane, {
    snapshot,
    selection: {
      kind: "node",
      scope: snapshot.scopes["research.1/lodging.1"],
      operation: interactive,
      runtime,
    },
  });
  await expect
    .element(screen.getByRole("link", { name: "quickstart" }))
    .toHaveAttribute("href", "https://example.com/docs");
  expect(document.querySelector(".exchange-prose strong")?.textContent).toBe("Startup");
  expect(document.querySelector(".exchange-prose code")?.textContent).toBe("Application.swift");
  expect(document.querySelector(".exchange-prose pre code")?.textContent).toBe(
    '{"literal":"**text** <tag>"}',
  );
  await expect
    .element(screen.getByText("**literal user** <script>literal</script>", { exact: true }))
    .toBeVisible();
  expect(document.querySelector(".exchanges script")).toBeNull();
  expect(document.querySelector(".exchanges b")).toBeNull();
  const empty = structuredClone(snapshot);
  for (const row of Object.values(empty.interactions)) row.messages = [];
  await screen.rerender({
    snapshot: empty,
    selection: {
      kind: "node",
      scope: empty.scopes["research.1/lodging.1"],
      operation: interactive,
      runtime: empty.interactions[runtime.question_id],
    },
  });
  await expect.element(screen.getByText("Exchanges", { exact: true })).toBeVisible();
  expect(document.querySelectorAll(".exchange-prose")).toHaveLength(0);
});

test("the selected loop emits the existing wrap-up control payload", async () => {
  const sent: LoopMessage[] = [];
  const selection = {
    kind: "sheet" as const,
    scope: implementInteractiveFixture.snapshot.scopes["implementation.1"],
  };
  const screen = await render(DetailPane, {
    snapshot: implementInteractiveFixture.snapshot,
    observation: new RunObservation(implementInteractiveFixture.snapshot),
    selection,
    onloop: async (message: LoopMessage) => {
      sent.push(message);
      return { ok: true, message: "waiting" };
    },
  });

  await screen.getByRole("tab", { name: "Context" }).click();

  const refreshed = structuredClone(implementInteractiveFixture.snapshot);
  refreshed.scopes["implementation.1"].values.backlog = { value: "one task remains" };
  await screen.rerender({
    snapshot: refreshed,
    observation: new RunObservation(refreshed),
    selection,
    onloop: async (message: LoopMessage) => {
      sent.push(message);
      return { ok: true, message: "waiting" };
    },
  });
  await screen.getByText("backlog", { exact: true }).click();
  await expect.element(screen.getByText("one task remains")).toBeVisible();

  await screen.getByRole("button", { name: "Wrap up" }).click();
  expect(sent).toEqual([
    {
      run: implementInteractiveFixture.snapshot.run.id,
      scope: "implementation.1",
      message: "",
      wrap_up: true,
    },
  ]);
});

test("steering emits the selected runtime session and entered message", async () => {
  const loop = implementInteractiveFixture.graph.body.find(
    (operation) => operation.kind === "promise_loop",
  );
  const coding = loop?.body.find((operation) => operation.kind === "agent_call");
  if (!coding || coding.kind !== "agent_call") throw new Error("coding node is missing");
  const turn = implementInteractiveFixture.snapshot.turns["coding.1/turn.3"];
  steerRemote.sent.mockClear();
  const screen = await render(DetailPane, {
    snapshot: implementInteractiveFixture.snapshot,
    observation: new RunObservation(implementInteractiveFixture.snapshot),
    selection: {
      kind: "node" as const,
      scope: implementInteractiveFixture.snapshot.scopes["implementation.1/task.3"],
      operation: coding,
      runtime: turn,
    },
  });

  await screen.getByPlaceholder("Steer this turn…").fill("Check the failing assertion first.");
  await screen.getByRole("button", { name: "Send steer" }).click();
  expect(steerRemote.sent).toHaveBeenCalledWith({
    run: implementInteractiveFixture.snapshot.run.id,
    session: turn.session,
    message: "Check the failing assertion first.",
  });
});

test("an unobserved guarded command does not inherit its ended scope state", async () => {
  const loop = implementInteractiveFixture.graph.body.find(
    (operation) => operation.kind === "promise_loop",
  );
  const command = loop?.body.find((operation) => operation.kind === "command");
  if (!command || command.kind !== "command") throw new Error("command node is missing");

  const screen = await render(DetailPane, {
    snapshot: implementInteractiveFixture.snapshot,
    selection: {
      kind: "node",
      scope: implementInteractiveFixture.snapshot.scopes["implementation.1/task.3"],
      operation: command,
    },
  });

  await expect.element(screen.getByText("Not observed")).toBeVisible();
  expect(document.querySelector('aside [data-state="queued"]')).not.toBeNull();
});

test("a service selection shows declaration ownership and source without runtime status", async () => {
  const operation = serviceOwnershipFixture.graph.body.find(
    (candidate) => candidate.kind === "scope",
  );
  if (!operation || operation.kind !== "scope") throw new Error("backend scope is missing");
  const service = operation.services[0];
  if (!service) throw new Error("api service is missing");

  const screen = await render(DetailPane, {
    snapshot: serviceOwnershipFixture.snapshot,
    selection: {
      kind: "service",
      scope: serviceOwnershipFixture.snapshot.scopes["backend.1"],
      service,
    },
  });

  await expect.element(screen.getByText("Declared service")).toBeVisible();
  await expect.element(screen.getByText("examples/services/services.go:15")).toBeVisible();
  expect(document.querySelector("aside .title-row [data-state]")).toBeNull();
});

test("a service process command selected from search retains command detail", async () => {
  const operation = serviceOwnershipFixture.graph.body.find(
    (candidate) => candidate.kind === "scope",
  );
  if (!operation || operation.kind !== "scope") throw new Error("backend scope is missing");
  const service = operation.services[0];
  if (!service) throw new Error("api service is missing");
  const runtime = serviceOwnershipFixture.snapshot.commands["backend.1/api.1"];

  const screen = await render(DetailPane, {
    snapshot: serviceOwnershipFixture.snapshot,
    selection: {
      kind: "service",
      scope: serviceOwnershipFixture.snapshot.scopes["backend.1"],
      service,
      runtime,
    },
  });

  await expect.element(screen.getByText("This instance")).toBeVisible();
  await expect.element(screen.getByText("api", { exact: true }).first()).toBeVisible();
  await expect.element(screen.getByText("Declared service")).not.toBeInTheDocument();
});

test("command detail exposes both streams and their complete-output references", async () => {
  const loop = implementInteractiveFixture.graph.body.find(
    (operation) => operation.kind === "promise_loop",
  );
  const command = loop?.body.find(
    (operation) => operation.kind === "command" && operation.name === "build",
  );
  if (!command || command.kind !== "command") throw new Error("build command is missing");

  const snapshot = structuredClone(implementInteractiveFixture.snapshot);
  const runtime = snapshot.commands["implementation.1/task.1/build.1"];
  runtime.stdout = "build completed\n";
  runtime.stderr = "development warning\n";
  runtime.stdout_file = "artifacts/commands/build.1/stdout.log";
  runtime.stderr_file = "artifacts/commands/build.1/stderr.log";

  const screen = await render(DetailPane, {
    snapshot,
    selection: {
      kind: "node",
      scope: snapshot.scopes["implementation.1/task.1"],
      operation: command,
      runtime,
    },
  });

  await expect.element(screen.getByText("build completed")).toBeVisible();
  await expect.element(screen.getByText("development warning")).toBeVisible();
  await expect.element(screen.getByText("artifacts/commands/build.1/stdout.log")).toBeVisible();
  await expect.element(screen.getByText("artifacts/commands/build.1/stderr.log")).toBeVisible();
});

test("watchers and definition disclosures show graph facts and honest empty states", async () => {
  const loop = implementInteractiveFixture.graph.body.find(
    (operation) => operation.kind === "promise_loop",
  );
  const coding = loop?.body.find((operation) => operation.kind === "agent_call");
  if (!coding || coding.kind !== "agent_call") throw new Error("coding node is missing");

  const screen = await render(DetailPane, {
    snapshot: implementInteractiveFixture.snapshot,
    observation: new RunObservation(implementInteractiveFixture.snapshot),
    selection: {
      kind: "node",
      scope: implementInteractiveFixture.snapshot.scopes["implementation.1/task.3"],
      operation: coding,
      runtime: implementInteractiveFixture.snapshot.turns["coding.1/turn.3"],
    },
  });

  await screen.getByRole("tab", { name: "Result" }).click();
  await expect
    .element(screen.getByText("implementation-scope-review", { exact: true }).first())
    .toBeVisible();
  await expect
    .element(screen.getByText("architectural-critique", { exact: true }).first())
    .toBeVisible();
  await screen.getByText("architectural-critique", { exact: true }).first().click();
  await expect
    .element(screen.getByText("No completed watcher result has been recorded."))
    .toBeVisible();
  await expect
    .element(screen.getByText("No watcher turn has been recorded in this scope."))
    .not.toBeInTheDocument();

  await screen.getByRole("tab", { name: "Source" }).click();
  await expect.element(screen.getByText("coding", { exact: true }).last()).toBeVisible();
  await expect.element(screen.getByText(coding.prompt)).toBeVisible();
});

test("a selected watcher distinguishes objections, no objections, and no recorded turn", async () => {
  const loop = implementInteractiveFixture.graph.body.find(
    (operation) => operation.kind === "promise_loop",
  );
  const coding = loop?.body.find((operation) => operation.kind === "agent_call");
  if (!coding || coding.kind !== "agent_call") throw new Error("coding node is missing");
  const supervisor = coding.supervisors.find(
    (candidate) => candidate.session === "implementation-scope-review",
  );
  if (!supervisor) throw new Error("watcher definition is missing");

  const noObjections = structuredClone(implementInteractiveFixture.snapshot);
  const scope = noObjections.scopes["implementation.1/task.3"];
  const turn = noObjections.turns["implementation-scope-review.1/turn.3"];
  turn.result = '{"objections":[]}';
  const screen = await render(DetailPane, {
    snapshot: noObjections,
    observation: new RunObservation(noObjections),
    selection: { kind: "watcher", scope, supervisor, turn },
  });

  await expect
    .element(screen.getByRole("tab", { name: "Result" }))
    .toHaveAttribute("aria-selected", "true");
  await expect.element(screen.getByText("Objections", { exact: true })).toBeVisible();
  await expect.element(screen.getByText("None", { exact: true })).toBeVisible();

  const objections = structuredClone(noObjections);
  const objectingTurn = objections.turns["implementation-scope-review.1/turn.3"];
  objectingTurn.result = '{"objections":["remove the wrapper"]}';
  await screen.rerender({
    snapshot: objections,
    observation: new RunObservation(objections),
    selection: {
      kind: "watcher",
      scope: objections.scopes["implementation.1/task.3"],
      supervisor,
      turn: objectingTurn,
    },
  });
  await expect.element(screen.getByText(/remove the wrapper/)).toBeVisible();

  await screen.rerender({
    snapshot: implementInteractiveFixture.snapshot,
    observation: new RunObservation(implementInteractiveFixture.snapshot),
    selection: {
      kind: "watcher",
      scope: implementInteractiveFixture.snapshot.scopes["implementation.1/task.1"],
      supervisor,
    },
  });
  await expect
    .element(screen.getByText("No watcher turn has been recorded in this scope."))
    .toBeVisible();
  await expect.element(screen.getByText(supervisor.instruction)).toBeVisible();
});

function issue325Coding() {
  const implementation = issue325Fixture.graph.body.find(
    (operation) => operation.kind === "promise_loop",
  );
  const coding = implementation?.body.find((operation) => operation.kind === "agent_call");
  if (!coding || coding.kind !== "agent_call") throw new Error("coding node is missing");
  return coding;
}

test("a running turn opens on Activity, not Result", async () => {
  const coding = issue325Coding();
  const scope = issue325Fixture.snapshot.scopes["implementation.1/task.1"];
  const turn = issue325Fixture.snapshot.turns["coding.1/turn.2"];

  const screen = await render(DetailPane, {
    snapshot: issue325Fixture.snapshot,
    observation: new RunObservation(issue325Fixture.snapshot),
    selection: { kind: "node", scope, operation: coding, runtime: turn },
  });

  await expect
    .element(screen.getByRole("tab", { name: "Activity" }))
    .toHaveAttribute("aria-selected", "true");
  await expect
    .element(screen.getByRole("tab", { name: "Result" }))
    .toHaveAttribute("aria-selected", "false");
});

test("a finished turn opens on Result and shows turn.result", async () => {
  const coding = issue325Coding();
  const scope = issue325Finished.scopes["implementation.1/task.1"];
  const turn = issue325Finished.turns["coding.1/turn.2"];

  const screen = await render(DetailPane, {
    snapshot: issue325Finished,
    observation: new RunObservation(issue325Finished),
    selection: { kind: "node", scope, operation: coding, runtime: turn },
  });

  await expect
    .element(screen.getByRole("tab", { name: "Result" }))
    .toHaveAttribute("aria-selected", "true");
  await expect.element(screen.getByText(/Added the document-level dragend listener/)).toBeVisible();
});

test("the assignment lives only on the scope's Overview, not the agent-turn pane", async () => {
  const coding = issue325Coding();
  const scope = issue325Fixture.snapshot.scopes["implementation.1/task.1"];
  const turn = issue325Fixture.snapshot.turns["coding.1/turn.2"];
  const assignmentSnippet = "Implement GitHub issue #1: replace Scrabbler's click-to-place";

  const turnScreen = await render(DetailPane, {
    snapshot: issue325Fixture.snapshot,
    observation: new RunObservation(issue325Fixture.snapshot),
    selection: { kind: "node", scope, operation: coding, runtime: turn },
  });
  expect(document.body.textContent).not.toContain(assignmentSnippet);

  const scopeScreen = await render(DetailPane, {
    snapshot: issue325Fixture.snapshot,
    selection: { kind: "sheet", scope },
  });
  await expect
    .element(scopeScreen.getByText(new RegExp(assignmentSnippet.slice(0, 40))))
    .toBeVisible();
  void turnScreen;
});

test("delivery uncertainty renders unresolved rather than a failed native outcome", async () => {
  const snapshot = structuredClone(issue325Finished);
  const turn = snapshot.turns["coding.1/turn.2"];
  turn.unresolved = true;
  turn.interrupted = false;
  turn.result = "";
  turn.error = "request outcome unresolved: listener loss after retained native completion";
  turn.events_unavailable = true;
  turn.observation_error = turn.error;
  const screen = await render(DetailPane, {
    snapshot,
    observation: new RunObservation(snapshot),
    selection: {
      kind: "node",
      scope: snapshot.scopes["implementation.1/task.1"],
      operation: issue325Coding(),
      runtime: turn,
    },
  });
  await expect.element(screen.getByText("unresolved", { exact: true }).first()).toBeVisible();
  await expect.element(screen.getByText("failed", { exact: true })).not.toBeInTheDocument();
  await expect.element(screen.getByText("delivery problem", { exact: true })).toBeVisible();
  expect(document.querySelectorAll('.pip[data-state="needs-you"]').length).toBeGreaterThan(0);
  expect(document.querySelectorAll('.pip[data-state="failed"]').length).toBe(0);
});
