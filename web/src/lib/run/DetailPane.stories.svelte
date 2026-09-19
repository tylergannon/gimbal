<script module lang="ts">
  import { defineMeta } from "@storybook/addon-svelte-csf";
  import DetailPane, {
    type CommandSelection,
    type InterviewSelection,
    type LoopSelection,
    type ScopeSelection,
    type TurnSelection,
  } from "./DetailPane.svelte";
  import { implementInterviewFixture, planTripFixture } from "./fixtures/index.js";

  function required<T>(value: T | undefined, message: string): T {
    if (!value) throw new Error(message);
    return value;
  }

  const implementGraph = implementInterviewFixture.graph;
  const implementSnapshot = implementInterviewFixture.snapshot;
  const loopOperation = required(
    implementGraph.body.find((operation) => operation.kind === "promise_loop"),
    "implementation loop is missing",
  );
  if (loopOperation.kind !== "promise_loop") throw new Error("implementation is not a loop");
  const turnOperation = required(
    loopOperation.body.find(
      (operation) => operation.kind === "agent_call" && operation.session === "coding",
    ),
    "coding call is missing",
  );
  const commandOperation = required(
    loopOperation.body
      .flatMap((operation) => (operation.kind === "condition" ? operation.branches : []))
      .flatMap((branch) => branch.body)
      .find((operation) => operation.kind === "command"),
    "task-check command is missing",
  );
  if (turnOperation.kind !== "agent_call" || commandOperation.kind !== "command") {
    throw new Error("detail operations have the wrong kind");
  }

  const turnSelection: TurnSelection = {
    kind: "turn",
    operation: turnOperation,
    turn: required(implementSnapshot.turns["coding.1/turn.3"], "coding turn is missing"),
    scope: required(
      implementSnapshot.scopes["implementation.1/task.3"],
      "task 3 scope is missing",
    ),
    session: required(implementSnapshot.sessions["coding.1"], "coding session is missing"),
  };

  const commandSelection: CommandSelection = {
    kind: "command",
    operation: commandOperation,
    command: required(
      implementSnapshot.commands?.["implementation.1/task.2/task-check.1"],
      "failed task check is missing",
    ),
    scope: required(
      implementSnapshot.scopes["implementation.1/task.2"],
      "task 2 scope is missing",
    ),
  };

  const scopeSelection: ScopeSelection = {
    kind: "scope",
    scope: required(
      implementSnapshot.scopes["implementation.1/task.2"],
      "task 2 scope is missing",
    ),
  };

  const loopSelection: LoopSelection = {
    kind: "loop",
    operation: loopOperation,
    scope: required(implementSnapshot.scopes["implementation.1"], "loop scope is missing"),
  };

  const planSnapshot = planTripFixture.snapshot;
  const researchGroup = required(
    planTripFixture.graph.body.find((operation) => operation.kind === "group"),
    "research group is missing",
  );
  if (researchGroup.kind !== "group") throw new Error("research is not a group");
  const interviewOperation = required(
    researchGroup.children[0]?.body.find((operation) => operation.kind === "interview"),
    "lodging interview is missing",
  );
  if (interviewOperation.kind !== "interview") throw new Error("preferences is not an interview");
  const pendingInterview = required(
    Object.values(planSnapshot.interviews).find(
      (row) => row.scope === "research.1/lodging.1" && row.status === "pending",
    ),
    "pending lodging question is missing",
  );
  const interviewSelection: InterviewSelection = {
    kind: "interview",
    operation: interviewOperation,
    interview: pendingInterview,
    scope: required(planSnapshot.scopes[pendingInterview.scope], "lodging scope is missing"),
    session: required(planSnapshot.sessions[pendingInterview.session], "interview session is missing"),
  };

  const { Story } = defineMeta({
    title: "Gimble/Run/DetailPane",
    component: DetailPane,
    parameters: { layout: "centered" },
  });
</script>

<script lang="ts">
  let turnEvent = $state("Use a disclosure or turn control");
  let commandEvent = $state("Open or close command output");
  let loopEvent = $state("Use the loop wrap-up control");
  let interviewEvent = $state("Answer, end, or open the other interview");
</script>

<Story name="Selected agent call" asChild>
  <div class="story-frame">
    <p class="event" aria-live="polite">{turnEvent}</p>
    <div class="pane-frame">
      <DetailPane
        selection={turnSelection}
        snapshot={implementSnapshot}
        onsteer={({ turn, message }) => (turnEvent = `steer · ${turn.id} · ${message}`)}
        onopentranscript={(turn) => (turnEvent = `open-transcript · ${turn.id}`)}
        ondisclosurechange={({ name, open }) =>
          (turnEvent = `disclosure · ${name} · ${open ? "open" : "closed"}`)}
      />
    </div>
  </div>
</Story>

<Story name="Selected command" asChild>
  <div class="story-frame">
    <p class="event" aria-live="polite">{commandEvent}</p>
    <div class="pane-frame">
      <DetailPane
        selection={commandSelection}
        snapshot={implementSnapshot}
        ondisclosurechange={({ name, open }) =>
          (commandEvent = `disclosure · ${name} · ${open ? "open" : "closed"}`)}
      />
    </div>
  </div>
</Story>

<Story name="Selected scope" asChild>
  <div class="story-frame">
    <p class="event">Fixture-backed values and sessions</p>
    <div class="pane-frame">
      <DetailPane selection={scopeSelection} snapshot={implementSnapshot} />
    </div>
  </div>
</Story>

<Story name="Selected loop" asChild>
  <div class="story-frame">
    <p class="event" aria-live="polite">{loopEvent}</p>
    <div class="pane-frame">
      <DetailPane
        selection={loopSelection}
        snapshot={implementSnapshot}
        onwrapup={({ scope, message }) => (loopEvent = `wrap-up · ${scope.key} · ${message}`)}
      />
    </div>
  </div>
</Story>

<Story name="Selected interview" asChild>
  <div class="story-frame">
    <p class="event" aria-live="polite">{interviewEvent}</p>
    <div class="pane-frame">
      <DetailPane
        selection={interviewSelection}
        snapshot={planSnapshot}
        onanswer={({ interview, answer }) =>
          (interviewEvent = `answer · ${interview.question_id} · ${answer}`)}
        onendinterview={(interview) => (interviewEvent = `end-interview · ${interview.question_id}`)}
        onopeninterview={(interview) => (interviewEvent = `open-interview · ${interview.question_id}`)}
      />
    </div>
  </div>
</Story>

<style>
  .story-frame {
    min-width: 720px;
    min-height: 100vh;
    padding: 20px;
    color: var(--foreground);
    background: color-mix(in oklch, var(--background) 96%, var(--foreground));
  }

  .event {
    box-sizing: border-box;
    width: 400px;
    min-height: 38px;
    padding: 9px 12px;
    margin: 0 auto 12px;
    font-family: var(--font-mono);
    font-size: 12px;
    line-height: 18px;
    background: var(--card);
    border: 1px solid var(--border);
    border-radius: calc(var(--radius) - 2px);
  }

  .pane-frame {
    width: 400px;
    height: 760px;
    margin: 0 auto;
    box-shadow: var(--shadow-lg);
  }
</style>
