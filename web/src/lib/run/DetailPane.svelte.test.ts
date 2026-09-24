import { expect, test } from "vite-plus/test";
import { render } from "vitest-browser-svelte";
import type { InterviewAnswer } from "../../routes/interview.remote.js";
import type { LoopMessage, Steer } from "../../routes/steer.remote.js";
import { RunObservation } from "../observation/index.js";
import DetailPane from "./DetailPane.svelte";
import {
  implementInterviewFixture,
  issue325Finished,
  issue325Fixture,
  planTripFixture,
  serviceOwnershipFixture,
} from "./fixtures/index.js";

test("a selected interview follows the next pending question and submits its identity", async () => {
  const operation = planTripFixture.graph.body[0];
  if (operation.kind !== "group") throw new Error("plan-trip interview group is missing");
  const interview = operation.children[0].body.find((item) => item.kind === "interview");
  if (!interview || interview.kind !== "interview") throw new Error("interview node is missing");

  const first = planTripFixture.snapshot.interviews["01M2RWXNFFYQA4HHQWMQ252CYA"];
  const submitted: InterviewAnswer[] = [];
  const selection = {
    kind: "node" as const,
    scope: planTripFixture.snapshot.scopes["research.1/lodging.1"],
    operation: interview,
    runtime: first,
  };
  const screen = await render(DetailPane, {
    snapshot: planTripFixture.snapshot,
    observation: new RunObservation(planTripFixture.snapshot),
    selection,
    onanswer: async (answer: InterviewAnswer) => {
      submitted.push(answer);
      return { ok: true, message: "accepted" };
    },
  });

  await expect
    .element(screen.getByText("Would you trade reliable Wi-Fi for a more secluded cabin?"))
    .toBeVisible();
  await screen.getByLabelText("Your answer").fill("Yes, for a weekend.");
  await screen.getByRole("button", { name: "Answer" }).click();
  expect(submitted[0]?.question_id).toBe("01M2RWXNFFYQA4HHQWMQ252CYF");

  const refreshed = structuredClone(planTripFixture.snapshot);
  refreshed.interviews["01M2RWXNFFYQA4HHQWMQ252CYF"].status = "answered";
  refreshed.interviews["01M2RWXNFFYQA4HHQWMQ252CYF"].answer = "Yes, for a weekend.";
  refreshed.interviews["01M2RWXNFFYQA4HHQWMQ252CYF"].answered += 10_000;
  refreshed.interviews["question-3"] = {
    ...refreshed.interviews["01M2RWXNFFYQA4HHQWMQ252CYF"],
    question_id: "question-3",
    question: "How remote is too remote?",
    status: "pending",
    answer: "",
    asked: refreshed.interviews["01M2RWXNFFYQA4HHQWMQ252CYF"].asked + 20_000,
    answered: 0,
  };
  await screen.rerender({
    snapshot: refreshed,
    observation: new RunObservation(refreshed),
    selection,
    onanswer: async (answer: InterviewAnswer) => {
      submitted.push(answer);
      return { ok: true, message: "accepted" };
    },
  });
  await expect.element(screen.getByText("How remote is too remote?")).toBeVisible();
  await screen.getByLabelText("Your answer").fill("An hour from groceries.");
  await screen.getByRole("button", { name: "Answer" }).click();
  expect(submitted.at(-1)?.question_id).toBe("question-3");
});

test("the selected loop emits the existing wrap-up control payload", async () => {
  const sent: LoopMessage[] = [];
  const selection = {
    kind: "sheet" as const,
    scope: implementInterviewFixture.snapshot.scopes["implementation.1"],
  };
  const screen = await render(DetailPane, {
    snapshot: implementInterviewFixture.snapshot,
    observation: new RunObservation(implementInterviewFixture.snapshot),
    selection,
    onloop: async (message: LoopMessage) => {
      sent.push(message);
      return { ok: true, message: "waiting" };
    },
  });

  await screen.getByRole("tab", { name: "Context" }).click();

  const refreshed = structuredClone(implementInterviewFixture.snapshot);
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
      run: implementInterviewFixture.snapshot.run.id,
      scope: "implementation.1",
      message: "",
      wrap_up: true,
    },
  ]);
});

