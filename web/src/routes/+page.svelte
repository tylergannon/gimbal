<script lang="ts">
  import { goto } from "$app/navigation";
  import type { InterviewRow, RunRow } from "#lib/observation/index.js";
  import RunsList from "#lib/run/RunsList.svelte";
  import SmallStates from "#lib/run/SmallStates.svelte";

  type RunItem = { run: RunRow; summary: string; elapsed: string };
  let { data }: { data: { items: RunItem[]; attention: InterviewRow[]; now: number } } = $props();

  const runs = $derived(data.items.map((item) => item.run));
  const summaries = $derived(Object.fromEntries(data.items.map((item) => [item.run.id, item.summary])));
  const elapsed = $derived(Object.fromEntries(data.items.map((item) => [item.run.id, item.elapsed])));

  function openRun(run: RunRow) {
    void goto(`/runs/${encodeURIComponent(run.id)}`);
  }
</script>

<svelte:head>
  <title>Runs — Gimble</title>
  <meta
    name="description"
    content="Every live and recorded agent workflow run in this Gimble project."
  />
</svelte:head>

<div class="runs-page">
  {#if runs.length === 0}
    <div class="empty-project"><SmallStates state="empty" /></div>
  {:else}
    <RunsList
      {runs}
      attention={data.attention}
      {summaries}
      {elapsed}
      now={data.now}
      onopenrun={openRun}
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
