<script lang="ts">
  import { workflowCatalog } from '../../routes/workflow.remote';
  let { project, onopen }: { project: string; onopen: (name: string) => void } = $props();
  const catalog = $derived(workflowCatalog(project));
</script>

<section class="workflow-browser" aria-label="Workflows">
  <h1>Workflows</h1>
  <p>Explore a workflow’s structure before starting a run.</p>
  {#if catalog.error}
    <p role="alert">Could not load workflows. <button onclick={() => catalog.refresh()}>Try again</button></p>
  {:else if !catalog.current}
    <p role="status">Loading workflows…</p>
  {:else}
    <ul>
      {#each catalog.current.workflows as workflow (workflow.name)}
        <li><button onclick={() => onopen(workflow.name)}>
          <strong>{workflow.name}</strong>
          <span>{workflow.summary || workflow.help}</span>
          <span class="action">View structure →</span>
        </button></li>
      {:else}<li>No workflows are installed in this runtime.</li>{/each}
    </ul>
  {/if}
</section>

<style>
  .workflow-browser { height: 100%; overflow: auto; padding: 24px; box-sizing: border-box; }
  h1 { font-size: 20px; margin: 0 0 8px; font-weight: 600; }
  p { color: var(--muted-foreground); font-size: 13px; }
  ul { list-style: none; padding: 0; margin: 24px 0; max-width: 800px; }
  li { border-bottom: 1px solid var(--border); }
  li button { width: 100%; display: grid; gap: 6px; padding: 16px 12px; text-align: left; border: 0; color: var(--foreground); background: transparent; font: inherit; cursor: pointer; border-radius: 6px; }
  li button:hover { background: var(--accent); }
  button:focus-visible { outline: 2px solid var(--ring); outline-offset: 2px; }
  strong { font-size: 14px; font-weight: 500; }
  span { color: var(--muted-foreground); font-size: 13px; line-height: 1.5; }
  .action { color: var(--foreground); font-size: 12px; }
</style>
