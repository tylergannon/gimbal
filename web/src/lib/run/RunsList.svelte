<script module lang="ts">
  import type { InterviewRow, RunRow } from '../observation/index.js';
  export type RunsFilter = 'live' | 'needs-you' | 'ended' | 'all';
  export type RunCardItem = { run: RunRow; summary: string; elapsed: string; activity_at: number; cost: string; session_count: number; turn_count: number; instruction: string };
  export type RunsListProps = { items: RunCardItem[]; attention: InterviewRow[]; now?: number; active?: boolean; onopenrun?: (run: RunRow) => void };
</script>
<script lang="ts">
  import SearchIcon from '@lucide/svelte/icons/search';
  import EllipsisIcon from '@lucide/svelte/icons/ellipsis';
  import * as Popover from '#lib/components/ui/popover';
  import * as AlertDialog from '#lib/components/ui/alert-dialog';
  import { cancelRun } from '../../routes/control.remote';
  import { navigateOnUnauthorized } from '#lib/auth/unauthorized';
  import { errorMessage } from '#lib/errors';
  import Pip from './Pip.svelte';
  let { items, attention, now = Date.now(), active = true, onopenrun }: RunsListProps = $props();
  let filter = $state<RunsFilter>('all');
  let query = $state('');
  let searchOpen = $state(false);
  let searchInput = $state<HTMLInputElement>();
  let cancelling = $state<RunRow>();
  let busy = $state(false);
  let menuRun = $state('');
  let feedback = $state('');
  $effect(() => { if (!active) { menuRun = ''; cancelling = undefined; feedback = ''; } });
  const pending = $derived(new Set(attention.filter(item => item.status === 'pending').map(item => item.run)));
  const filters = [{ value: 'live', label: 'Live' }, { value: 'needs-you', label: 'Needs you' }, { value: 'ended', label: 'Ended' }, { value: 'all', label: 'All' }] as const;
  const shown = $derived(items.filter(item => {
    const matches = filter === 'all' || filter === 'live' && item.run.status === 'running' || filter === 'needs-you' && pending.has(item.run.id) || filter === 'ended' && item.run.status !== 'running';
    const needle = query.trim().toLocaleLowerCase();
    return matches && (!needle || `${item.run.name} ${item.run.id} ${item.summary}`.toLocaleLowerCase().includes(needle));
  }));
  const pipState = (run: RunRow) => pending.has(run.id) && run.status === 'running' ? 'waiting' : run.status === 'running' ? 'running' : run.status === 'failed' ? 'failed' : 'ended';
  const status = (run: RunRow) => pending.has(run.id) && run.status === 'running' ? 'Needs you' : run.status === 'completed' ? 'Completed' : run.status[0].toUpperCase() + run.status.slice(1);
  function relative(timestamp: number) { const seconds = Math.max(0, Math.floor((now - timestamp) / 1000)); return timestamp === 0 ? 'No activity yet' : seconds < 60 ? `${seconds} s ago` : seconds < 3600 ? `${Math.floor(seconds / 60)} min ago` : new Date(timestamp).toLocaleString(); }
  async function cancel() {
    if (!cancelling || busy) return;
    busy = true; feedback = '';
    try { const result = await cancelRun({ run: cancelling.id }); feedback = result.accepted ? 'Cancellation accepted. Waiting for the run record to update.' : 'The run did not accept cancellation.'; if (result.accepted) cancelling = undefined; }
    catch (error) { if (!navigateOnUnauthorized(error)) feedback = errorMessage(error); }
    finally { busy = false; }
  }
