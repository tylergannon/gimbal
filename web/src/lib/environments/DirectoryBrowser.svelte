<script lang="ts">
  import { untrack } from 'svelte';
  import { listEnvironmentDirectories } from '#lib/environments.remote';
  import { errorMessage } from '#lib/errors';
  let { project, machine, path = '/', onchoose }: { project: string; machine: string; path?: string; onchoose: (path: string) => void } = $props();
  let directory = $state(untrack(() => path || '/'));
  const listing = $derived(listEnvironmentDirectories({ project, machine, directory }));
  const shown = $derived(listing.current?.path || directory);
  const parent = $derived(shown.replace(/\/+$/, '').replace(/\/[^/]*$/, '') || '/');
  const segments = $derived(shown.split('/').filter(Boolean));
  export function showDirectory(path: string) { directory = path || '/'; }
  function visit(path: string) { showDirectory(path); onchoose(path); }
</script>

<section class="browser" aria-label="Directory browser">
  <nav aria-label="Directory path"><button type="button" onclick={() => visit('/')}>/</button>{#each segments as segment, index (`${index}-${segment}`)}<button type="button" onclick={() => visit('/' + segments.slice(0, index + 1).join('/'))}>{segment}</button>{/each}</nav>
  <div class="columns"><div class="parent"><button type="button" disabled={shown === '/'} onclick={() => visit(parent)}>↑ Parent directory</button><p>{shown}</p></div><div class="children" role="region" aria-label="Subdirectories">
    {#if listing.current}<ul>{#each listing.current.entries as entry (entry.path)}<li><button type="button" onclick={() => visit(entry.path)}><span aria-hidden="true">▱</span>{entry.name}<span aria-hidden="true">›</span></button></li>{:else}<li class="empty">No subdirectories</li>{/each}</ul>
    {:else if listing.error}<p role="alert">{errorMessage(listing.error)} <button type="button" onclick={() => listing.refresh()}>Retry</button></p>{:else}<p role="status">Reading directories…</p>{/if}
  </div></div>
</section>

<style>
  .browser { display: flex; flex-direction: column; flex: 1 1 240px; min-height: 80px; border: 1px solid var(--border); border-radius: 8px; overflow: hidden; font-size: 12px; }
  nav { display: flex; flex: none; gap: 5px; overflow-x: auto; white-space: nowrap; border-bottom: 1px solid var(--border); padding: 6px; background: var(--muted); }
  .browser button { border: 0; padding: 5px; background: none; color: inherit; font: inherit; cursor: pointer; text-align: left; }
  .browser button:hover { background: var(--muted); }
  .columns { display: grid; flex: 1; min-height: 0; grid-template-columns: minmax(90px, 1fr) minmax(0, 2fr); grid-template-rows: minmax(0, 1fr); }
  .parent { border-right: 1px solid var(--border); padding: 5px; overflow: auto; }
  .parent p { margin: 6px; overflow-wrap: anywhere; color: var(--muted-foreground); }
  .children { min-height: 0; overflow-y: auto; overscroll-behavior: contain; }
  ul { padding: 4px; margin: 0; list-style: none; }
  .browser li button { display: flex; width: 100%; align-items: center; gap: 7px; padding: 6px 8px; border: 0; border-radius: 4px; background: none; }
  li button span:last-child { margin-left: auto; }
  .empty, .children p { margin: 8px; color: var(--muted-foreground); }
</style>
