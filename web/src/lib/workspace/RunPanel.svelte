<script lang="ts">
  import { navigateOnUnauthorized } from "#lib/auth/unauthorized";
  import { viewRun } from "../runs.remote";
  import RunView from "#lib/run/RunView.svelte";
  import { untrack } from "svelte";
  import type { RunSnapshot } from "#lib/observation";
  let { project, id, initial, active }: {
    project: string; id: string; initial?: { snapshot: RunSnapshot; graph: string };
    active: boolean;
  } = $props();
  let preload = $state.raw(untrack(() => initial?.snapshot?.run.id === id ? initial : undefined));
  const query = $derived(viewRun({ project, id }));
  const data = $derived(preload ?? query.current);
  $effect(() => { if (query.error) void query.catch(navigateOnUnauthorized); });
  async function refresh() { await query.refresh(); preload = query.current; }
</script>

{#if data}
  <RunView {project} {data} {active} onrefresh={refresh} />
{:else if query.error}
  <p class="notice" role="alert">Could not load this run. <button onclick={refresh}>Retry</button></p>
{:else}
  <p class="notice" role="status">Loading run…</p>
{/if}

<style>.notice { padding: 24px; }</style>
