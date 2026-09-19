<script module lang="ts">
  import { defineMeta } from "@storybook/addon-svelte-csf";
  import SmallStates from "./SmallStates.svelte";

  const { Story } = defineMeta({
    title: "Gimble/Run/SmallStates",
    component: SmallStates,
    parameters: { layout: "fullscreen" },
  });
</script>

<script lang="ts">
  let event = $state("All four states are fixture-free presentation states");
</script>

<Story name="Empty loading disconnected and no graph" asChild>
  <div class="story-page">
    <header>
      <div>
        <h1>Small states</h1>
        <p>Run-list and workspace fallbacks, shown together.</p>
      </div>
      <code aria-live="polite">{event}</code>
    </header>
    <main>
      <SmallStates state="empty" />
      <SmallStates state="loading" />
      <SmallStates state="disconnected" onreconnect={() => (event = "reconnect")} />
      <SmallStates
        state="no-graph"
        workflowName="implement"
        graphProblem="mismatch"
      />
    </main>
  </div>
</Story>

<style>
  .story-page {
    box-sizing: border-box;
    min-width: 900px;
    min-height: 100vh;
    padding: 28px;
    color: var(--foreground);
    background: color-mix(in oklch, var(--background) 96%, var(--foreground));
  }

  header {
    display: flex;
    align-items: flex-end;
    justify-content: space-between;
    gap: 24px;
    margin-bottom: 20px;
  }

  h1,
  p {
    margin: 0;
  }

  h1 {
    font-size: 24px;
    line-height: 32px;
  }

  p {
    color: var(--status-muted);
    font-size: 14px;
  }

  code {
    padding: 7px 10px;
    font-family: var(--font-mono);
    font-size: 12px;
    background: var(--card);
    border: 1px solid var(--map-line);
    border-radius: calc(var(--radius) - 2px);
  }

  main {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 20px;
  }
</style>
