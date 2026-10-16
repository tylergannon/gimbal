<script lang="ts">
  import { clock } from '#lib/clock.svelte';
  import type { WorkspaceTab } from '#lib/shell/types';
  import type { Conversation } from '#lib/skgo/conversation/types';
  import type { SessionTurn } from './sessionTurns';
  import { runEvents } from './sessionTurns';
  import ConversationMessage from './ConversationMessage.svelte';
  import Pip from '#lib/run/Pip.svelte';
  let { project, record, turn, runStatuses = {} }: { project:string; record:Conversation; turn:SessionTurn; runStatuses?:Record<string,WorkspaceTab['status']> } = $props();
  const running = $derived(record.active_request === turn.request.id);
  const failed = $derived(turn.request.status === 'failed' || !!turn.request.error);
  const times = $derived(turn.messages.flatMap(message => [message.time?.completed]).filter(value => typeof value === 'number'));
  const ended = $derived(times.length ? Math.max(...times) : undefined);
  const seconds = $derived(running ? Math.max(0,Math.floor((clock.now-turn.request.created)/1000)) : ended ? Math.max(0,Math.floor((ended-turn.request.created)/1000)) : undefined);
  const events = $derived(turn.messages.flatMap(message => message.content ?? []).filter(part => part.type === 'tool' || part.type === 'reasoning').length);
</script>
<section class="turn" aria-label={`Turn ${turn.ordinal}`}>
 {#each turn.messages.filter(message => message.type === 'user' && message.requestID === turn.request.id) as message (message.id)}<ConversationMessage {message} revision={record.revision} provider="Agent" model="" location="" compact />{/each}
 <div class="turn-head"><b>Turn {turn.ordinal}</b><span>·</span><Pip state={running ? 'running' : failed ? 'failed' : turn.request.status === 'canceled' ? 'cancelled' : turn.request.status === 'accepted' ? 'queued' : 'ended'} /><span>{running ? 'running' : failed ? 'failed' : turn.request.status === 'canceled' ? 'stopped' : turn.request.status === 'accepted' ? 'queued' : 'ended'}</span>{#if seconds !== undefined}<span>·</span><span>{Math.floor(seconds/60)}m {String(seconds%60).padStart(2,'0')}s</span>{/if}<span>·</span><span>{events} {events === 1 ? 'event' : 'events'}</span></div>
 {#each turn.messages.filter(message => !(message.type === 'user' && message.requestID === turn.request.id)) as message (message.id)}<ConversationMessage {message} revision={record.revision} provider="Agent" model="" location="" compact {project} {runStatuses} runs={runEvents(message,record,turn.request)} />{/each}
 {#if turn.request.error}<p class="error">{turn.request.error}</p>{/if}
</section>
<style>.turn { display:flex; flex-direction:column; gap:14px; min-width:0; } .turn-head { display:flex; flex-wrap:wrap; align-items:center; gap:7px; color:var(--muted-foreground); font:12px/16px var(--font-sans); } b { font-weight:500; color:var(--foreground); } .error { margin:0; color:var(--destructive); font-size:13px; }</style>
