<script module lang="ts">
  import { defineMeta } from "@storybook/addon-svelte-csf";
  import Loop from "./Loop.svelte";
  import Node from "./Node.svelte";
  import { implementInterviewFixture } from "./fixtures/index.js";
  import { buildMapLayout } from "./layout.js";

  const layout = buildMapLayout(implementInterviewFixture.graph, implementInterviewFixture.snapshot);
  const loop = layout.loops[0];
  if (!loop) throw new Error("implement-interview loop is missing");
  const nodes = layout.nodes.filter(
    (node) => node.x >= loop.x && node.y >= loop.y && node.y + node.height <= loop.y + loop.height,
  );
  const sheets = layout.sheets.filter(
    (sheet) => sheet.x >= loop.x && sheet.y >= loop.y && sheet.y + sheet.height <= loop.y + loop.height,
  );

  const { Story } = defineMeta({
    title: "Gimbal/Run/Loop",
    component: Loop,
    parameters: { layout: "centered" },
  });
</script>

<Story name="Fixture loop return" asChild>
  <div class="frame">
    <h2>Implement interview · implementation</h2>
    <div class="stage" style:width={`${loop.width}px`} style:height={`${loop.height}px`}>
      {#each sheets as sheet}
        <div
          class="sheet"
          class:task={sheet.scope.name === "task"}
          style:left={`${sheet.x - loop.x}px`}
          style:top={`${sheet.y - loop.y}px`}
          style:width={`${sheet.width}px`}
          style:height={`${sheet.height}px`}
        >
          <span>{sheet.scope.name === "task" ? "task 3 of 3" : sheet.scope.name}</span>
        </div>
      {/each}
      <Loop {...loop} />
      {#each nodes as node}
        <div
          class="node"
          style:left={`${node.x - loop.x}px`}
          style:top={`${node.y - loop.y}px`}
          style:width={`${node.width}px`}
        >
          <Node
            operation={node.operation}
            state={node.state}
            meta={node.meta}
            selected={node.selected}
            small={node.operation.kind === "command"}
          />
        </div>
      {/each}
    </div>
  </div>
</Story>

<style>
  .frame {
    display: flex;
    flex-direction: column;
    gap: 16px;
    padding: 32px;
    color: var(--foreground);
    background: var(--background);
  }

  h2 {
    margin: 0;
    font-size: 15px;
    line-height: 20px;
  }

  .stage {
    position: relative;
    box-sizing: border-box;
    background: var(--map-paper-1);
    border: 1px solid var(--map-line-soft);
    border-radius: 10px;
  }

  .sheet {
    position: absolute;
    box-sizing: border-box;
    background: var(--map-paper-1);
    border: 1px solid var(--map-line-soft);
    border-radius: 10px;
  }

  .sheet.task {
    background: var(--map-paper-2);
  }

  .sheet > span {
    position: absolute;
    top: -12px;
    left: 12px;
    padding: 2px 9px;
    font-size: 13px;
    font-weight: 600;
    line-height: 18px;
    background: inherit;
    border: 1px solid var(--map-line-soft);
    border-radius: 7px;
  }

  .node {
    position: absolute;
    z-index: 2;
  }
</style>
