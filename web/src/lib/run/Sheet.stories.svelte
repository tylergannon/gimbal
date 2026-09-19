<script module lang="ts">
  import { defineMeta } from "@storybook/addon-svelte-csf";
  import Node, { type NodeOperation } from "./Node.svelte";
  import type { PipState } from "./Pip.svelte";
  import Sheet, { type FoldedStep } from "./Sheet.svelte";
  import type { ScopeRow } from "../observation/index.js";
  import { implementInterviewFixture } from "./fixtures/index.js";

  const { graph, snapshot } = implementInterviewFixture;
  const rootScope = snapshot.scopes[""];
  const implementationScope = snapshot.scopes["implementation.1"];
  const taskScopes = Object.values(snapshot.scopes)
    .filter((scope) => scope.key.startsWith("implementation.1/task."))
    .sort((left, right) => left.began - right.began);
  const latestTaskKey = taskScopes.at(-1)?.key ?? "";
  const backendScope = snapshot.scopes["reconnaissance.1/backend.1"];
  const frontendScope = snapshot.scopes["reconnaissance.1/frontend.1"];

  const implementation = graph.body.find((operation) => operation.kind === "promise_loop");
  const reconnaissance = graph.body.find((operation) => operation.kind === "group");
  const coding = implementation?.body.find((operation) => operation.kind === "agent_call");
  const taskCheck = implementation?.body
    .find((operation) => operation.kind === "condition")
    ?.branches.flatMap((branch) => branch.body)
    .find((operation) => operation.kind === "command");
  const bodyCommands = implementation?.body.filter((operation) => operation.kind === "command");
  const validator = implementation?.body.filter((operation) => operation.kind === "agent_call")[1];
  const backendAgent = reconnaissance?.children[0]?.body.find(
    (operation) => operation.kind === "agent_call",
  );
  const frontendAgent = reconnaissance?.children[1]?.body.find(
    (operation) => operation.kind === "agent_call",
  );

  if (
    !rootScope ||
    !implementationScope ||
    taskScopes.length !== 3 ||
    !backendScope ||
    !frontendScope ||
    !coding ||
    !taskCheck ||
    !bodyCommands ||
    bodyCommands.length !== 3 ||
    !validator ||
    !backendAgent ||
    !frontendAgent
  ) {
    throw new Error("implement-interview specimen is incomplete");
  }

  const backendOperation = backendAgent as NodeOperation;
  const frontendOperation = frontendAgent as NodeOperation;
  const taskOperations: NodeOperation[] = [coding, taskCheck, ...bodyCommands, validator];
  const observedAt = Math.max(
    snapshot.run.started,
    ...Object.values(snapshot.scopes).map((scope) => scope.ended || scope.began),
    ...Object.values(snapshot.turns).map((turn) => turn.ended || turn.started + turn.duration),
    ...Object.values(snapshot.commands ?? {}).map(
      (command) => command.ended || command.started + command.duration,
    ),
  );

  function stateFor(scope: ScopeRow, operation: NodeOperation): PipState {
    if (operation.kind === "command") {
      const command = Object.values(snapshot.commands ?? {}).find(
        (row) => row.scope === scope.key && row.name === operation.name,
      );
      if (!command) return "not-yet";
      if (command.interrupted) return "ended";
      return command.error || command.exit_code !== 0 ? "failed" : "ended";
    }

    if (operation.kind === "interview") {
      const interview = Object.values(snapshot.interviews).find(
        (row) => row.scope === scope.key && row.name === operation.name,
      );
      if (!interview) return "not-yet";
      return interview.status === "pending" ? "waiting" : "ended";
    }

    const sessions = Object.values(snapshot.sessions).filter(
      (session) => session.name === operation.session,
    );
    const turn = Object.values(snapshot.turns).find(
      (row) => row.scope === scope.key && sessions.some((session) => session.id === row.session),
    );
    if (!turn) return "not-yet";
    if (turn.interrupted) return "ended";
    if (turn.error) return "failed";
    return turn.ended === 0 ? "running" : "ended";
  }

  function stepsFor(scope: ScopeRow): FoldedStep[] {
    return taskOperations.map((operation) => ({ operation, state: stateFor(scope, operation) }));
  }

  function elapsedFor(scope: ScopeRow) {
    const milliseconds = Math.max(0, (scope.ended || observedAt) - scope.began);
    const minutes = Math.floor(milliseconds / 60_000);
    const seconds = Math.floor(milliseconds / 1_000) % 60;
    return `${minutes}m ${seconds.toString().padStart(2, "0")}s`;
  }

  function metaFor(scope: ScopeRow, operation: NodeOperation) {
    if (operation.kind === "command") {
      const command = Object.values(snapshot.commands ?? {}).find(
        (row) => row.scope === scope.key && row.name === operation.name,
      );
      return command ? `exit ${command.exit_code}` : "not started";
    }
    const turn = Object.values(snapshot.turns).find(
      (row) =>
        row.scope === scope.key &&
        Object.values(snapshot.sessions).some(
          (session) => session.name === operation.session && session.id === row.session,
        ),
    );
    return turn ? turn.id.slice(turn.id.lastIndexOf("/") + 1).replace(".", " ") : "not started";
  }

  const { Story } = defineMeta({
    title: "Gimble/Run/Sheet",
    component: Sheet,
    parameters: { layout: "centered" },
  });
</script>

