<script lang="ts">
  import { goto, invalidateAll } from "$app/navigation";
	import { page } from "$app/state";
  import { onMount } from "svelte";
  import type { InterviewRow, RunRow } from "#lib/observation/index.js";
  import RunsList, { type RunCardItem } from "#lib/run/RunsList.svelte";
  import SmallStates from "#lib/run/SmallStates.svelte";

  let { data }: { data: { items: RunCardItem[]; attention: InterviewRow[]; now: number } } =
    $props();

  onMount(() => {
    const refresh = window.setInterval(() => void invalidateAll(), 2_000);
    return () => window.clearInterval(refresh);
  });

  function openRun(run: RunRow) {
	void goto(`/projects/${page.params.project}/runs/${encodeURIComponent(run.id)}`);
  }
</script>

<svelte:head>
  <title>Runs — Gimbal</title>
  <meta
    name="description"
    content="Every live and recorded agent workflow run in this Gimbal project."
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
