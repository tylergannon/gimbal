<script lang="ts">
  import { tick } from 'svelte';
  import '@xyflow/svelte/dist/style.css';
  import '#lib/workflow-design/viewer.css';
  import { workflowGuide } from '../../routes/workflow.remote';
  import Guide from '#lib/workflow-design/Guide.svelte';
  import WorkflowCanvas from '#lib/workflow-design/WorkflowCanvas.svelte';
  import type { SourcePage } from '#lib/workflow-design/types';
  import type { ViewState } from '#lib/workflow-design/view-state';
  let { name }: { name: string } = $props();
  const query = $derived(workflowGuide(name));
  const source = $derived(query.current?.source_json ? JSON.parse(query.current.source_json) as SourcePage : undefined);
  let viewer = $state<WorkflowCanvas>();
  async function showStructure() { panel = 'diagram'; await tick(); if (source) await viewer?.show(source, view); }
  let panel = $state<'guide' | 'diagram'>('diagram');
  let view = $state<ViewState>({ open: [], details: [], types: false });
</script>
<section class="workflow-guide">
  <header><h1>{name}</h1><nav aria-label="Workflow guide views"><button aria-pressed={panel === 'guide'} onclick={() => panel = 'guide'}>Guide</button><button aria-pressed={panel === 'diagram'} onclick={showStructure}>Structure</button></nav></header>
  {#if query.error}<p class="message" role="alert">Could not read this workflow. <button onclick={() => query.refresh()}>Try again</button></p>
  {:else if !query.current}<p class="message" role="status">Loading guide…</p>
  {:else if !source}<p class="message">This workflow has no source guide recorded.</p>
  {:else}
    <div class="reading" hidden={panel !== 'guide'}><Guide page={source} /></div>
    {#if panel === 'diagram'}<div class="diagram workflow-viewer">
      {#if source.diagnostics?.length}<details class="coverage"><summary>Some source could not be represented ({source.diagnostics.length})</summary>{#each source.diagnostics as diagnostic, index (index)}<p>{diagnostic.message}</p>{/each}</details>{/if}
      <WorkflowCanvas bind:this={viewer} page={source} initialView={view} oncommit={async (next) => { view = next; await viewer?.show(source, next); }} />
    </div>{/if}
  {/if}
</section>
<style>
 .workflow-guide { height: 100%; display: flex; flex-direction: column; min-height: 0; }
 header { flex: none; display: flex; align-items: center; justify-content: space-between; gap: 16px; padding: 14px 24px; }
 h1 { margin: 0; font-size: 14px; font-weight: 500; }
 nav { display: flex; gap: 10px; } button { background: transparent; border: 0; color: var(--muted-foreground); font: inherit; font-size: 12px; cursor: pointer; padding: 4px; } button[aria-pressed='true'] { color: var(--foreground); }
 .reading { flex: 1; min-height: 0; overflow: auto; padding: 24px max(24px, calc((100% - 680px)/2)); }
 .diagram { flex: 1; min-height: 0; display: flex; flex-direction: column; }
 .reading[hidden] { display: none; }
 .coverage { padding: 8px 24px; color: var(--muted-foreground); font-size: 12px; }
 .message { padding: 16px 24px; color: var(--muted-foreground); font-size: 13px; }
</style>