<script lang="ts">
  let selectedTaskKey = $state(latestTaskKey);
  let selectedMessage = $state(latestTaskKey);
  let implementationFolded = $state(false);
  let pathFolded = $state(false);
  let foldedTaskKey = $state(taskScopes[1].key);
  let foldedDemo = $state(true);
  let openSibling = $state(backendScope.key);
  let siblingMessage = $state(`Open: ${backendScope.key}`);

  const selectedTask = $derived(
    taskScopes.find((scope) => scope.key === selectedTaskKey) ?? taskScopes[2],
  );
  const foldedTask = $derived(
    taskScopes.find((scope) => scope.key === foldedTaskKey) ?? taskScopes[1],
  );

  function selectTask(scope: ScopeRow) {
    selectedTaskKey = scope.key;
    selectedMessage = `Selected ${scope.key}`;
  }

  function openImplementation() {
    implementationFolded = false;
    selectedTaskKey = latestTaskKey;
    selectedMessage = `Opened ${implementationScope.key}; showing ${latestTaskKey}`;
  }

  function open(scope: ScopeRow) {
    openSibling = scope.key;
    siblingMessage = `Opened ${scope.key}; folded its sibling`;
  }
</script>

<Story name="Fixture-backed selected path" asChild>
  <div class="story-frame">
    <p class="event" aria-live="polite">{selectedMessage}</p>
    <Sheet scope={rootScope} root folded>
      <Sheet
        scope={implementationScope}
        kind="loop"
        selectionPath
        elapsed={elapsedFor(implementationScope)}
        steps={stepsFor(selectedTask)}
        folded={implementationFolded}
        onselect={(scope) => (selectedMessage = `Selected ${scope.key}`)}
        onopen={openImplementation}
        onfold={() => (implementationFolded = true)}
      >
        <Sheet
          scope={taskScopes[0]}
          instances={taskScopes}
          depth={1}
          selected
          contextTotal={8}
          elapsed={elapsedFor(selectedTask)}
          steps={stepsFor(selectedTask)}
          folded={pathFolded}
          onselect={(scope) => (selectedMessage = `Selected ${scope.key}`)}
          oninstancechange={selectTask}
          onopen={() => (pathFolded = false)}
          onfold={() => (pathFolded = true)}
        >
          <div class="steps">
            {#each stepsFor(selectedTask) as step}
              <Node
                operation={step.operation}
                state={step.state}
                meta={metaFor(selectedTask, step.operation)}
                small={step.operation.kind === "command"}
                selected={step.operation.kind === "agent_call" && step.operation.session === "coding"}
                onselect={(operation) =>
                  (selectedMessage = `Selected ${
                    operation.kind === "agent_call" ? operation.session : operation.name
                  } in ${selectedTask.key}`)}
              />
            {/each}
          </div>
        </Sheet>
      </Sheet>
    </Sheet>
  </div>
</Story>

<Story name="Folded repeated scope" asChild>
  <div class="story-frame compact">
    <p class="event" aria-live="polite">
      {foldedDemo ? `Folded ${foldedTask.key}` : `Opened ${foldedTask.key}`}
    </p>
    <Sheet
      scope={foldedTask}
      instances={taskScopes}
      selectedInstance={foldedTask.key}
      selected
      contextTotal={8}
      elapsed={elapsedFor(foldedTask)}
      steps={stepsFor(foldedTask)}
      folded={foldedDemo}
      oninstancechange={(scope) => (foldedTaskKey = scope.key)}
      onopen={() => (foldedDemo = false)}
      onfold={() => (foldedDemo = true)}
    >
      <div class="steps">
        {#each stepsFor(foldedTask) as step}
          <Node
            operation={step.operation}
            state={step.state}
            meta={metaFor(foldedTask, step.operation)}
            small={step.operation.kind === "command"}
          />
        {/each}
      </div>
    </Sheet>
  </div>
</Story>

<Story name="Sibling folding and permanent root" asChild>
  <div class="story-frame sibling-story">
    <p class="event" aria-live="polite">{siblingMessage}</p>
    <Sheet scope={rootScope} root folded>
      <div class="siblings">
        <Sheet
          scope={backendScope}
          kind="scope"
          folded={openSibling !== backendScope.key}
          elapsed={elapsedFor(backendScope)}
          steps={[{ operation: backendOperation, state: "ended" }]}
          onopen={open}
          onfold={() => open(frontendScope)}
        >
          <Node operation={backendOperation} state="ended" />
        </Sheet>
        <Sheet
          scope={frontendScope}
          kind="scope"
          folded={openSibling !== frontendScope.key}
          elapsed={elapsedFor(frontendScope)}
          steps={[{ operation: frontendOperation, state: "ended" }]}
          onopen={open}
          onfold={() => open(backendScope)}
        >
          <Node operation={frontendOperation} state="ended" />
        </Sheet>
      </div>
    </Sheet>
  </div>
</Story>

<style>
  .story-frame {
    width: min(720px, calc(100vw - 64px));
    box-sizing: border-box;
    padding: 28px;
    color: var(--foreground);
    background: var(--background);
  }

  .story-frame.compact {
    width: min(640px, calc(100vw - 64px));
    padding-top: 36px;
  }

  .event {
    margin: 0 0 26px;
    color: var(--status-muted);
    font-size: 13px;
    line-height: 18px;
  }

  .steps {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .siblings {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 16px;
    padding-top: 13px;
  }

  .sibling-story {
    width: min(780px, calc(100vw - 64px));
  }
</style>
