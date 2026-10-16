<script lang="ts">
  import { tick } from 'svelte';
  import { ChevronDown, Plus, X } from '@lucide/svelte';
  import * as Dropdown from '#lib/components/ui/dropdown-menu';
  import TabGlyph from './TabGlyph.svelte';
  import TabStatus from './TabStatus.svelte';
  import { groupTabs, tabUrgency, type TabGrouping } from './tabs';
  import type { WorkspaceTab } from './types';
  let { tabs, active, onselect, onclose, onnew }: {
    tabs: WorkspaceTab[]; active?: string; onselect: (id: string) => void;
    /** Closing a view never cancels its execution. */
    onclose: (id: string) => void; onnew?: () => void;
  } = $props();
  let width = $state(0);
  let grouping = $state<TabGrouping>('origin');
  let individual = $state<string[]>([]);
  let expanded = $state('');
  let buttons = $state<Record<string, HTMLButtonElement>>({});
  const entries = $derived(groupTabs(tabs, grouping, individual));
  const selected = (tab: WorkspaceTab) => tab.id === active || !!tab.members?.some(member => member.id === active);
  const capacity = $derived(Math.max(1, Math.floor((width - (onnew ? 32 : 0) - 78) / 200)));
  const visible = $derived.by(() => {
    if (entries.length * 200 + 64 + (onnew ? 32 : 0) <= width) return entries;
    const current = entries.find(selected);
    const first = entries.slice(0, capacity);
    if (current && !first.includes(current)) first[first.length - 1] = current;
    return first;
  });
  const overflow = $derived(entries.filter(tab => !visible.includes(tab)));
  async function select(id: string) { expanded = ''; onselect(id); await tick(); buttons[entries.find(tab => tab.id === id || tab.members?.some(member => member.id === id))?.id ?? id]?.focus(); }
  async function close(tab: WorkspaceTab) { for (const member of tab.members ?? [tab]) onclose(member.id); await tick(); buttons[entries.find(selected)?.id ?? '']?.focus(); }
  function keydown(event: KeyboardEvent, tab: WorkspaceTab) {
    const index = entries.findIndex(item => item.id === tab.id);
    let next: WorkspaceTab | undefined;
    if (event.key === 'ArrowRight') next = entries[(index + 1) % entries.length];
    else if (event.key === 'ArrowLeft') next = entries[(index - 1 + entries.length) % entries.length];
    else if (event.key === 'Home') next = entries[0];
    else if (event.key === 'End') next = entries.at(-1);
    else if (event.key === 'Delete') { event.preventDefault(); void close(tab); return; }
    else return;
    event.preventDefault(); if (next) void select(next.members?.find(member => member.id === active)?.id ?? next.members?.[0].id ?? next.id);
  }
  function regroup(value: string) { grouping = value as TabGrouping; individual = []; }
