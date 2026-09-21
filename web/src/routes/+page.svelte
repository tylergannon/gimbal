<script lang="ts">
  import type { InterviewRow } from "#lib/observation/index.svelte.js";
  import RunsList, { type RunCardItem } from "#lib/run/RunsList.svelte";
  import SmallStates from "#lib/run/SmallStates.svelte";
  import { watchRuns } from "./runs.remote.js";

  const runs = watchRuns();
  const data = $derived(
    (await runs) as { items: RunCardItem[]; attention: InterviewRow[]; now: number },
  );
</script>

<svelte:head>
  <title>Runs — Gimble</title>
  <meta
    name="description"
    content="Every live and recorded agent workflow run in this Gimble project."
  />
</svelte:head>

<div class="runs-page">
  {#if data.items.length === 0}
    <div class="empty-project"><SmallStates state="empty" /></div>
  {:else}
    <RunsList
      items={data.items}
      attention={data.attention}
      now={data.now}
    />
  {/if}
</div>

<style>
  .runs-page {
    display: flex;
    min-width: 0;
    justify-content: center;
    padding: 0 32px;
  }

  .empty-project {
    box-sizing: border-box;
    width: min(1120px, 100%);
    padding: 36px 0 24px;
  }
</style>
