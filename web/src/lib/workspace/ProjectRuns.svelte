<script lang="ts">
  import RunsList, { type RunCardItem } from '#lib/run/RunsList.svelte';
  import WorkflowLauncher from './WorkflowLauncher.svelte';
  import type { InteractiveRow, RunRow } from '#lib/observation';
  let { project, active = true, data, error, onretry, onopen, onopenguide }: {
    project: string; active?: boolean; data?: { items: RunCardItem[]; attention: InteractiveRow[]; now: number };
    error?: unknown; onretry: () => void; onopen: (path: string) => void; onopenguide: (name: string) => void;
  } = $props();
  function open(run: RunRow) { onopen(`/projects/${project}/runs/${encodeURIComponent(run.id)}`); }
</script>
<section class="project-runs" aria-label="Project runs">
  <div class="launch"><WorkflowLauncher {project} {active} {onopen} {onopenguide} /></div>
  {#if error}
    <p role="alert">Could not read runs. <button onclick={onretry}>Retry</button></p>
  {:else if !data}<p role="status">Loading runs…</p>
  {:else if !data.items.length}<div class="empty"><h1>Empty project</h1><h2>No runs yet</h2><p>Choose a workflow to see its purpose and start a run in this project.</p><p>Use Run a workflow above to get started.</p></div>
  {:else}<RunsList {active} items={data.items} attention={data.attention} now={data.now} onopenrun={open} />{/if}
</section>
<style>
 .project-runs { height: 100%; overflow: auto; padding: 20px 24px; }
 .launch { display: inline-block; margin-bottom: 16px; font-size: 13px; color: var(--foreground); }
 .empty { max-width: 560px; margin: 32px auto; }
 .empty h1 { font-size: 20px; }
 .empty p { color: var(--muted-foreground); }
</style>
