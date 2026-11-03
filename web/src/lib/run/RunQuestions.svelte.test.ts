import { expect, test, vi } from "vite-plus/test";
import { render } from "vitest-browser-svelte";
import Questions from "./RunQuestions.svelte";
const remote = vi.hoisted(() => ({ sent: vi.fn() }));
vi.mock("../../routes/interactive.remote", () => ({
  finishInteractive: vi.fn(async () => ({ accepted: true })),
  answerInteractive: {
    for: () => {
      const field = (name: string) => ({
        as: (type: string, value?: string) => ({ name, type, value }),
      });
      return {
        pending: 0,
        fields: {
          request_id: field("request_id"),
          run: field("run"),
          question_id: field("question_id"),
          answer: field("answer"),
          allIssues: () => [],
        },
        enhance: (handler: (value: unknown) => Promise<void>) => ({
          onsubmit: async (event: SubmitEvent) => {
            event.preventDefault();
            remote.sent(Object.fromEntries(new FormData(event.currentTarget as HTMLFormElement)));
            await handler({ submit: async () => true, result: { accepted: true } });
          },
        }),
      };
    },
  },
}));
test("a retained question answers only its owning run and question", async () => {
  const question = {
    run: "run-a",
    question_id: "question-a",
    consumed: [],
    ordinal: 1,
    unresolved: "",
    address: "preferences",
    messages: [{ role: "assistant", message: "Which color?" }],
    ended_by: "",
    name: "preferences",
    scope: "",
    session: "session-a",
    question: "Which color?",
    status: "pending" as const,
    answer: "",
    asked: 1,
    answered: 0,
  };
  const screen = await render(Questions, { run: "run-a", question });
  await screen.getByRole("textbox", { name: "Answer the waiting question" }).fill("Blue");
  screen.container.hidden = true;
  screen.container.hidden = false;
  await screen.getByRole("button", { name: "Send answer", exact: true }).click();
  expect(remote.sent).toHaveBeenCalledWith(
    expect.objectContaining({
      run: "run-a",
      question_id: "preferences",
      answer: "Blue",
    }),
  );
  await expect.element(screen.getByText("Answer accepted.", { exact: true })).toBeVisible();
});

test("an initial interaction explains the next action in both card and composer", async () => {
  const question = {
    run: "run-a",
    question_id: "question-a",
    consumed: [],
    ordinal: 1,
    unresolved: "",
    address: "conversation",
    messages: [],
    ended_by: "",
    name: "conversation",
    scope: "",
    session: "session-a",
    question: "",
    status: "pending" as const,
    answer: "",
    asked: 1,
    answered: 0,
  };
  const screen = await render(Questions, { run: "run-a", question });
  await expect
    .element(
      screen.getByText(
        "Send a message to start this interaction. The purpose provides context for the agent.",
      ),
    )
    .toBeVisible();
  await screen.getByRole("textbox", { name: "First message" }).fill("Please start here.");
  await screen.getByRole("button", { name: "Send message", exact: true }).click();
  expect(remote.sent).toHaveBeenCalledWith(
    expect.objectContaining({ answer: "Please start here." }),
  );
  await screen.rerender({ run: "run-a", question, composer: true });
  await expect
    .element(
      screen.getByText(
        "Send a message to start this interaction. The purpose provides context for the agent.",
      ),
    )
    .toBeVisible();
});

test("attention cards preserve multiline replies and literal JSON", async () => {
  const reply = 'First line\n\nSecond line\n{"key":"value"}';
  const question = {
    run: "run-a",
    question_id: "question-a",
    consumed: [],
    ordinal: 1,
    unresolved: "",
    address: "conversation",
    messages: [{ role: "assistant", message: reply }],
    ended_by: "",
    name: "conversation",
    scope: "",
    session: "session-a",
    question: reply,
    status: "pending" as const,
    answer: "",
    asked: 1,
    answered: 0,
  };
  const screen = await render(Questions, { run: "run-a", question });
  const heading = screen.container.querySelector("h2")!;
  expect(heading.textContent).toBe(reply);
  expect(getComputedStyle(heading).whiteSpace).toBe("pre-wrap");
});
