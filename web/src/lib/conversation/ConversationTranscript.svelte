<script lang="ts">
  import type { RequestState } from '#lib/skgo/conversation/types';
  import type { JSONObject } from '#lib/sessionstate';
  import ToolCall from '#lib/run/detail/ToolCall.svelte';
  let { messages, provider, model, location, requests = [] }: { messages: JSONObject[]; provider: string; model: string; location: string; requests?: RequestState[] } = $props();
  const digest = (part: JSONObject) => {
    const input = part.state?.input;
    if (!input || typeof input !== 'object') return '';
    return input.file_path ?? input.path ?? input.command ?? input.description ?? '';
  };
</script>

{#each messages as message, index (message.id)}
  {#if message.type === 'user'}
    <article class="user" data-request-id={message.requestID}><p>{message.text}</p>{#if message.steered}<span class="caption">Sent while {provider} was working</span>{/if}</article>
  {:else if message.type === 'assistant'}
    <article class="assistant"><header><span class="avatar" aria-hidden="true">{provider.slice(0, 1)}</span><span>{provider} · {model} · {location}</span></header>
      {#each message.content ?? [] as part, i (`${message.id}/${i}`)}
        {#if part.type === 'text'}<p>{part.text}</p>
        {:else if part.type === 'reasoning'}<details class="reasoning"><summary>Thinking</summary><p>{part.text}</p></details>
        {:else if part.type === 'tool'}<details class="tool"><summary><span>{part.name}</span><code title={String(digest(part))}>{digest(part)}</code></summary><ToolCall {part} /></details>{/if}
      {/each}
    </article>
    {@const stopped = requests.find(request => request.status === 'canceled' && String(message.id).startsWith(`${request.id}/`))}
    {#if stopped && !String(messages[index + 1]?.id ?? '').startsWith(`${stopped.id}/`)}<p class="caption" role="status">Stopped</p>{/if}
  {:else if message.type === 'connection'}<p class="caption">{message.message}</p>{/if}
{/each}

<style>
  article { min-width: 0; }
  p { margin: 0; white-space: pre-wrap; overflow-wrap: anywhere; font-size: 15px; line-height: 1.65; }
  .user { align-self: flex-end; max-width: min(560px, 86%); padding: 12px 16px; background: var(--muted); border-radius: 18px; }
  .assistant { display: grid; gap: 12px; }
  header { display: flex; align-items: center; gap: 8px; color: var(--muted-foreground); font-size: 11px; }
  .avatar { display: grid; place-items: center; width: 22px; height: 22px; border-radius: 6px; background: var(--muted); color: var(--foreground); font-size: 12px; }
  .caption { display: block; margin-top: 6px; color: var(--muted-foreground); font-size: 11px; }
  details { min-width: 0; font-size: 12px; color: var(--muted-foreground); }
  summary { cursor: pointer; }
  .tool summary { display: flex; align-items: center; gap: 8px; }
  .tool summary::before { content: '›'; }
  .tool[open] summary::before { content: '⌄'; }
  code { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font: 11px var(--font-mono); }
  .reasoning p { margin-top: 8px; font-size: 13px; }
</style>
