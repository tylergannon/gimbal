<script module lang="ts">
  import { defineMeta } from "@storybook/addon-svelte-csf";
  import Node, { type NodeOperation } from "./Node.svelte";
  import type { PipState } from "./Pip.svelte";
  import { implementInterviewGraph, planTripGraph } from "./fixtures/index.js";

  const implementation = implementInterviewGraph.body.find(
    (operation) => operation.kind === "promise_loop",
  );
  const coding = implementation?.body.find((operation) => operation.kind === "agent_call");
  const command = implementation?.body.find((operation) => operation.kind === "command");
  const research = planTripGraph.body.find((operation) => operation.kind === "group");
  const interview = research?.children[0]?.body.find(
    (operation) => operation.kind === "interview",
  );

  if (!coding || !command || !interview) throw new Error("specimen graph steps are missing");

  const operations: Array<{ operation: NodeOperation; meta: string }> = [
    { operation: coding, meta: "turn 3" },
    { operation: command, meta: "exit 0" },
    { operation: interview, meta: "question 2" },
  ];
  const states: PipState[] = ["ended", "running", "failed", "waiting", "not-yet"];

  function operationName(operation: NodeOperation) {
    return operation.kind === "agent_call" ? operation.session : operation.name;
  }

  const { Story } = defineMeta({
    title: "Gimbal/Run/Node",
    component: Node,
    parameters: { layout: "centered" },
  });
</script>

<script lang="ts">
  let selected = $state(operationName(operations[0].operation));
</script>

<Story name="Every kind and state" asChild>
  <div class="catalog">
    {#each operations as item}
      <section>
        <h2>{item.operation.kind.replace("_", " ")}</h2>
        <div class="state-grid">
          {#each states as state}
            <div>
              <span>{state}</span>
              <div class="selection-pair">
                <Node operation={item.operation} {state} meta={item.meta} />
                <Node operation={item.operation} {state} meta={item.meta} selected />
              </div>
            </div>
          {/each}
        </div>
      </section>
    {/each}
  </div>
</Story>

<Story name="Small variants" asChild>
  <div class="catalog">
    {#each operations as item}
      <section>
        <h2>{item.operation.kind.replace("_", " ")}</h2>
        <div class="state-grid">
          {#each states as state}
            <div>
              <span>{state}</span>
              <div class="selection-pair">
                <Node operation={item.operation} {state} meta={item.meta} small />
                <Node operation={item.operation} {state} meta={item.meta} small selected />
              </div>
            </div>
          {/each}
        </div>
      </section>
    {/each}
  </div>
</Story>

<Story name="Selection event" asChild>
  <div class="selection-story">
    <p aria-live="polite">
      Selected: {selected}
    </p>
    <div class="selection-row">
      {#each operations as item}
        <Node
          operation={item.operation}
          state="ended"
          meta={item.meta}
          selected={selected === operationName(item.operation)}
          onselect={(operation) => (selected = operationName(operation))}
        />
      {/each}
    </div>
    <div class="selection-row small-row">
      {#each operations as item}
        <Node
          operation={item.operation}
          state="ended"
          meta={item.meta}
          selected={selected === operationName(item.operation)}
          onselect={(operation) => (selected = operationName(operation))}
          small
        />
      {/each}
    </div>
  </div>
</Story>

<style>
  .catalog {
    display: flex;
    width: min(1120px, calc(100vw - 64px));
    flex-direction: column;
    gap: 28px;
    padding: 24px;
    color: var(--foreground);
    background: var(--background);
  }

  section {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  h2,
  p {
    margin: 0;
    font-size: 13px;
    font-weight: 600;
    line-height: 18px;
  }

  .state-grid {
    display: grid;
    grid-template-columns: repeat(5, minmax(170px, 1fr));
    gap: 12px;
  }

  .state-grid > div {
    display: flex;
    min-width: 0;
    flex-direction: column;
    gap: 5px;
  }

  .state-grid > div > span {
    color: var(--status-muted);
    font-size: 13px;
    line-height: 18px;
  }

  .selection-pair {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .selection-story {
    display: flex;
    width: min(980px, calc(100vw - 64px));
    flex-direction: column;
    gap: 14px;
    padding: 24px;
    color: var(--foreground);
    background: var(--background);
  }

  .selection-row {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 12px;
  }

  .small-row {
    margin-top: 4px;
  }
</style>
