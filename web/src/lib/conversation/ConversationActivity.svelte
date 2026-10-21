<script lang="ts">
  import ChevronRight from '@lucide/svelte/icons/chevron-right';
  import Terminal from '@lucide/svelte/icons/terminal';
  import Wrench from '@lucide/svelte/icons/wrench';
  import type { ActivityItem } from './activity';
  import { groupSummary, isCommand, toolStatus } from './activity';
  import ConversationMessage from './ConversationMessage.svelte';
  import TranscriptEvent from './TranscriptEvent.svelte';
  let { items }: { items: ActivityItem[] } = $props();
  const failures = (item: Extract<ActivityItem, { type: 'tools' }>) => item.parts.filter(part => ['error', 'failed'].includes(toolStatus(part))).length;
</script>

<div class="activity-content">
  {#each items as item (item.id)}
    {#if item.type === 'tools'}
      <details class="tool-group">
        <summary title={groupSummary(item.parts)}>{#if item.parts.every(isCommand)}<Terminal size={15} />{:else}<Wrench size={15} />{/if}<span class="group-label">{groupSummary(item.parts)}</span><ChevronRight size={14} class="chevron" />{#if failures(item)}<span class="failed">{failures(item)} failed</span>{/if}</summary>
        <div class="tool-rows">{#each item.parts as part (part.id)}<TranscriptEvent {part} />{/each}</div>
      </details>
    {:else}
      <ConversationMessage message={item.message} provider="Agent" model="" location="" compact />
    {/if}
  {/each}
</div>

<style>
  .activity-content { display:grid; gap:20px; min-width:0; }
  .tool-group { min-width:0; color:var(--muted-foreground); font:400 15px/22px var(--font-sans); }
  summary { display:flex; align-items:center; gap:8px; width:fit-content; max-width:100%; min-height:28px; cursor:pointer; list-style:none; border-radius:4px; }
  summary::-webkit-details-marker { display:none; }
  summary :global(svg) { flex:none; }
  summary :global(.chevron) { transition:transform .1s; }
  .tool-group[open]>summary :global(.chevron) { transform:rotate(90deg); }
  summary:focus-visible { outline:2px solid var(--ring); outline-offset:3px; }
  .tool-rows { display:grid; gap:2px; min-width:0; padding-top:3px; }
  .group-label { min-width:0; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
  .failed { flex:none; color:var(--destructive); font-size:12px; }
  @media(prefers-reduced-motion:reduce) { summary :global(.chevron) { transition:none; } }
</style>
