<script module lang="ts">
  import { defineMeta } from "@storybook/addon-svelte-csf";
  import DetailPane from "./DetailPane.svelte";
  import Map, { type MapSelection } from "./Map.svelte";
  import Workspace from "./Workspace.svelte";
  import { implementInterviewFixture } from "./fixtures/index.js";

  const implementation = implementInterviewFixture.graph.body.find(
    (operation) => operation.kind === "promise_loop",
  );
  const coding = implementation?.body.find((operation) => operation.kind === "agent_call");
  if (!coding || coding.kind !== "agent_call") throw new Error("workspace fixture is incomplete");
  const codingNode = coding;
  const nodeSelection: MapSelection = {
    kind: "node",
    scope: implementInterviewFixture.snapshot.scopes["implementation.1/task.3"],
    operation: codingNode,
    runtime: implementInterviewFixture.snapshot.turns["coding.1/turn.3"],
  };

  const { Story } = defineMeta({
    title: "Gimble/Run/Workspace resize",
    component: Workspace,
    parameters: { layout: "fullscreen" },
  });
</script>

<Story name="Default" asChild>
  <div class="story-frame">
    <div class="story-topbar">Runs › implement · a stand-in for the real Topbar</div>
    <Workspace open>
      {#snippet map()}
        <Map graph={implementInterviewFixture.graph} snapshot={implementInterviewFixture.snapshot} selected={nodeSelection} />
      {/snippet}
      {#snippet pane({ width, maximized, onmaximize })}
        <DetailPane
          snapshot={implementInterviewFixture.snapshot}
          selection={nodeSelection}
          {width}
          {maximized}
          {onmaximize}
        />
      {/snippet}
    </Workspace>
  </div>
</Story>

<Story name="Maximized" asChild>
  <div class="story-frame">
    <div class="story-topbar">Runs › implement · a stand-in for the real Topbar</div>
    <Workspace open initialMaximized={true}>
      {#snippet map()}
        <Map graph={implementInterviewFixture.graph} snapshot={implementInterviewFixture.snapshot} selected={nodeSelection} />
      {/snippet}
      {#snippet pane({ width, maximized, onmaximize })}
        <DetailPane
          snapshot={implementInterviewFixture.snapshot}
          selection={nodeSelection}
          {width}
          {maximized}
          {onmaximize}
        />
      {/snippet}
    </Workspace>
  </div>
</Story>

<Story name="Narrow overlay" asChild>
  <div class="story-frame">
    <div class="story-topbar">Runs › implement · a stand-in for the real Topbar</div>
    <Workspace open>
      {#snippet map()}
        <Map graph={implementInterviewFixture.graph} snapshot={implementInterviewFixture.snapshot} selected={nodeSelection} />
      {/snippet}
      {#snippet pane({ width, maximized, onmaximize })}
        <DetailPane
          snapshot={implementInterviewFixture.snapshot}
          selection={nodeSelection}
          {width}
          {maximized}
          {onmaximize}
        />
      {/snippet}
    </Workspace>
  </div>
</Story>

<style>
  .story-frame {
    display: flex;
    height: 100vh;
    flex-direction: column;
    overflow: hidden;
    color: var(--foreground);
    background: var(--background);
  }

  .story-topbar {
    display: flex;
    height: 56px;
    box-sizing: border-box;
    flex-shrink: 0;
    align-items: center;
    padding: 0 16px;
    color: var(--status-muted);
    font-size: 13px;
    background: var(--card);
    border-bottom: 1px solid var(--map-line);
  }
</style>
