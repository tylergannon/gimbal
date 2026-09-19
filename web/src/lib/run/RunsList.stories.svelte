<script module lang="ts">
  import { defineMeta } from "@storybook/addon-svelte-csf";
  import RunsList from "./RunsList.svelte";
  import { runsListFixture } from "./fixtures/index.js";

  const { Story } = defineMeta({
    title: "Gimble/Run/RunsList",
    component: RunsList,
    parameters: { layout: "fullscreen" },
  });
</script>

<script lang="ts">
  let event = $state("Choose a run, answer card, or filter");
</script>

<Story name="Recorded runs" asChild>
  <div class="story-page">
    <header class="project-bar">
      <strong>Gimble</strong>
      <span>/</span>
      <code>~/src/gimble</code>
      <span class="spacer"></span>
      <span class="connected"><i></i>Connected · local runtime</span>
    </header>
    <main>
      <RunsList
        runs={runsListFixture.runs}
        attention={runsListFixture.attention}
        summaries={runsListFixture.summaries}
        elapsed={runsListFixture.elapsed}
        now={1_800_000_760_000}
        onopenrun={(run) => (event = `open-run · ${run.id}`)}
      />
    </main>
    <p class="event" aria-live="polite">{event}</p>
  </div>
</Story>

<style>
  .story-page {
    min-width: 1100px;
    min-height: 100vh;
    color: var(--foreground);
    background: var(--background);
  }

  .project-bar {
    box-sizing: border-box;
    display: flex;
    height: 56px;
    align-items: center;
    gap: 12px;
    padding: 0 24px;
    background: var(--card);
    border-bottom: 1px solid var(--map-line);
  }

  .project-bar span,
  .project-bar code {
    color: var(--status-muted);
    font-size: 13px;
  }

  .spacer {
    flex: 1;
  }

  .connected {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 2px 8px;
    color: var(--foreground) !important;
    background: var(--secondary);
    border-radius: 999px;
  }

  .connected i {
    width: 6px;
    height: 6px;
    background: var(--foreground);
    border-radius: 999px;
  }

  main {
    display: flex;
    justify-content: center;
  }

  .event {
    position: fixed;
    right: 16px;
    bottom: 16px;
    padding: 7px 10px;
    margin: 0;
    color: var(--foreground);
    font-family: var(--font-mono);
    font-size: 12px;
    background: var(--card);
    border: 1px solid var(--border);
    border-radius: calc(var(--radius) - 2px);
    box-shadow: var(--shadow-md);
  }
</style>
