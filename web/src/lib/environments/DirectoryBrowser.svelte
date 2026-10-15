<script lang="ts">
  import { untrack } from 'svelte';
  import { listEnvironmentDirectories } from '#lib/environments.remote';
  import { errorMessage } from '#lib/errors';
  let { project, machine, path = '/', onchoose }: { project: string; machine: string; path?: string; onchoose: (path: string) => void } = $props();
  let directory = $state(untrack(() => path.startsWith('/') ? path : '/'));
  const listing = $derived(listEnvironmentDirectories({ project, machine, directory }));
  const parent = $derived(directory.replace(/\/+$/, '').replace(/\/[^/]*$/, '') || '/');
  const segments = $derived(directory.split('/').filter(Boolean));
  function visit(path: string) { directory = path; onchoose(path); }
</script>

<section class="browser" aria-label="Directory browser">
  <nav aria-label="Directory path"><button type="button" onclick={() => visit('/')}>/</button>{#each segments as segment, index (`${index}-${segment}`)}<button type="button" onclick={() => visit('/' + segments.slice(0, index + 1).join('/'))}>{segment}</button>{/each}</nav>
  <div class="columns"><div class="parent"><button type="button" onclick={() => visit(parent)}>↑ Parent directory</button><p>{directory}</p></div><div class="children">
    {#if listing.current}<ul>{#each listing.current.entries as entry (entry.path)}<li><button type="button" onclick={() => visit(entry.path)}><span aria-hidden="true">▱</span>{entry.name}<span aria-hidden="true">›</span></button></li>{:else}<li class="empty">No subdirectories</li>{/each}</ul>
    {:else if listing.error}<p role="alert">{errorMessage(listing.error)} <button type="button" onclick={() => listing.refresh()}>Retry</button></p>{:else}<p role="status">Reading directories…</p>{/if}
  </div></div>
</section>

<style>
  .browser { border: 1px solid var(--border); border-radius: 8px; overflow: hidden; font-size: 12px; }
  nav { display: flex; gap: 5px; flex-wrap: wrap; border-bottom: 1px solid var(--border); padding: 6px; background: var(--muted); }
  button { border: 0; padding: 5px; background: none; color: inherit; font: inherit; cursor: pointer; text-align: left; }
  button:hover { background: var(--muted); }
  .columns { display: grid; grid-template-columns: 1fr 1.5fr; min-height: 150px; max-height: 220px; }
  .parent { border-right: 1px solid var(--border); padding: 5px; overflow: auto; }
  .parent p { margin: 6px; overflow-wrap: anywhere; color: var(--muted-foreground); }
  .children { overflow: auto; }
  ul { padding: 4px; margin: 0; list-style: none; }
  li button { display: flex; width: 100%; align-items: center; gap: 7px; border-radius: 4px; }
  li button span:last-child { margin-left: auto; }
  .empty, .children p { margin: 8px; color: var(--muted-foreground); }
</style>
