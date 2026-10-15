<script lang="ts">
  import * as Popover from '#lib/components/ui/popover';
  import Monitor from '@lucide/svelte/icons/monitor';
  import Check from '@lucide/svelte/icons/check';
  import ChevronDown from '@lucide/svelte/icons/chevron-down';
  import type { Snapshot } from '#lib/skgo/environments/types';
  import { openMachines, timeLabel } from './navigation';
  import StateGlyph from './StateGlyph.svelte';

  let { snapshot, selected = 'local', directory, machine = 'This Mac', onpick }: {
    snapshot: Snapshot; selected?: string; directory: string; machine?: string; onpick: (id: string) => void;
  } = $props();
  let open = $state(false);
  const machines = $derived([...snapshot.machines].sort((a, b) => {
    if (a.id === 'local') return -1;
    if (b.id === 'local') return 1;
    const recent = (id: string) => Math.max(0, ...snapshot.environments.filter(row => row.environment.machine === id).map(row => row.last_used));
    return recent(b.id) - recent(a.id);
  }));
</script>

<Popover.Root bind:open>
  <Popover.Trigger class="chip environment" aria-label="Environment" title={`${machine} · ${directory}`}>
    <Monitor size={14} aria-hidden="true" /><span class="chip-label">{machine} · {directory}</span><ChevronDown size={14} aria-hidden="true" />
  </Popover.Trigger>
  <Popover.Content align="start" collisionPadding={8} class="w-[min(400px,calc(100vw-32px))] gap-1 p-1.5">
    {#each machines as item (item.id)}
      <h3>{item.name}{#if item.id !== 'local'}<small> · {item.kind === 'route' ? 'Queue / plugin' : item.kind === 'ssh' ? 'SSH' : 'Address'}</small>{/if}</h3>
      {#if item.state !== 'connected'}<p class="state" class:error={item.state === 'error'}>
        <StateGlyph state={item.state} />
        {item.state === 'not-started' ? 'Starts when needed' : `${item.state} since ${timeLabel(item.since)}`}
        {#if item.detail}<small>{item.detail}</small>{/if}
        {#if item.state === 'error'}<small>Open Machines to reconnect or fix this machine in Terminal.</small>{/if}
      </p>{/if}
      {#each snapshot.environments.filter(row => row.environment.machine === item.id) as row (row.environment.id)}
        <button type="button" class="option" class:selected={selected === row.environment.id}
          disabled={item.state === 'unreachable' || item.state === 'error' || item.state === 'connecting' || item.state === 'starting' || !!row.issues.length}
          title={row.issues.map(issue => issue.message).join('\n')}
          onclick={() => { onpick(row.environment.id); open = false; }}>
          <span class="label">{row.environment.directory}<small>{row.directory.git ? `Git · ${row.directory.branch}` : row.directory.unborn ? 'Git · No commits yet' : row.observed ? 'Not a Git repository' : 'Inspects when selected'}{#if row.live_sessions} · {row.live_sessions} sessions{/if}{#if !row.capabilities.steer} · No steering{/if}</small></span>
          {#if selected === row.environment.id}<Check size={14} aria-hidden="true" />{/if}
        </button>
      {/each}
    {/each}
    {#each snapshot.issues as issue}<p class="state error">{issue.entry ? `${issue.entry}: ` : ''}{issue.message}</p>{/each}
    <div class="actions"><button type="button" onclick={() => { open = false; openMachines(snapshot.project); }}>Add environment…</button><button type="button" onclick={() => { open = false; openMachines(); }}>Machines…</button></div>
  </Popover.Content>
</Popover.Root>

<style>
  h3 { margin: 8px 8px 3px; font-size: 12px; font-weight: 500; }
  small { color: var(--muted-foreground); font-size: 11px; font-weight: 400; }
  .option { display: flex; width: 100%; align-items: center; gap: 8px; padding: 7px 8px; border: 0; border-radius: 6px; color: inherit; background: none; font: inherit; font-size: 13px; text-align: left; cursor: pointer; }
  .option:hover, .option.selected { background: var(--muted); }
  .option:disabled { opacity: .5; cursor: default; }
  .label { display: grid; flex: 1; min-width: 0; overflow-wrap: anywhere; }
  .state { margin: 3px 8px; color: var(--muted-foreground); font-size: 12px; overflow-wrap: anywhere; }
  .state small { display: block; }
  .error { color: var(--destructive); }
  .actions { display: flex; gap: 12px; justify-content: space-between; margin-top: 5px; padding: 8px; border-top: 1px solid var(--border); }
  .actions button { padding: 0; border: 0; background: none; color: inherit; font: inherit; font-size: 12px; cursor: pointer; }
</style>
