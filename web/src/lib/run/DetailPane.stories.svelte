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
  const commandSnapshot = structuredClone(implementInterviewFixture.snapshot);
  const commandScope = commandSnapshot.scopes["implementation.1/task.1"];
  const commandRuntime = commandSnapshot.commands["implementation.1/task.1/build.1"];
  const scopeSelection = implementInterviewFixture.snapshot.scopes["implementation.1/task.2"];
  const loopSelection = implementInterviewFixture.snapshot.scopes["implementation.1"];
  if (!commandScope || !commandRuntime || !scopeSelection || !loopSelection) {
    throw new Error("detail story selections are incomplete");
  }
  commandRuntime.stdout = "built web application\n… 72 KiB omitted …\nbuilt bin/gimble\n";
  commandRuntime.stderr = "warning: fixture uses a development source map\n";
  commandRuntime.stdout_file = "artifacts/commands/build.1/stdout.log";
  commandRuntime.stderr_file = "artifacts/commands/build.1/stderr.log";

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

<Story name="Selected command" asChild>
  <div style="display: flex; justify-content: flex-end; height: 900px; background: var(--background);">
    <DetailPane
      snapshot={commandSnapshot}
      selection={{
        kind: "node",
        scope: commandScope,
        operation: commandNode,
        runtime: commandRuntime,
      }}
    />
  </div>
</Story>

<Story name="Selected scope" asChild>
  <div style="display: flex; justify-content: flex-end; height: 900px; background: var(--background);">
    <DetailPane
      snapshot={implementInterviewFixture.snapshot}
      selection={{ kind: "sheet", scope: scopeSelection }}
    />
  </div>
</Story>

<Story name="Selected loop" asChild>
  <div style="display: flex; justify-content: flex-end; height: 900px; background: var(--background);">
    <DetailPane
      snapshot={implementInterviewFixture.snapshot}
      selection={{ kind: "sheet", scope: loopSelection }}
      onloop={async () => ({ ok: true, message: "Waiting for the planner’s next decision." })}
    />
  </div>
</Story>
