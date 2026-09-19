<script module lang="ts">
  import { defineMeta } from "@storybook/addon-svelte-csf";
  import Lanes from "./Lanes.svelte";
  import { mismatchedHistoryFixture } from "./fixtures/index.js";

  const { Story } = defineMeta({
    title: "Gimble/Run/Lanes",
    component: Lanes,
    parameters: { layout: "fullscreen" },
  });
</script>

<script lang="ts">
  let event = $state("Select a recorded scope, turn, or command");
</script>

<Story name="Recorded run without matching graph" asChild>
  <div class="story-page">
    <header class="record-bar">
      <strong>Runs / implement-interview / <code>01M2QZ7P</code></strong>
      <span class="failed">Failed</span>
      <span>Ended · 1h 12m</span>
      <span class="spacer"></span>
      <span>Recorded run · nothing here is live</span>
    </header>
    <main>
      <Lanes
        snapshot={mismatchedHistoryFixture.snapshot}
        notice={mismatchedHistoryFixture.graphNotice}
        onselect={(selection) =>
          (event = `select-${selection.kind} · ${
            selection.kind === "scope" ? selection.row.key : selection.row.id
          }`)}
      />
    </main>
    <p class="event" aria-live="polite">{event}</p>
  </div>
</Story>

<style>
  .story-page {
    min-width: 960px;
    min-height: 100vh;
    color: var(--foreground);
    background: var(--background);
  }

  .record-bar {
    box-sizing: border-box;
    display: flex;
    height: 56px;
    align-items: center;
    gap: 10px;
    padding: 0 16px;
    font-size: 13px;
    background: var(--card);
    border-bottom: 1px solid var(--map-line);
  }

  .record-bar strong {
    font-size: 14px;
  }

  .record-bar span:not(.failed) {
    color: var(--status-muted);
  }

  .failed {
    padding: 2px 8px;
    color: var(--destructive);
    font-size: 12px;
    font-weight: 600;
    background: color-mix(in oklch, var(--destructive) 12%, transparent);
    border-radius: 999px;
  }

  .spacer {
    flex: 1;
  }

  main {
    width: min(1100px, calc(100% - 32px));
    margin: 0 auto;
  }

  .event {
    width: min(1068px, calc(100% - 64px));
    padding: 8px 12px;
    margin: 0 auto;
    font-family: var(--font-mono);
    font-size: 12px;
    background: var(--card);
    border: 1px solid var(--border);
    border-radius: calc(var(--radius) - 2px);
  }
</style>
