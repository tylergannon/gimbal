<script lang="ts">
  import { clock } from '#lib/clock.svelte';
  import type { JSONObject } from '#lib/sessionstate';
  import type { RunObservation, RunSnapshot, TurnRow } from '#lib/observation';
  import ConversationTranscript from '#lib/conversation/ConversationTranscript.svelte';
  import Payload from './detail/Payload.svelte';
  import Pip from './Pip.svelte';
  let { turn, ordinal, snapshot, observation, revision }: { turn: TurnRow; ordinal: number; snapshot: RunSnapshot; observation: RunObservation; revision: number } = $props();
  const session = $derived(snapshot.sessions[turn.session]);
  const structured = (value: string | undefined) => { if (!value) return false; try { const parsed = JSON.parse(value); return parsed !== null && typeof parsed === 'object'; } catch { return false; } };
  const recorded = $derived.by(() => { revision; return Object.values(observation.state(turn.id)?.message ?? {}).flat(); });
  const drafts = $derived(recorded.flatMap(message => message.content ?? []).filter((part: JSONObject) => part.type === 'text' && structured(part.text) && part.text !== turn.result));
  const toolCount = $derived(recorded.flatMap(message => message.content ?? []).filter((part: JSONObject) => part.type === 'tool').length);
  const messages = $derived.by(() => {
    revision;
    // The initial user event is the prompt shown under Prompt & context.
    // Structured result echoes belong to the one result disclosure, rather
    // than appearing repeatedly as prose while a provider finalizes it.
    return recorded.filter((message, index) => !(index === 0 && message.type === 'user')).map((message): JSONObject => ({ ...message,
      content: message.content?.filter((part: { type?: string; text?: string }) => {
        return part.type !== 'text' || !structured(part.text);
      }),
    }));
  });
  const messageRevisions = $derived.by(() => { revision; return Object.fromEntries(messages.map(message => [message.id, revision + observation.messageRevision(turn.id, message.id)])); });
  const seconds = $derived(Math.max(0, Math.floor((turn.duration || (turn.ended || clock.now) - turn.started) / 1000)));
</script>
<article class="turn" aria-label={`Turn ${ordinal}`}>
  <div class="turn-head"><b>Turn {ordinal}</b><span>·</span><span>{turn.scope || session?.name || 'root'}</span><span>·</span><Pip state={turn.unresolved ? 'needs-you' : turn.interrupted ? 'cancelled' : !turn.ended ? 'running' : turn.error && !turn.interrupted ? 'failed' : 'ended'} /><span>{turn.unresolved ? 'delivery unresolved' : !turn.ended ? 'running' : turn.interrupted ? 'stopped' : turn.error ? 'failed' : 'ended'}</span><span>·</span><span>{Math.floor(seconds / 60)}m {String(seconds % 60).padStart(2, '0')}s</span><span>·</span><span>{toolCount} tool calls</span></div>
  <div class="prose"><ConversationTranscript {messages} {revision} {messageRevisions} provider={session?.name || 'Agent'} model={session?.model || ''} location={session?.workdir || ''} compact /></div>
  {#if !messages.length}<p class="empty">{turn.events_unavailable ? 'Transcript events are unavailable for this turn.' : turn.ended ? 'No transcript recorded for this turn.' : 'Waiting for transcript events…'}</p>{/if}
  {#if turn.error}<p class="error">{turn.error}</p>{/if}
  {#if turn.observation_error}<p class="empty">{turn.observation_error}</p>{/if}
  {#if drafts.length}<details><summary>Structured response drafts · {drafts.length} events</summary>{#each drafts as draft, index (index)}<Payload label={`draft ${index+1}`} text={draft.text} />{/each}</details>{/if}
  {#if turn.result}<details open><summary>Structured result</summary><Payload label="result" text={turn.result} /></details>{/if}
  <details><summary>Prompt & context</summary><Payload label="prompt" text={turn.prompt} />{#each turn.context ?? [] as item (`${item.scope}:${item.key}`)}<p class="empty">{item.key} · {item.scope || 'root'} · {item.complete ? 'complete' : 'shortened or omitted'}</p>{/each}</details>
</article>
<style>
  .turn { display: flex; flex-direction: column; gap: 12px; min-width: 0; } .turn-head { display: flex; align-items: center; flex-wrap: wrap; gap: 8px; color: var(--muted-foreground); font: 12px/16px var(--font-sans); } b { font-weight: 500; color: var(--foreground); } .prose { display: flex; flex-direction: column; gap: 18px; } details { font-size: 12px; color: var(--muted-foreground); } summary { cursor: pointer; } .empty { margin: 0; font-size: 12px; line-height: 1.6; color: var(--muted-foreground); } .error { margin: 0; font-size: 13px; white-space: pre-wrap; color: var(--destructive); }
</style>