</script>
<section class="runs-list" aria-labelledby="runs-title">
  <header><h1 id="runs-title">Runs</h1><div class="tools"><div class="filters" role="group" aria-label="Filter runs">
    {#each filters as item (item.value)}<button aria-pressed={filter === item.value} onclick={() => filter = item.value}>{item.label}</button>{/each}
  </div>
  {#if searchOpen}<input bind:this={searchInput} type="search" aria-label="Search runs" placeholder="Search runs" bind:value={query} onkeydown={(event) => { if (event.key === 'Escape') { query = ''; searchOpen = false; } }} />{/if}
  <button class="search-button" aria-label={searchOpen ? 'Close search' : 'Search runs'} aria-expanded={searchOpen} onclick={async () => { searchOpen = !searchOpen; if (!searchOpen) query = ''; else { const { tick } = await import('svelte'); await tick(); searchInput?.focus(); } }}><SearchIcon size={14} /></button>
  </div></header>
  <div class="rows">
    {#each shown as item (item.run.id)}
      <div class="run-row">
        <button type="button" class="open-run" aria-label={`Open ${item.run.name} ${item.run.id}`} onclick={() => onopenrun?.(item.run)}>
          <span title={status(item.run)}><Pip state={pipState(item.run)} /><span class="sr-only">{status(item.run)}</span></span>
          <span class="copy"><strong>{item.run.name}</strong><span class="summary" title={`${item.summary || item.run.error || 'Recorded run'} · ${relative(item.activity_at)}`}>{item.summary || item.run.error || 'Recorded run'}</span></span>
          <span class="facts"><span title="Elapsed">{item.elapsed}</span><span title="Total cost">{item.cost}</span><span title={`${item.session_count} sessions · ${item.turn_count} turns`}>{item.session_count} {item.session_count === 1 ? 'session' : 'sessions'}</span></span>
        </button>
        {#if item.run.status === 'running'}
          <Popover.Root open={active && menuRun === item.run.id} onOpenChange={(open) => menuRun = open ? item.run.id : ''}><Popover.Trigger class="run-menu" aria-label={`Actions for ${item.run.name} ${item.run.id}`}><EllipsisIcon size={15} /></Popover.Trigger>{#if active && menuRun === item.run.id}<Popover.Content align="end" class="w-40 p-1"><button class="cancel-action" onclick={() => { menuRun = ''; cancelling = item.run; feedback = ''; }}>Cancel run…</button></Popover.Content>{/if}</Popover.Root>
        {:else}<span class="menu-space"></span>{/if}
      </div>
    {:else}<p class="empty" role="status">No runs match this filter or search.</p>{/each}
  </div>
  {#if feedback}<p class="feedback" role="status">{feedback}</p>{/if}
</section>
{#if active && cancelling}
<AlertDialog.Root open onOpenChange={(next) => { if (!next && !busy) cancelling = undefined; }}>
  <AlertDialog.Content animate={false}><AlertDialog.Header><AlertDialog.Title>Cancel the whole run?</AlertDialog.Title><AlertDialog.Description>This ends all active work in {cancelling.name}, including its sessions, commands and child scopes. The record so far is kept. The run record will report the outcome.</AlertDialog.Description></AlertDialog.Header>
  {#if feedback}<p role="alert">{feedback}</p>{/if}
  <AlertDialog.Footer><AlertDialog.Cancel disabled={busy}>Keep running</AlertDialog.Cancel><AlertDialog.Action variant="destructive" disabled={busy} onclick={cancel}>{busy ? 'Cancelling…' : 'Cancel run'}</AlertDialog.Action></AlertDialog.Footer></AlertDialog.Content>
</AlertDialog.Root>
{/if}
<style>
 .runs-list { width: 100%; max-width: 1100px; margin: 0 auto; padding: 12px 0 24px; color: var(--foreground); }
 header { display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 12px; margin-bottom: 18px; }
 h1 { margin: 0; font-size: 15px; font-weight: 500; } .tools, .filters { display: flex; align-items: center; gap: 3px; } .tools { gap: 10px; }
 button { font: inherit; cursor: pointer; } .filters button { background: transparent; border: 0; border-radius: 6px; padding: 4px 9px; font-size: 12px; color: var(--muted-foreground); } .filters button[aria-pressed='true'] { background: var(--muted); color: var(--foreground); }
 .search-button { padding: 5px; background: transparent; border: 0; color: var(--muted-foreground); } input { width: 160px; border: 1px solid var(--border); border-radius: 6px; padding: 4px 8px; color: var(--foreground); background: var(--background); font-size: 12px; }
 .run-row { display: flex; align-items: center; min-width: 0; border-bottom: 1px solid color-mix(in oklch, var(--border) 65%, transparent); } .run-row:hover { background: var(--muted); }
 .open-run { display: flex; flex: 1; align-items: center; min-width: 0; gap: 12px; padding: 14px 8px; border: 0; color: inherit; background: transparent; text-align: left; }
 .copy { display: flex; align-items: baseline; min-width: 0; flex: 1; gap: 12px; } strong { flex: 0 0 auto; font-size: 13px; font-weight: 500; } .summary { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--muted-foreground); font-size: 12px; }
 .facts { display: flex; gap: 16px; color: var(--muted-foreground); font: 11px var(--font-mono); white-space: nowrap; font-variant-numeric: tabular-nums; } .facts span { min-width: 50px; text-align: right; }
 :global(.run-menu), .menu-space { display: grid; width: 26px; height: 26px; flex: none; place-items: center; border: 0; border-radius: 5px; color: var(--muted-foreground); background: transparent; } :global(.run-menu) { opacity: 0; cursor: pointer; } .run-row:hover :global(.run-menu), .run-row:focus-within :global(.run-menu) { opacity: 1; }
 .cancel-action { display: block; width: 100%; border: 0; background: transparent; color: var(--foreground); padding: 7px 9px; text-align: left; font-size: 12px; }
 .empty { margin: 30px 0; color: var(--muted-foreground); font-size: 13px; } .feedback { color: var(--muted-foreground); font-size: 12px; }
 button:focus-visible, input:focus-visible { outline: 2px solid var(--ring); outline-offset: 2px; }
 @media(max-width:700px) { .copy { flex-direction: column; gap: 3px; } .summary { max-width: 100%; } .facts { gap: 8px; } .facts span:last-child { display: none; } }
</style>
