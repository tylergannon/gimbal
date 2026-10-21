<script lang="ts">
  import ChevronRight from '@lucide/svelte/icons/chevron-right';
  import Terminal from '@lucide/svelte/icons/terminal';
  import Wrench from '@lucide/svelte/icons/wrench';
  import type { JSONObject } from '#lib/sessionstate';
  import Payload from '#lib/run/detail/Payload.svelte';
  import StructuredValue from '#lib/run/detail/StructuredValue.svelte';
  import { isCommand, toolDetail, toolStatus, toolVerb } from './activity';
  let { part }: { part: JSONObject } = $props();
  const command = $derived(isCommand(part));
  const detail = $derived(toolDetail(part));
  const status = $derived(toolStatus(part));
  const label = $derived(command ? `${toolVerb(part)} ${detail || 'command'}` : String(part.name ?? 'Tool'));
  const output = $derived((part.state?.content ?? []).filter((entry: JSONObject) => entry.type === 'text').map((entry: JSONObject) => entry.text ?? '').join('\n'));
  const error = $derived(part.state?.error ? typeof part.state.error === 'string' ? part.state.error : part.state.error.message ?? JSON.stringify(part.state.error) : '');
  const terminal = $derived(`$ ${detail}${output ? `\n${output}` : ''}${error ? `\n${error}` : ''}`);
  const structuredOutput = $derived((part.state?.content ?? []).filter((entry: JSONObject) => !['text', 'transcript'].includes(entry.type)));
  const progress = $derived(['running', 'streaming'].includes(status) ? part.state?.metadata : undefined);
</script>

{#if part.type === 'reasoning'}
  {#if part.text?.trim()}<p class="narrative">{part.text}</p>{/if}
{:else}
  <details class="event">
    <summary title={label}>
      {#if command}<Terminal size={15} />{:else}<Wrench size={15} />{/if}
      <span class="label"><span>{label}</span>{#if !command && detail} · {detail}{/if}</span>
      {#if !['completed', 'success'].includes(status)}<span class={['state', { failed: ['error', 'failed'].includes(status) }]} role="status">{status}</span>{/if}
      <ChevronRight size={14} class="chevron" />
    </summary>
    <div class="body">
      {#if command}
        <div class="terminal"><div class="terminal-title">Shell</div>
          <!-- svelte-ignore a11y_no_noninteractive_tabindex (the scroll region needs keyboard access to long command output) -->
          <pre tabindex="0" role="region" aria-label="Shell command and output">{terminal}</pre>
        </div>
      {:else}
        {#if part.state?.input}<StructuredValue value={part.state.input} />{/if}
        {#if output}<Payload label="output" text={output} maxHeight={300} />{/if}
        {#each structuredOutput as content (content)}<StructuredValue value={content} />{/each}
        {#if progress}<StructuredValue value={progress} />{/if}
        {#if error}<Payload label="error" text={error} tone="error" />{/if}
      {/if}
    </div>
  </details>
{/if}

<style>
  .event { min-width:0; color:var(--muted-foreground); font:400 15px/22px var(--font-sans); }
  summary { display:flex; align-items:center; gap:8px; min-width:0; min-height:28px; cursor:pointer; list-style:none; border-radius:4px; }
  summary::-webkit-details-marker { display:none; }
  summary :global(svg) { flex:none; }
  summary:focus-visible, pre:focus-visible { outline:2px solid var(--ring); outline-offset:2px; }
  .label { min-width:0; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
  summary :global(.chevron) { flex:none; transition:transform .1s; }
  .event[open]>summary :global(.chevron) { transform:rotate(90deg); }
  .state { flex:none; margin-left:auto; font-size:12px; }
  .failed { color:var(--destructive); }
  .body { min-width:0; padding:8px 0 12px; }
  .terminal { min-width:0; overflow:hidden; border:1px solid var(--border); border-radius:9px; background:var(--muted); }
  .terminal-title { padding:10px 12px 0; }
  pre { max-height:300px; margin:0; padding:14px 12px; overflow:auto; color:var(--muted-foreground); font:13px/21px var(--font-mono); white-space:pre; }
  .narrative { margin:0; color:var(--foreground); white-space:pre-wrap; overflow-wrap:anywhere; font:15px/22px var(--font-sans); }
  @media(prefers-reduced-motion:reduce) { summary :global(.chevron) { transition:none; } }
</style>
