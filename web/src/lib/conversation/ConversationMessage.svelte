<script lang="ts">
  import type { WorkspaceTab } from '#lib/shell/types';
  import type { JSONObject } from '#lib/sessionstate';
  import type { Run } from '#lib/skgo/conversation/types';
  import RunEvent from './RunEvent.svelte';
  import TranscriptEvent from './TranscriptEvent.svelte';
  import Payload from '#lib/run/detail/Payload.svelte';
  let { message, revision = 0, provider, model, location, compact = false, project = '', runs = {}, runStatuses = {} }: { message: JSONObject; revision?: number; provider: string; model: string; location: string; compact?: boolean; project?:string; runs?:Record<number,Run[]>; runStatuses?:Record<string,WorkspaceTab['status']> } = $props();
  // Canonical projections retain identity, including nested tool state. Copy only this
  // message on invalidation so derived tool input/output sees a new reference too.
  const row = $derived.by(() => { revision; return JSON.parse(JSON.stringify(message)) as JSONObject; });
  const content = $derived((row.content ?? []).filter((part: JSONObject) => (part.type !== 'tool' || part.name !== 'StructuredOutput') && (part.type !== 'reasoning' || part.text?.trim())));
  // A completion-only step (including its thinking) has no chat content.
  const completionOnly = $derived(content.length < (row.content ?? []).length && !content.some((part: JSONObject) => part.type !== 'reasoning') && !row.error && !row.retry);
  const text = (value: unknown) => typeof value === 'string' ? value : JSON.stringify(value, null, 2);
  const cause = (value: unknown): string => {
    if (!value || typeof value !== 'object') return text(value);
    const error = value as Record<string, unknown>;
    const facts = [error.message, error.type, error.code, error.status ? `HTTP ${error.status}` : undefined].filter(Boolean);
    return facts.length ? facts.join(' · ') : text(value);
  };
</script>
  {#if row.type === 'user'}
    <article class="user" data-request-id={row.requestID}><p>{row.text}</p>{#if row.steered}<span class="caption">Sent while {provider} was working</span>{/if}</article>
  {:else if row.type === 'assistant'}
    {#if !completionOnly}
    <article class="assistant">{#if !compact}<header><span class="avatar" aria-hidden="true">{provider.slice(0, 1)}</span><span>{provider} · {model} · {location}</span></header>{/if}
      {#each content as part, i (`${row.id}/${i}`)}
        {#if part.type === 'text'}{#if part.html}<div class="prose">{@html part.html}</div>{:else}<p>{part.text}</p>{/if}
        {:else if part.type === 'reasoning' || part.type === 'tool'}<TranscriptEvent {part} />{/if}
        {#each runs[(row.content ?? []).indexOf(part)] ?? [] as run (run.id)}<RunEvent {project} {run} state={runStatuses[run.id]} />{/each}
      {/each}
      {#if row.retry}<p class="warning" role="status">Retry scheduled{row.retry.attempt ? ` · attempt ${row.retry.attempt}` : ''}: {cause(row.retry.error)}</p>{/if}
      {#if row.error}<p class="error" role="alert">{cause(row.error)}</p>{/if}
    </article>
    {/if}
  {:else if row.type === 'connection'}<p class="caption">{row.message}</p>
  {:else if row.type === 'synthetic' && row.description === 'Harness retry'}<p class="warning" role="status">{row.text} · {cause({ type: row.metadata?.error, status: row.metadata?.error_status })}</p><details><summary>Retry details</summary><Payload label="retry" text={text(row.metadata)} /></details>
  {:else if row.type === 'shell'}<details><summary>Shell command</summary><Payload label="shell" text={`$ ${row.command}\n${row.output ?? ''}`} /></details>
  {:else if row.type === 'compaction'}<details><summary>Context compaction</summary><p>{row.summary}</p></details>
  {:else}<details><summary>{row.type ?? 'Activity record'}</summary><Payload label={String(row.type ?? 'event')} text={text(row)} /></details>{/if}
<style>
  article { min-width: 0; }
  p { margin: 0; white-space: pre-wrap; overflow-wrap: anywhere; font-size: 15px; line-height: 22px; }
  .prose { min-width: 0; overflow-wrap: anywhere; font-size: 15px; line-height: 22px; }
  .prose :global(p) { margin: 0 0 12px; white-space: normal; }
  .prose :global(:last-child) { margin-bottom: 0; }
  .prose :global(h1), .prose :global(h2), .prose :global(h3), .prose :global(h4), .prose :global(h5), .prose :global(h6) { margin: 18px 0 8px; font-size: 1.05em; font-weight: 600; line-height: 1.4; }
  .prose :global(ul) { list-style: disc; padding-left: 24px; margin: 8px 0 12px; }
  .prose :global(ol) { list-style: decimal; padding-left: 24px; margin: 8px 0 12px; }
  .prose :global(li + li) { margin-top: 4px; }
  .prose :global(strong) { font-weight: 600; }
  .prose :global(a) { text-decoration: underline; text-underline-offset: 3px; }
  .prose :global(pre) { max-width: 100%; overflow: auto; padding: 12px; margin: 10px 0; background: var(--muted); border-radius: 6px; white-space: pre; }
  .prose :global(code) { font-family: var(--font-mono); font-size: .875em; }
  .prose :global(blockquote) { padding-left: 14px; border-left: 2px solid var(--border); color: var(--muted-foreground); }
  .user { align-self: flex-end; max-width: min(560px, 86%); padding: 12px 16px; background: var(--muted); border-radius: 12px; }
  .assistant { display: grid; gap: 12px; }
  header { display: flex; align-items: center; gap: 8px; color: var(--muted-foreground); font-size: 11px; }
  .avatar { display: grid; place-items: center; width: 22px; height: 22px; border-radius: 6px; background: var(--muted); color: var(--foreground); font-size: 12px; }
  .caption { display: block; margin-top: 6px; color: var(--muted-foreground); font-size: 11px; }
  details { min-width: 0; font-size: 12px; color: var(--muted-foreground); }
  summary { cursor: pointer; }
  .error { color: var(--destructive); font-size: 12px; }
  .warning { color: var(--foreground); font-size: 12px; }
</style>
