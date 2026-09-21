<script module lang="ts">
  import { defineMeta } from "@storybook/addon-svelte-csf";
  import SmallStates from "./SmallStates.svelte";

  const { Story } = defineMeta({
    title: "Gimble/Run/SmallStates",
    component: SmallStates,
    parameters: { layout: "fullscreen" },
  });
</script>

<Story name="Empty and no graph" asChild>
  <div class="story-page">
    <header>
      <div>
        <h1>Small states</h1>
        <p>Run-list and workspace fallbacks, shown together.</p>
      </div>
    </header>
    <main>
      <SmallStates state="empty" />
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

  header { margin-bottom: 20px; }

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

  main {
    display: grid;
    max-width: 680px;
    gap: 20px;
  }
</style>
