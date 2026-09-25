<script module lang="ts">
  import { defineMeta } from "@storybook/addon-svelte-csf";
  import Group from "./Group.svelte";
  import Node from "./Node.svelte";
  import { implementInterviewFixture, planTripFixture } from "./fixtures/index.js";
  import { buildMapLayout } from "./layout.js";

  const examples = [
    { title: "Implement interview · reconnaissance", fixture: implementInterviewFixture },
    { title: "Plan trip · research", fixture: planTripFixture },
  ].map(({ title, fixture }) => {
    const layout = buildMapLayout(fixture.graph, fixture.snapshot);
    const group = layout.groups[0];
    if (!group) throw new Error(`${title} group is missing`);
    return { title, layout, group };
  });

  function groupNodes(example: (typeof examples)[number]) {
    const { group, layout } = example;
    return layout.nodes.filter(
      (node) =>
        node.x >= group.x &&
        node.x + node.width <= group.x + group.width &&
        node.y >= group.y &&
        node.y + node.height <= group.y + group.height,
    );
  }

  function groupSheets(example: (typeof examples)[number]) {
    const { group, layout } = example;
    return layout.sheets.filter(
      (sheet) =>
        sheet.kind === "scope" &&
        sheet.x >= group.x &&
        sheet.y >= group.y &&
        sheet.y + sheet.height <= group.y + group.height,
    );
  }

  const { Story } = defineMeta({
    title: "Gimbal/Run/Group",
    component: Group,
    parameters: { layout: "centered" },
  });
</script>

<Story name="Fixture groups" asChild>
  <div class="catalog">
    {#each examples as example}
      <section>
        <h2>{example.title}</h2>
        <div class="stage" style:width={`${example.group.width}px`} style:height={`${example.group.height}px`}>
          {#each groupSheets(example) as sheet}
            <div
              class="branch-sheet"
              style:left={`${sheet.x - example.group.x}px`}
              style:top={`${sheet.y - example.group.y}px`}
              style:width={`${sheet.width}px`}
              style:height={`${sheet.height}px`}
            >
              <span>{sheet.scope.name}</span>
            </div>
          {/each}
          <Group
            group={example.group.group}
            width={example.group.width}
            height={example.group.height}
            forkY={example.group.forkY}
            joinY={example.group.joinY}
            branches={example.group.branches}
          />
          {#each groupNodes(example) as node}
            <div
              class="node"
              style:left={`${node.x - example.group.x}px`}
              style:top={`${node.y - example.group.y}px`}
              style:width={`${node.width}px`}
            >
              <Node operation={node.operation} state={node.state} meta={node.meta} selected={node.selected} />
            </div>
          {/each}
        </div>
      </section>
    {/each}
  </div>
</Story>

<style>
  .catalog {
    display: flex;
    flex-direction: column;
    gap: 32px;
    padding: 32px;
    color: var(--foreground);
    background: var(--background);
  }

  section {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  h2 {
    margin: 0;
    font-size: 15px;
    line-height: 20px;
  }

  .stage {
    position: relative;
    background: var(--map-paper-1);
    border: 1px solid var(--map-line-soft);
    border-radius: 10px;
  }

  .branch-sheet {
    position: absolute;
    box-sizing: border-box;
    background: var(--map-paper-2);
    border: 1px solid var(--map-line-soft);
    border-radius: 10px;
  }

  .branch-sheet > span {
    position: absolute;
    top: -12px;
    left: 12px;
    padding: 2px 9px;
    font-size: 13px;
    font-weight: 600;
    line-height: 18px;
    background: var(--map-paper-2);
    border: 1px solid var(--map-line-soft);
    border-radius: 7px;
  }

  .node {
    position: absolute;
    z-index: 2;
  }
</style>
