import { expect, test } from "vite-plus/test";
import { render } from "vitest-browser-svelte";
import type { InterviewAnswer } from "../../routes/interview.remote.js";
import type { LoopMessage } from "../../routes/steer.remote.js";
import { RunObservation } from "../observation/index.js";
import DetailPane from "./DetailPane.svelte";
import { implementInterviewFixture, planTripFixture } from "./fixtures/index.js";

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
