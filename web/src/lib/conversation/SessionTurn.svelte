<script lang="ts">
  import ChevronRight from '@lucide/svelte/icons/chevron-right';
  import { clock } from '#lib/clock.svelte';
  import type { WorkspaceTab } from '#lib/shell/types';
  import type { Conversation } from '#lib/skgo/conversation/types';
  import type { SessionTurn } from './sessionTurns';
  import { runEvents, sessionTurnSeconds } from './sessionTurns';
  import { turnPresentation } from './activity';
  import ConversationActivity from './ConversationActivity.svelte';
  import ConversationMessage from './ConversationMessage.svelte';
  import RunEvent from './RunEvent.svelte';
  let { project, record, turn, runStatuses = {} }: { project:string; record:Conversation; turn:SessionTurn; runStatuses?:Record<string,WorkspaceTab['status']> } = $props();
  const running = $derived(record.active_request === turn.request.id);
  const failed = $derived(turn.request.status === 'failed' || !!turn.request.error);
  const stopped = $derived(turn.request.status === 'canceled');
  const notSent = $derived(turn.request.status === 'unprocessed');
  const seconds = $derived(sessionTurnSeconds(turn,running ? clock.now : undefined));
  const duration = $derived(seconds === undefined ? '' : seconds < 60 ? `${seconds}s` : `${Math.floor(seconds / 60)}m ${seconds % 60}s`);
  const label = $derived(notSent ? 'Delivery unconfirmed' : turn.request.status === 'accepted' ? 'Queued' : `${running ? 'Working' : failed ? 'Failed' : stopped ? 'Stopped' : 'Worked'}${duration ? ` ${failed || stopped ? 'after' : 'for'} ${duration}` : ''}`);
  const messages = $derived(turn.messages.filter(message => !(message.type === 'user' && message.requestID === turn.request.id)));
  const presentation = $derived(turnPresentation(messages, running));
  const launches = $derived(turn.messages.flatMap(message => Object.values(runEvents(message, record, turn.request)).flat()));
</script>

<section class="turn" aria-label={`Turn ${turn.ordinal}`}>
  {#each turn.messages.filter(message => message.type === 'user' && message.requestID === turn.request.id) as message (message.id)}
    <ConversationMessage {message} revision={record.revision} provider="Agent" model="" location="" compact />
  {/each}
  {#if presentation.activity.length}
    <details class="work">
      <summary><span>{label}</span><ChevronRight size={15} class="chevron" /></summary>
      <div class="work-body"><ConversationActivity items={presentation.activity} /></div>
    </details>
  {:else if running || failed || stopped || notSent || turn.request.status === 'accepted'}
    <p class="status" role="status">{label}</p>
  {/if}
  {#if notSent}<p class="status">Delivery of this message was not confirmed.</p>{/if}
  {#each presentation.results as item (item.id)}
    <ConversationMessage message={item.message} revision={record.revision} provider="Agent" model="" location="" compact />
  {/each}
  {#each launches as run (run.id)}<RunEvent {project} {run} state={runStatuses[run.id]} />{/each}
  {#each messages.filter(message => message.error || message.retry) as message (message.id)}
    <ConversationMessage message={{ ...message, content: [] }} revision={record.revision} provider="Agent" model="" location="" compact />
  {/each}
  {#if turn.request.error}<p class="error" role="alert">{turn.request.error}</p>{/if}
</section>

<style>
  .turn { display:flex; flex-direction:column; gap:20px; min-width:0; }
  .work { min-width:0; }
  summary { display:flex; align-items:center; gap:6px; width:fit-content; max-width:100%; color:var(--muted-foreground); font:400 15px/22px var(--font-sans); cursor:pointer; list-style:none; border-radius:4px; }
  summary::-webkit-details-marker { display:none; }
  summary :global(.chevron) { flex:none; transition:transform .1s; }
  .work[open]>summary :global(.chevron) { transform:rotate(90deg); }
  summary:focus-visible { outline:2px solid var(--ring); outline-offset:3px; }
  .work-body { margin-top:12px; border-top:1px solid var(--border); padding-top:20px; }
  .status { margin:0; color:var(--muted-foreground); font:13px/20px var(--font-sans); }
  .error { margin:0; color:var(--destructive); font-size:13px; }
  @media(prefers-reduced-motion:reduce) { summary :global(.chevron) { transition:none; } }
</style>