</script>
{#snippet label(tab: WorkspaceTab)}
  {#if tab.status}<TabStatus status={tab.status} />{:else}<TabGlyph {tab} />{/if}<span class="title">{tab.title}</span>{#if tab.unseen}<span class="unseen-mark" role="img" aria-label="Unseen changes"></span>{/if}{#if tab.count}<span class="count">{tab.count}</span>{/if}
{/snippet}
{#snippet groupChoices(tab: WorkspaceTab)}
  <Dropdown.Label>{tab.origin ? `Started by ${tab.origin.title}` : tab.title} · {tab.count} runs</Dropdown.Label>
  {#each [...(tab.members ?? [])].sort((a,b) => tabUrgency(a)-tabUrgency(b)) as member (member.id)}
    <Dropdown.Item onclick={() => select(member.id)} class="member"><TabStatus status={member.status} /><span class="choice-title">{member.title}</span>{#if member.unseen}<span class="unseen-mark" role="img" aria-label="Unseen changes"></span>{/if}<span class="detail">{member.detail}</span></Dropdown.Item>
  {/each}
  <Dropdown.Separator /><Dropdown.Item onclick={() => { individual = [...individual, tab.id]; expanded = ''; }}>Open each as its own tab</Dropdown.Item>
  {@render groupingChoices()}
{/snippet}
{#snippet groupingChoices()}
  <Dropdown.Sub><Dropdown.SubTrigger>Group tabs by</Dropdown.SubTrigger><Dropdown.SubContent><Dropdown.RadioGroup value={grouping} onValueChange={regroup}>{#each ['origin','workflow','none'] as mode (mode)}<Dropdown.RadioItem value={mode}>{mode[0].toUpperCase()+mode.slice(1)}</Dropdown.RadioItem>{/each}</Dropdown.RadioGroup></Dropdown.SubContent></Dropdown.Sub>
{/snippet}
<div class="strip" bind:clientWidth={width}>
  <div class="tabs" role="tablist" aria-label="Open sessions, runs and files">
    {#each visible as tab (tab.id)}
      <div class={['tab', { active: selected(tab), unseen: tab.unseen, grouped: !!tab.members }]} role="presentation">
        {#if tab.members}
          <Dropdown.Root open={expanded === tab.id} onOpenChange={value => expanded = value ? tab.id : ''}>
            <Dropdown.Trigger class="select group" role="tab" aria-selected={selected(tab)} tabindex={selected(tab) ? 0 : -1} bind:ref={() => buttons[tab.id] ?? null, value => { if (value) buttons[tab.id] = value; }} title={tab.title} onkeydown={event => keydown(event, tab)}>{@render label(tab)}<ChevronDown size={11} /></Dropdown.Trigger>
            <Dropdown.Content align="start" class="w-[400px] max-w-[80vw]">{@render groupChoices(tab)}</Dropdown.Content>
          </Dropdown.Root>
        {:else}
          <button class="select" type="button" role="tab" bind:this={buttons[tab.id]} id={`workspace-tab-${tab.id}`} aria-controls={`workspace-panel-${tab.id}`} aria-selected={selected(tab)} tabindex={selected(tab) ? 0 : -1} title={tab.title} onclick={() => select(tab.id)} onkeydown={event => keydown(event, tab)}>{@render label(tab)}</button>
        {/if}
        <button class="close" type="button" aria-label={`Close ${tab.title}`} title="Close view" onclick={() => close(tab)}><X size={12} aria-hidden="true" /></button>
      </div>
    {/each}
  </div>
  <Dropdown.Root open={expanded === 'overflow'} onOpenChange={value => expanded = value ? 'overflow' : ''}>
    <Dropdown.Trigger class="overflow" aria-label={overflow.length ? `${overflow.length} more tabs` : 'Tab options'}>{#if overflow.length}{overflow.length} more{/if}<ChevronDown size={12} aria-hidden="true" /></Dropdown.Trigger>
    <Dropdown.Content align="end" class="w-80 max-w-[80vw]">
      {#each overflow as tab (tab.id)}
        {#if tab.members}<Dropdown.Sub><Dropdown.SubTrigger>{@render label(tab)}</Dropdown.SubTrigger><Dropdown.SubContent class="w-[400px] max-w-[80vw]">{@render groupChoices(tab)}</Dropdown.SubContent></Dropdown.Sub>
        {:else}<Dropdown.Item onclick={() => select(tab.id)}><TabGlyph {tab} /><span class="choice-title">{tab.title}</span><TabStatus status={tab.status} />{#if tab.unseen}<span class="unseen-mark" role="img" aria-label="Unseen changes"></span>{/if}</Dropdown.Item>{/if}
      {/each}
      {#if overflow.length}<Dropdown.Separator />{/if}{@render groupingChoices()}
    </Dropdown.Content>
  </Dropdown.Root>
  {#if onnew}<button class="new" type="button" aria-label="New tab" onclick={onnew}><Plus size={16} aria-hidden="true" /></button>{/if}
</div>
<style>
  .strip { display:flex; align-items:center; gap:2px; min-width:0; flex:1; } .tabs { display:flex; gap:2px; min-width:0; } .tab { display:flex; align-items:center; min-width:0; max-width:200px; border-radius:6px; color:var(--muted-foreground); } .tab.grouped { width:200px; flex:0 0 200px; } .tab.grouped :global(.select.group) { flex:1; } .tab.active { background:var(--muted); color:var(--foreground); } .tab.unseen { color:var(--foreground); } .tab:hover, .tab:focus-within { background:var(--accent); color:var(--foreground); }
  button, :global(.select), :global(.overflow) { font:inherit; background:transparent; color:inherit; border:0; cursor:pointer; } .select, :global(.select.group) { display:flex; gap:6px; align-items:center; height:30px; padding:0 2px 0 10px; min-width:0; font-size:13px; font-weight:400; } .tab.unseen .select { font-weight:500; } .title { overflow:hidden; text-overflow:ellipsis; white-space:nowrap; } .unseen-mark { width:6px; height:6px; flex:none; border-radius:50%; background:var(--foreground); } .count { font:11px var(--font-sans); border-radius:4px; padding:1px 4px; background:var(--muted); }
  .close { display:flex; align-items:center; justify-content:center; flex:none; width:22px; height:26px; opacity:0; border-radius:5px; } .tab:hover .close, .tab:focus-within .close, .close:focus-visible { opacity:1; } button:focus-visible, :global(.select:focus-visible), :global(.overflow:focus-visible) { outline:2px solid var(--ring); outline-offset:-2px; }
  .new { display:flex; align-items:center; justify-content:center; width:28px; height:28px; flex:none; border-radius:6px; color:var(--muted-foreground); } .new:hover { background:var(--accent); } :global(.overflow) { display:flex; align-items:center; gap:4px; padding:6px 8px; border-radius:6px; font-size:12px; color:var(--muted-foreground); } .choice-title { flex:1; min-width:0; white-space:normal; overflow-wrap:anywhere; } .detail { color:var(--muted-foreground); font-size:11px; margin-left:auto; } @media(hover:none) { .close { opacity:1; } }
</style>
