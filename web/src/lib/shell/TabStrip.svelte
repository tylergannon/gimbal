<script lang="ts">
  import { tick } from 'svelte';
  import { ChevronDown, Plus, X } from '@lucide/svelte';
  import TabGlyph from './TabGlyph.svelte';
  import TabStatus from './TabStatus.svelte';
  import type { WorkspaceTab } from './types.js';

  let { tabs, active, onselect, onclose, onnew }: {
    tabs: WorkspaceTab[];
    active?: string;
    onselect: (id: string) => void;
    /** Closes a view only; this callback must not cancel execution. */
    onclose: (id: string) => void;
    onnew?: () => void;
  } = $props();
  let width = $state(0);
  let expanded = $state(false);
  let summary = $state<HTMLElement>();
  let buttons = $state<Record<string, HTMLButtonElement>>({});
  let capacity = $derived(Math.max(1, Math.floor((width - (onnew ? 32 : 0) - 78) / 170)));
  let visible = $derived.by(() => {
    if (tabs.length * 170 + (onnew ? 32 : 0) <= width) return tabs;
    const selected = tabs.find(tab => tab.id === active);
    const first = tabs.slice(0, capacity);
    if (selected && !first.includes(selected)) first[first.length - 1] = selected;
    return first;
  });
  let overflow = $derived(tabs.filter(tab => !visible.includes(tab)));

  async function select(id: string) {
    expanded = false;
    onselect(id);
    await tick();
    buttons[id]?.focus();
  }
  async function close(id: string) {
    onclose(id);
    await tick();
    buttons[active ?? '']?.focus();
  }
  function dismiss(event: KeyboardEvent) {
    if (event.key === 'Escape') { expanded = false; summary?.focus(); }
  }
  function keydown(event: KeyboardEvent, id: string) {
    const index = tabs.findIndex(tab => tab.id === (active ?? id));
    let next: WorkspaceTab | undefined;
    if (event.key === 'ArrowRight') next = tabs[(index + 1) % tabs.length];
    else if (event.key === 'ArrowLeft') next = tabs[(index - 1 + tabs.length) % tabs.length];
    else if (event.key === 'Home') next = tabs[0];
    else if (event.key === 'End') next = tabs[tabs.length - 1];
    else if (event.key === 'Delete') { event.preventDefault(); void close(id); return; }
    else return;
    event.preventDefault();
    if (next) void select(next.id);
  }
</script>

<div class="strip" bind:clientWidth={width}>
  <div class="tabs" role="tablist" aria-label="Open chats and files">
    {#each visible as tab (tab.id)}
      <div class="tab" class:active={active === tab.id} role="presentation">
        <button class="select" type="button" role="tab" bind:this={buttons[tab.id]}
          id={`workspace-tab-${tab.id}`} aria-controls={`workspace-panel-${tab.id}`}
          aria-selected={active === tab.id} tabindex={active === tab.id ? 0 : -1}
          title={tab.title} onclick={() => onselect(tab.id)} onkeydown={event => keydown(event, tab.id)}>
          <TabGlyph {tab} /><span class="title">{tab.title}</span><TabStatus status={tab.status} />
        </button>
        <button class="close" type="button" aria-label={`Close ${tab.title}`} title="Close view"
          onclick={() => close(tab.id)}><X size={12} aria-hidden="true" /></button>
      </div>
    {/each}
  </div>
  {#if overflow.length}
    <details class="overflow" bind:open={expanded}>
      <summary bind:this={summary} onkeydown={dismiss} aria-label={`${overflow.length} more tabs`}>{overflow.length} more<ChevronDown size={12} aria-hidden="true" /></summary>
      <div class="choices" aria-label="More tabs">
        {#each overflow as tab (tab.id)}
          <button type="button" onkeydown={dismiss} onclick={() => select(tab.id)}><TabGlyph {tab} /><span>{tab.title}</span><TabStatus status={tab.status} /></button>
        {/each}
      </div>
    </details>
  {/if}
  {#if onnew}<button class="new" type="button" aria-label="New tab" onclick={onnew}><Plus size={16} aria-hidden="true" /></button>{/if}
</div>

<style>
  .strip { display: flex; align-items: center; gap: 2px; min-width: 0; flex: 1; }
  .tabs { display: flex; gap: 2px; min-width: 0; }
  .tab { display: flex; align-items: center; min-width: 0; max-width: 170px; border-radius: 8px; color: var(--muted-foreground); }
  .tab.active { background: var(--muted); color: var(--foreground); }
  .tab:hover, .tab:focus-within { background: var(--accent); color: var(--foreground); }
  button { font: inherit; background: transparent; color: inherit; border: 0; cursor: pointer; }
  .select { display: flex; gap: 8px; align-items: center; height: 30px; padding: 0 2px 0 10px; min-width: 0; font-size: 13px; font-weight: 500; }
  .title { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .close { display: flex; justify-content: center; align-items: center; flex: none; width: 22px; height: 26px; opacity: 0; border-radius: 6px; }
  .tab:hover .close, .tab:focus-within .close, .close:focus-visible { opacity: 1; }
  button:focus-visible, summary:focus-visible { outline: 2px solid var(--ring); outline-offset: -2px; }
  .new { display: flex; align-items: center; justify-content: center; height: 28px; width: 28px; flex: none; border-radius: 8px; color: var(--muted-foreground); }
  .new:hover, summary:hover { background: var(--accent); }
  .overflow { position: relative; flex: none; color: var(--muted-foreground); font-size: 12px; }
  summary { display: flex; align-items: center; gap: 4px; padding: 6px 8px; border-radius: 8px; cursor: pointer; list-style: none; }
  summary::-webkit-details-marker { display: none; }
  .choices { position: absolute; right: 0; top: 34px; z-index: 10; padding: 4px; width: 280px; max-width: 65vw; max-height: 60vh; overflow: auto; background: var(--popover); color: var(--popover-foreground); box-shadow: var(--shadow-lg); border: 1px solid var(--border); border-radius: 10px; }
  .choices button { display: flex; align-items: center; gap: 8px; padding: 8px; width: 100%; text-align: left; border-radius: 6px; }
  .choices button:hover { background: var(--accent); }
  .choices span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  @media (hover: none) { .close { opacity: 1; } }
</style>
