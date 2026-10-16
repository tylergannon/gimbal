<script lang="ts">
  import type { JSONObject } from '#lib/sessionstate';
  import ToolCall from '#lib/run/detail/ToolCall.svelte';
  let { part }: { part: JSONObject } = $props();
  const label = $derived(part.type === 'reasoning' ? 'Thinking' : ({ commandExecution: 'Shell', fileChange: 'Edit files', webSearch: 'Search' } as Record<string, string>)[part.name] ?? part.name ?? 'Tool');
  const detail = $derived.by(() => { const input = part.state?.input; return input && typeof input === 'object' ? input.file_path ?? input.path ?? input.command ?? input.description ?? '' : typeof input === 'string' ? input : ''; });
</script>
{#if part.type !== 'reasoning' || part.text?.trim()}
  <details class="event"><summary><span class="chevron">›</span><span class="kind">{label}</span><code title={String(detail)}>{detail}</code><span class="state">{part.state?.status ?? ''}</span></summary><div class="body">{#if part.type === 'reasoning'}<p>{part.text}</p>{:else}<ToolCall {part} />{/if}</div></details>
{/if}
<style>
  .event { min-width:0; border-bottom:1px solid var(--border); color:var(--muted-foreground); font:12px/18px var(--font-sans); } summary { display:flex; align-items:center; gap:8px; min-height:30px; cursor:pointer; list-style:none; } summary::-webkit-details-marker { display:none; } .chevron { font-size:17px; width:10px; transition:transform .1s; } .event[open] .chevron { transform:rotate(90deg); } .kind { color:var(--foreground); font-weight:500; white-space:nowrap; } code { min-width:0; flex:1; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; font:12px var(--font-mono); } .state { margin-left:auto; white-space:nowrap; font-size:11px; } .body { padding:8px 0 12px 18px; } p { margin:0; font:13px/20px var(--font-sans); white-space:pre-wrap; overflow-wrap:anywhere; }
</style>
