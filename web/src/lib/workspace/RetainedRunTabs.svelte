<script lang="ts">
  // A retained run tab beside a draft, hosted as ProjectWorkspace hosts them, for browser tests.
  import WorkspaceShell from '#lib/shell/WorkspaceShell.svelte';
  import type { WorkspaceTab } from '#lib/shell/types';
  import type { RunSnapshot } from '#lib/observation';
  import RunPanel from './RunPanel.svelte';
  // The desktop selects and closes tabs from its menu, outside the page, so a test passes them.
  let { project, run, runs, selected = 'run', shown = ['run', 'draft'] }: {
    project: string; run: { snapshot: RunSnapshot; graph: string; source?: string }; runs?: Record<string, { snapshot: RunSnapshot; graph: string; source?: string }>; selected?: string; shown?: string[];
  } = $props();
  const byID = $derived(runs ?? { run });
  const tabs = $derived(shown.map((id): WorkspaceTab => byID[id] ? { id, title: byID[id].snapshot.run.name, kind: 'run' } : { id, title: 'New session', kind: 'chat' }));
  let active = $derived(selected);
</script>

<WorkspaceShell {project} items={[]} {tabs} {active} onselect={id => active = id} onclose={() => {}} onopen={id => active = id}>
  {#snippet content(tab)}
    {#if tab.kind === 'run'}<RunPanel {project} id={byID[tab.id].snapshot.run.id} initial={byID[tab.id]} active={active === tab.id} />
    {:else}<textarea aria-label="Draft"></textarea>{/if}
  {/snippet}
</WorkspaceShell>
