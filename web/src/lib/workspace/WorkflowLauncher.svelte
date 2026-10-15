<script lang="ts">
  import * as Popover from '#lib/components/ui/popover';
  import { workflowCatalog } from '../../routes/workflow.remote';
  import WorkflowRun from '#lib/workflow-design/WorkflowRun.svelte';
  let { project, active = true, onopen, onopenguide }: { project: string; active?: boolean; onopen: (path: string) => void; onopenguide: (name: string) => void } = $props();
  const catalog = $derived(workflowCatalog(project));
  let open = $state(false);
  let selected = $state('');
  $effect(() => { if (!active) open = false; });
  const workflow = $derived(catalog.current?.workflows.find(item => item.name === selected) ?? catalog.current?.workflows[0]);
</script>
<Popover.Root bind:open>
  <Popover.Trigger class="launch-trigger">Run a workflow</Popover.Trigger>
  {#if active && open}
  <Popover.Content align="start" class="workflow-popup w-[390px] max-h-[min(80vh,650px)] overflow-auto p-5">
    <h2>Run a workflow</h2>
    {#if catalog.error}<p role="alert">Could not load workflows. <button onclick={() => catalog.refresh()}>Try again</button></p>
    {:else if !catalog.current}<p role="status">Loading workflows…</p>
    {:else if !workflow}<p>No workflows are installed in this runtime.</p>
    {:else}
      <label class="workflow-choice">Workflow<select bind:value={() => workflow.name, (name) => selected = name}>{#each catalog.current.workflows as item (item.name)}<option value={item.name}>{item.name}</option>{/each}</select></label>
      {#key workflow.name}<WorkflowRun {workflow} project={catalog.current.project} onguide={() => { open = false; onopenguide(workflow.name); }} onstarted={(accepted) => { open = false; onopen(`/projects/${accepted.project_id}/runs/${encodeURIComponent(accepted.run_id)}`); }} />{/key}
    {/if}
  </Popover.Content>
  {/if}
</Popover.Root>
<style>
 :global(.launch-trigger) { padding: 6px 10px; color: var(--foreground); background: var(--background); border: 1px solid var(--border); border-radius: 7px; cursor: pointer; font-size: 12px; }
 :global(.workflow-popup) h2 { margin: 0 0 14px; font-size: 15px; font-weight: 600; }
 .workflow-choice { display: grid; gap: 6px; margin-bottom: 12px; font-size: 12px; }
 select { padding: 7px 9px; border: 1px solid var(--border); border-radius: 7px; color: var(--foreground); background: var(--background); font: inherit; }
</style>
