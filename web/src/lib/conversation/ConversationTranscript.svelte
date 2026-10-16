<script lang="ts">
  import type { RequestState } from '#lib/skgo/conversation/types';
  import type { JSONObject } from '#lib/sessionstate';
  import ConversationMessage from './ConversationMessage.svelte';
  let { messages, revision = 0, messageRevisions, provider, model, location, requests = [], compact = false }: { messages: JSONObject[]; revision?: number; messageRevisions?: Record<string, number>; provider: string; model: string; location: string; requests?: RequestState[]; compact?: boolean } = $props();
</script>

{#each messages as message, index (message.id)}
  <ConversationMessage {message} revision={messageRevisions?.[message.id] ?? revision} {provider} {model} {location} {compact} />
  {@const stopped = requests.find(request => request.status === 'canceled' && String(message.id).startsWith(`${request.id}/`))}
  {#if stopped && !String(messages[index + 1]?.id ?? '').startsWith(`${stopped.id}/`)}<p class="caption" role="status">Stopped</p>{/if}
{/each}
<style>.caption { margin: 6px 0 0; color: var(--muted-foreground); font-size: 11px; }</style>
