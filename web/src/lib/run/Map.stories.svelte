<script module lang="ts">
  import { defineMeta } from "@storybook/addon-svelte-csf";
  import Map, { type MapSelection } from "./Map.svelte";
  import {
    implementInterviewFixture,
    planTripFixture,
    serviceOwnershipFixture,
  } from "./fixtures/index.js";

  const { Story } = defineMeta({
    title: "Gimble/Run/Map",
    component: Map,
    parameters: { layout: "fullscreen" },
  });
</script>

<script lang="ts">
  let implementSelection = $state("Select a sheet, step, watcher, or task instance");
  let planSelection = $state("Select a sheet or interview");
  let serviceSelection = $state("Select a service or ordered step");
  let implementSnapshot = $state.raw(implementInterviewFixture.snapshot);

  function selectionLabel(selection: MapSelection) {
    if (selection.kind === "sheet") return `Selected sheet ${selection.scope.key}`;
    if (selection.kind === "instance") return `Selected instance ${selection.scope.key}`;
    if (selection.kind === "watcher") {
      return `Selected watcher ${selection.supervisor.session} in ${selection.scope.key}`;
    }
    if (selection.kind === "service") {
      return `Selected service ${selection.service.name} in ${selection.scope.key || "root"}`;
    }
    const name =
      selection.operation.kind === "agent_call"
        ? selection.operation.session
        : selection.operation.name;
    return `Selected ${name} in ${selection.scope.key}`;
  }

  function finishCurrentCodingTurn() {
    const next = structuredClone(implementSnapshot);
    const turn = next.turns["coding.1/turn.3"];
    const scope = next.scopes["implementation.1/task.3"];
    if (!turn || !scope) return;
    turn.duration = 120_000;
    turn.ended = turn.started + turn.duration;
    scope.status = "ended";
    scope.ended = turn.ended;
    implementSnapshot = next;
  }
</script>

<Story name="Implement interview" asChild>
  <div class="story-frame">
    <div class="story-controls">
      <p aria-live="polite">{implementSelection}</p>
      <button type="button" onclick={finishCurrentCodingTurn}>Refresh coding facts</button>
    </div>
    <Map
      graph={implementInterviewFixture.graph}
      snapshot={implementSnapshot}
      onselect={(selection) => (implementSelection = selectionLabel(selection))}
    />
  </div>
</Story>

<Story name="Scope-owned services" asChild>
  <div class="story-frame">
    <p class="selection" aria-live="polite">{serviceSelection}</p>
    <Map
      graph={serviceOwnershipFixture.graph}
      snapshot={serviceOwnershipFixture.snapshot}
      onselect={(selection) => (serviceSelection = selectionLabel(selection))}
    />
  </div>
</Story>

<Story name="Plan trip" asChild>
  <div class="story-frame">
    <p class="selection" aria-live="polite">{planSelection}</p>
    <Map
      graph={planTripFixture.graph}
      snapshot={planTripFixture.snapshot}
      onselect={(selection) => (planSelection = selectionLabel(selection))}
    />
  </div>
</Story>

<style>
  .story-frame {
    min-height: 100vh;
    color: var(--foreground);
    background: var(--background);
  }

  .story-controls,
  .selection {
    display: flex;
    min-height: 44px;
    box-sizing: border-box;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
    margin: 0;
    padding: 8px 16px;
    font-size: 13px;
    border-bottom: 1px solid var(--border);
  }

  .story-controls p {
    margin: 0;
  }

  .story-controls button {
    padding: 5px 9px;
    color: var(--foreground);
    font: inherit;
    cursor: pointer;
    background: var(--background);
    border: 1px solid var(--border);
    border-radius: 6px;
  }
</style>
