<script module lang="ts">
  import { defineMeta } from "@storybook/addon-svelte-csf";
  import Node from "./Node.svelte";
  import Watcher from "./Watcher.svelte";
  import { implementInterviewFixture } from "./fixtures/index.js";
  import { buildMapLayout } from "./layout.js";

  const layout = buildMapLayout(implementInterviewFixture.graph, implementInterviewFixture.snapshot);
  function required<T>(value: T | undefined, message: string): T {
    if (!value) throw new Error(message);
    return value;
  }

  const watched = required(
    layout.nodes.find(
      (node) => node.operation.kind === "agent_call" && node.operation.session === "coding",
    ),
    "implement-interview coding node is missing",
  );
  if (layout.watchers.length !== 2) throw new Error("implement-interview coding watchers are missing");
  const watcherWidth = layout.watchers[0].width;
  const bracketX = layout.watchers[0].x + watcherWidth;
  const targetX = watched.x - 2;

  const { Story } = defineMeta({
    title: "Gimbal/Run/Watcher",
    component: Watcher,
    parameters: { layout: "centered" },
  });
</script>

<Story name="Coding watchers" asChild>
  <div class="frame">
    <span class="caption">watching</span>
    <svg width="740" height="144" viewBox="0 0 740 144" aria-hidden="true">
      <path
        d={`M${bracketX},42 H${bracketX + 8} V90 H${bracketX} M${bracketX + 8},66 H${targetX}`}
      />
    </svg>
    {#each layout.watchers as watcher, index}
      <div
        class="watcher"
        style:top={`${24 + index * 48}px`}
        style:width={`${watcher.width}px`}
      >
        <Watcher supervisor={watcher.supervisor} state={watcher.state} />
      </div>
    {/each}
    <div class="node">
      <Node
        operation={watched.operation}
        state={watched.state}
        meta={watched.meta}
        selected={watched.selected}
      />
    </div>
  </div>
</Story>

<style>
  .frame {
    position: relative;
    width: 740px;
    height: 144px;
    color: var(--foreground);
    background: var(--background);
  }

  .caption {
    position: absolute;
    top: 0;
    left: 16px;
    color: var(--status-muted);
    font-size: 13px;
    font-weight: 600;
    line-height: 18px;
  }

  svg {
    position: absolute;
    inset: 0;
    overflow: visible;
  }

  path {
    fill: none;
    stroke: var(--status-muted);
    stroke-width: 1.5;
    stroke-dasharray: 2 3;
  }

  .watcher {
    position: absolute;
    left: 16px;
    height: 36px;
  }

  .node {
    position: absolute;
    top: 44px;
    left: 380px;
    width: 360px;
  }
</style>
