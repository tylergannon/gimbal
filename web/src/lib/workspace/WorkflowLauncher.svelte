<script lang="ts">
  import { afterNavigate } from '$app/navigation';
  import * as Popover from '#lib/components/ui/popover';
  import { Popover as PopoverPrimitive } from 'bits-ui';
  import { workflowCatalog } from '../../routes/workflow.remote';
  import WorkflowRun from '#lib/workflow-design/WorkflowRun.svelte';
  let { project, active = true, onopen, onopenguide }: { project: string; active?: boolean; onopen: (path: string) => void; onopenguide: (name: string) => void } = $props();
  const catalog = $derived(workflowCatalog(project));
  let open = $state(false);
  let selected = $state('');
  afterNavigate(() => { open = false; });
  const workflow = $derived(catalog.current?.workflows.find(item => item.name === selected) ?? catalog.current?.workflows[0]);
</script>
<div class="launcher">
<Popover.Root bind:open>
  <Popover.Trigger class="launch-trigger">Run a workflow</Popover.Trigger>
  <PopoverPrimitive.ContentStatic forceMount hidden={!active || !open} inert={!active || !open} class="workflow-popup">
    <h2>Run a workflow</h2>
    {#if catalog.error}<p role="alert">Could not load workflows. <button onclick={() => catalog.refresh()}>Try again</button></p>
    {:else if !catalog.current}<p role="status">Loading workflows…</p>
    {:else if !workflow}<p>No workflows are installed in this runtime.</p>
    {:else}
      <label class="workflow-choice">Workflow<select bind:value={() => workflow.name, (name) => selected = name}>{#each catalog.current.workflows as item (item.name)}<option value={item.name}>{item.name}</option>{/each}</select></label>
      {#key workflow.name}<WorkflowRun {workflow} project={catalog.current.project} onguide={() => { open = false; onopenguide(workflow.name); }} onstarted={(accepted) => { open = false; onopen(`/projects/${accepted.project_id}/runs/${encodeURIComponent(accepted.run_id)}`); }} />{/key}
    {/if}
  </PopoverPrimitive.ContentStatic>
</Popover.Root>
</div>
<style>
 .launcher { position: relative; }
 :global(.workflow-popup) { position: absolute; top: calc(100% + 6px); left: 0; z-index: 50; width: min(390px, calc(100vw - 80px)); max-height: min(70vh, 650px); box-sizing: border-box; overflow: auto; padding: 20px; color: var(--popover-foreground); background: var(--popover); border: 1px solid var(--border); border-radius: 10px; box-shadow: 0 8px 24px #0002; }
 :global(.launch-trigger) { padding: 6px 10px; color: var(--foreground); background: var(--background); border: 1px solid var(--border); border-radius: 7px; cursor: pointer; font-size: 12px; }
 :global(.workflow-popup[hidden]) { display: none !important; }
 :global(.workflow-popup) h2 { margin: 0 0 14px; font-size: 15px; font-weight: 600; }
 .workflow-choice { display: grid; gap: 6px; margin-bottom: 12px; font-size: 12px; }
 select { padding: 7px 9px; border: 1px solid var(--border); border-radius: 7px; color: var(--foreground); background: var(--background); font: inherit; }
</style>
