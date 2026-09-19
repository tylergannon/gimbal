<script module lang="ts">
  import { defineMeta } from "@storybook/addon-svelte-csf";
  import { RunObservation } from "../observation/index.js";
  import DetailPane from "./DetailPane.svelte";
  import { implementInterviewFixture, planTripFixture } from "./fixtures/index.js";

  const implementation = implementInterviewFixture.graph.body.find(
    (operation) => operation.kind === "promise_loop",
  );
  const coding = implementation?.body.find((operation) => operation.kind === "agent_call");
  const command = implementation?.body.find((operation) => operation.kind === "command");
  const interviewGroup = planTripFixture.graph.body.find((operation) => operation.kind === "group");
  const interview = interviewGroup?.children[0]?.body.find(
    (operation) => operation.kind === "interview",
  );
  if (
    !coding ||
    coding.kind !== "agent_call" ||
    !command ||
    command.kind !== "command" ||
    !interview ||
    interview.kind !== "interview"
  ) {
    throw new Error("detail fixtures are incomplete");
  }
  const codingNode = coding;
  const commandNode = command;
  const interviewNode = interview;
  const implementationObservation = new RunObservation(implementInterviewFixture.snapshot);
  const planObservation = new RunObservation(planTripFixture.snapshot);

  const { Story } = defineMeta({
    title: "Gimble/Run/Detail pane",
    component: DetailPane,
    parameters: { layout: "fullscreen" },
  });
</script>

<Story name="Running agent call" asChild>
  <div style="display: flex; justify-content: flex-end; height: 900px; background: var(--background);">
    <DetailPane
      snapshot={implementInterviewFixture.snapshot}
      observation={implementationObservation}
      selection={{
        kind: "node",
        scope: implementInterviewFixture.snapshot.scopes["implementation.1/task.3"],
        operation: codingNode,
        runtime: implementInterviewFixture.snapshot.turns["coding.1/turn.3"],
      }}
      onsteer={async () => ({ ok: true, message: "Sent into the running turn." })}
    />
  </div>
</Story>

<Story name="Waiting interview" asChild>
  <div style="display: flex; justify-content: flex-end; height: 900px; background: var(--background);">
    <DetailPane
      snapshot={planTripFixture.snapshot}
      observation={planObservation}
      selection={{
        kind: "node",
        scope: planTripFixture.snapshot.scopes["research.1/lodging.1"],
        operation: interviewNode,
        runtime: Object.values(planTripFixture.snapshot.interviews).find(
          (row) => row.scope === "research.1/lodging.1" && row.status === "pending",
        ),
      }}
      onanswer={async () => ({ ok: true, message: "Answer accepted by the waiting interview." })}
    />
  </div>
</Story>

<Story name="Guarded step not observed" asChild>
  <div style="display: flex; justify-content: flex-end; height: 900px; background: var(--background);">
    <DetailPane
      snapshot={implementInterviewFixture.snapshot}
      observation={implementationObservation}
      selection={{
        kind: "node",
        scope: implementInterviewFixture.snapshot.scopes["implementation.1/task.3"],
        operation: commandNode,
      }}
    />
  </div>
</Story>
