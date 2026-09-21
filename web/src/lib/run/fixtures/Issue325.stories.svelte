<script module lang="ts">
  import { defineMeta } from "@storybook/addon-svelte-csf";
  import { RunObservation } from "../../observation/index.js";
  import DetailPane from "../DetailPane.svelte";
  import { issue325Fixture, issue325Finished } from "./index.js";

  // The pane renders today's unchanged component against issue-325-shaped
  // data: a huge assignment, a glued call-site + context prompt, and a
  // running transcript full of escaped-newline tool payloads. This is meant
  // to look bad — that is the point of the fixture.
  const implementation = issue325Fixture.graph.body.find(
    (operation) => operation.kind === "promise_loop",
  );
  const coding = implementation?.body.find((operation) => operation.kind === "agent_call");
  const taskCheckCondition = implementation?.body.find(
    (operation) => operation.kind === "condition",
  );
  const taskCheck =
    taskCheckCondition?.kind === "condition"
      ? taskCheckCondition.branches[0]?.body.find(
          (operation) => operation.kind === "command" && operation.name === "task-check",
        )
      : undefined;
  if (!coding || coding.kind !== "agent_call" || !taskCheck || taskCheck.kind !== "command") {
    throw new Error("issue 325 fixture is missing the coding agent_call or task-check node");
  }
  const codingNode = coding;
  const taskCheckNode = taskCheck;
  const taskCheckScope = issue325Fixture.snapshot.scopes["implementation.1/task.1"];
  const taskCheckRuntime = issue325Fixture.snapshot.commands["implementation.1/task.1/task-check.1"];
  if (!taskCheckScope || !taskCheckRuntime) {
    throw new Error("issue 325 fixture is missing the task-check command row");
  }

  const runningObservation = new RunObservation(issue325Fixture.snapshot);
  const finishedObservation = new RunObservation(issue325Finished);

  const runningTaskScope = issue325Fixture.snapshot.scopes["implementation.1/task.1"];
  const runningCodingTurn = issue325Fixture.snapshot.turns["coding.1/turn.2"];
  const finishedTaskScope = issue325Finished.scopes["implementation.1/task.1"];
  const finishedCodingTurn = issue325Finished.turns["coding.1/turn.2"];
  if (!runningTaskScope || !runningCodingTurn || !finishedTaskScope || !finishedCodingTurn) {
    throw new Error("issue 325 story selections are incomplete");
  }

  const { Story } = defineMeta({
    title: "Gimble/Run/Issue 325 baseline",
    component: DetailPane,
    parameters: { layout: "fullscreen" },
  });
</script>

<Story name="Running coding turn" asChild>
  <div style="display: flex; justify-content: flex-end; height: 900px; background: var(--background);">
    <DetailPane
      snapshot={issue325Fixture.snapshot}
      observation={runningObservation}
      selection={{
        kind: "node",
        scope: runningTaskScope,
        operation: codingNode,
        runtime: runningCodingTurn,
      }}
      onsteer={async () => ({ ok: true, message: "Sent into the running turn." })}
    />
  </div>
</Story>

<Story name="Finished coding turn" asChild>
  <div style="display: flex; justify-content: flex-end; height: 900px; background: var(--background);">
    <DetailPane
      snapshot={issue325Finished}
      observation={finishedObservation}
      selection={{
        kind: "node",
        scope: finishedTaskScope,
        operation: codingNode,
        runtime: finishedCodingTurn,
      }}
    />
  </div>
</Story>

<Story name="Selected task scope" asChild>
  <div style="display: flex; justify-content: flex-end; height: 900px; background: var(--background);">
    <DetailPane
      snapshot={issue325Fixture.snapshot}
      observation={runningObservation}
      selection={{ kind: "sheet", scope: runningTaskScope }}
    />
  </div>
</Story>

<!-- A failed command (exit_code 1): stderr must render before stdout. -->
<Story name="Failed command" asChild>
  <div style="display: flex; justify-content: flex-end; height: 900px; background: var(--background);">
    <DetailPane
      snapshot={issue325Fixture.snapshot}
      observation={runningObservation}
      selection={{
        kind: "node",
        scope: taskCheckScope,
        operation: taskCheckNode,
        runtime: taskCheckRuntime,
      }}
    />
  </div>
</Story>