test("steering emits the selected runtime session and entered message", async () => {
  const loop = implementInterviewFixture.graph.body.find(
    (operation) => operation.kind === "promise_loop",
  );
  const coding = loop?.body.find((operation) => operation.kind === "agent_call");
  if (!coding || coding.kind !== "agent_call") throw new Error("coding node is missing");
  const turn = implementInterviewFixture.snapshot.turns["coding.1/turn.3"];
  const sent: Steer[] = [];
  const screen = await render(DetailPane, {
    snapshot: implementInterviewFixture.snapshot,
    observation: new RunObservation(implementInterviewFixture.snapshot),
    selection: {
      kind: "node" as const,
      scope: implementInterviewFixture.snapshot.scopes["implementation.1/task.3"],
      operation: coding,
      runtime: turn,
    },
    onsteer: async (message: Steer) => {
      sent.push(message);
      return { ok: true, message: "landed" };
    },
  });

  await screen.getByPlaceholder("Steer this turn…").fill("Check the failing assertion first.");
  await screen.getByRole("button", { name: "Steer" }).click();
  expect(sent).toEqual([
    {
      run: implementInterviewFixture.snapshot.run.id,
      session: turn.session,
      message: "Check the failing assertion first.",
    },
  ]);
});

test("an unobserved guarded command does not inherit its ended scope state", async () => {
  const loop = implementInterviewFixture.graph.body.find(
    (operation) => operation.kind === "promise_loop",
  );
  const command = loop?.body.find((operation) => operation.kind === "command");
  if (!command || command.kind !== "command") throw new Error("command node is missing");

  const screen = await render(DetailPane, {
    snapshot: implementInterviewFixture.snapshot,
    selection: {
      kind: "node",
      scope: implementInterviewFixture.snapshot.scopes["implementation.1/task.3"],
      operation: command,
    },
  });

  await expect.element(screen.getByText("Not observed")).toBeVisible();
  expect(document.querySelector('aside [data-state="not-yet"]')).not.toBeNull();
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
  const loop = implementInterviewFixture.graph.body.find(
    (operation) => operation.kind === "promise_loop",
  );
  const command = loop?.body.find(
    (operation) => operation.kind === "command" && operation.name === "build",
  );
  if (!command || command.kind !== "command") throw new Error("build command is missing");

  const snapshot = structuredClone(implementInterviewFixture.snapshot);
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
  const loop = implementInterviewFixture.graph.body.find(
    (operation) => operation.kind === "promise_loop",
  );
  const coding = loop?.body.find((operation) => operation.kind === "agent_call");
  if (!coding || coding.kind !== "agent_call") throw new Error("coding node is missing");

  const screen = await render(DetailPane, {
    snapshot: implementInterviewFixture.snapshot,
    observation: new RunObservation(implementInterviewFixture.snapshot),
    selection: {
      kind: "node",
      scope: implementInterviewFixture.snapshot.scopes["implementation.1/task.3"],
      operation: coding,
      runtime: implementInterviewFixture.snapshot.turns["coding.1/turn.3"],
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
  const loop = implementInterviewFixture.graph.body.find(
    (operation) => operation.kind === "promise_loop",
  );
  const coding = loop?.body.find((operation) => operation.kind === "agent_call");
  if (!coding || coding.kind !== "agent_call") throw new Error("coding node is missing");
  const supervisor = coding.supervisors.find(
    (candidate) => candidate.session === "implementation-scope-review",
  );
  if (!supervisor) throw new Error("watcher definition is missing");

  const noObjections = structuredClone(implementInterviewFixture.snapshot);
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
    snapshot: implementInterviewFixture.snapshot,
    observation: new RunObservation(implementInterviewFixture.snapshot),
    selection: {
      kind: "watcher",
      scope: implementInterviewFixture.snapshot.scopes["implementation.1/task.1"],
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
