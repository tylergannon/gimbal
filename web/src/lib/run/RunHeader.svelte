<script lang="ts">
  import Search from '@lucide/svelte/icons/search';
  import MoreHorizontal from '@lucide/svelte/icons/ellipsis';
  import { clock } from '#lib/clock.svelte';
  import type { RunRow, ConnectionState } from '#lib/observation';
  import type { RunNavigationItem } from './selection';
  import Pip from './Pip.svelte';
  let { run, waiting, connection, mode, onmode, oncancel, searchItems, onnavigate }: {
    run: RunRow; waiting: number; connection: ConnectionState; mode: 'transcript' | 'map' | 'focus';
    onmode: (mode: 'transcript' | 'map') => void; oncancel: () => void;
    searchItems: RunNavigationItem[]; onnavigate: (item: RunNavigationItem) => void;
  } = $props();
  let searching = $state(false);
  let query = $state('');
  const seconds = $derived(Math.max(0, Math.floor(((run.ended || clock.now) - run.started) / 1000)));
  const matches = $derived(searchItems.filter(item => `${item.label} ${item.context}`.toLowerCase().includes(query.toLowerCase())).slice(0, 8));
</script>
<header class="run-head">
  <h1>{run.name}</h1>
  <span class="status"><Pip state={waiting ? 'needs-you' : run.status === 'running' ? 'running' : run.status === 'failed' ? 'failed' : run.status === 'cancelled' ? 'cancelled' : 'ended'} />{waiting ? 'Needs you' : run.status === 'running' ? 'Running' : run.status === 'completed' ? 'Ended' : run.status === 'failed' ? 'Failed' : 'Cancelled'} <span class="quiet">{Math.floor(seconds / 60)}m {String(seconds % 60).padStart(2, '0')}s</span></span>
  <div class="spacer"></div>
  <div class="view" role="group" aria-label="Run view"><button class={{ chosen: mode !== 'map' }} onclick={() => onmode('transcript')}>Transcript</button><button class={{ chosen: mode === 'map' }} onclick={() => onmode('map')}>Map</button></div>
  {#if run.task_environment}<span class="identity">{run.task_environment}</span>{/if}
  <button class="icon" aria-label="Find a scope or turn" onclick={() => searching = !searching}><Search size={15} /></button>
  <details class="more"><summary aria-label="More run actions"><MoreHorizontal size={17} /></summary><div class="menu"><span class="quiet">{run.id}</span>{#if run.status === 'running' || run.cancellation?.cleanup === 'unconfirmed'}<button onclick={oncancel}>{run.status === 'running' ? 'Cancel run…' : 'Retry cleanup'}</button>{/if}</div></details>
  {#if run.status === 'running' && connection === 'disconnected'}<p class="lost" role="status">Observation stream disconnected. The run may still be working.</p>{/if}
  {#if searching}<div class="find"><input aria-label="Find a scope or turn" placeholder="Find a scope or turn…" bind:value={query} />{#each matches as item (item.id)}<button onclick={() => { searching = false; onnavigate(item); }}><strong>{item.label}</strong><span class="quiet">{item.context}</span></button>{/each}</div>{/if}
</header>
<style>
  .run-head { position: relative; display: flex; align-items: center; flex-wrap: wrap; gap: 12px; width: 100%; max-width: 860px; box-sizing: border-box; margin: 0 auto; padding: 28px 0 16px; }
  h1 { margin: 0; font: 600 20px/26px var(--font-sans); } .status { display: inline-flex; align-items: center; gap: 6px; font-size: 13px; } .quiet { color: var(--muted-foreground); font-size: 12px; } .spacer { flex: 1; } button, summary { cursor: pointer; }
  .view { display: flex; border: 1px solid var(--border); border-radius: 5px; overflow: hidden; } .view button { padding: 4px 10px; border: 0; color: var(--muted-foreground); background: var(--background); font: 12px/18px var(--font-sans); } .view button.chosen { background: var(--foreground); color: var(--background); }
  .identity { border: 1px solid var(--border); border-radius: 6px; padding: 6px 10px; font-size: 12px; } .icon, summary { display: grid; place-items: center; width: 30px; height: 30px; border: 1px solid var(--border); border-radius: 6px; background: var(--background); color: var(--foreground); } summary { border: 0; list-style: none; } summary::-webkit-details-marker { display: none; }
  .more { position: relative; } .menu { position: absolute; right: 0; top: 34px; z-index: 20; display: grid; gap: 12px; width: 230px; padding: 12px; border: 1px solid var(--border); border-radius: 8px; background: var(--background); box-shadow: 0 6px 24px #0002; overflow-wrap: anywhere; } .menu button { text-align: left; font: inherit; color: var(--destructive); border: 0; background: transparent; }
  .find { display: grid; gap: 4px; width: 100%; padding: 10px; border: 1px solid var(--border); border-radius: 6px; } .find input { padding: 8px; color: var(--foreground); background: var(--background); border: 1px solid var(--border); border-radius: 4px; } .find button { display: flex; justify-content: space-between; gap: 12px; padding: 6px; color: var(--foreground); background: transparent; border: 0; text-align: left; font: 12px var(--font-sans); } .find button:hover { background: var(--muted); } .lost { flex-basis: 100%; margin: 0; color: var(--status-needs-you); font-size: 12px; }
  @media(max-width:1000px) { .run-head { padding: 20px 24px 16px; } }
</style>
