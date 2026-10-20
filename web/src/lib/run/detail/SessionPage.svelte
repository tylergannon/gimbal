<script lang="ts">
 import { tick, untrack } from 'svelte';
 import { navigateOnUnauthorized } from '#lib/auth/unauthorized';
 import { clock } from '../../clock.svelte';
 import type { AgentCall } from '../../workflow/types';
 import { usageOf, type RunObservation, type RunSnapshot, type TurnRow } from '../../observation';
 import { turnOutcome } from '../watchers';
 import { environmentSnapshot } from '../../environments.remote';
 import { harnessName } from '../../conversation/names';
 import ConversationTranscript from '../../conversation/ConversationTranscript.svelte';
 import PromptAndContext from './PromptAndContext.svelte';
 import Payload from './Payload.svelte';
 import SteerBox from './SteerBox.svelte';
 import RunQuestions from '../RunQuestions.svelte';
 import UsageTable from './UsageTable.svelte';
 import Pip from '../Pip.svelte';
 import type { WatcherRow } from './SessionDetail.svelte';
 let { project, snapshot, observation, revision = 0, sessionID, definition, watchers = [], onscope }: {
  project?: string; snapshot: RunSnapshot; observation?: RunObservation; revision?: number; sessionID: string;
  definition?: AgentCall; watchers?: WatcherRow[]; onscope?: (key: string) => void;
 } = $props();
 const session = $derived(snapshot.sessions[sessionID]);
 const provider = $derived(harnessName((session?.adapter ?? '').replace(/^\*/, '').replace(/\.adapter$/i, '')));
 const live = $derived(snapshot.run.status === 'running');
 const turns = $derived(Object.values(snapshot.turns).filter(row => row.session === sessionID).sort((a,b) => a.started-b.started));
 const running = $derived(turns.find(row => !row.ended));
 const interaction = $derived(live ? Object.values(snapshot.interactions).filter(row => row.session === sessionID && ['pending','active','unresolved'].includes(row.status)).sort((a,b) => b.ordinal-a.ordinal)[0] : undefined);
 let picked = $state('');
 const chosen = $derived(turns.find(row => row.id === picked) ?? running ?? turns.at(-1));
 const history = $derived(!!chosen && !!running && chosen.id !== running.id);
 // Projection objects mutate in place. Read the revision at the message-list boundary.
 const messages = $derived.by(() => { revision; return Object.values(chosen && observation ? observation.state(chosen.id)?.message ?? {} : {}).flat(); });
 const messageRevisions = $derived.by(() => {
  revision;
  return Object.fromEntries(messages.map(message => [message.id,
   message.type === 'assistant' && observation && chosen ? observation.messageRevision(chosen.id, message.id) : revision]));
 });
 let scroller = $state<HTMLDivElement>();
 let scrollTurn: string | undefined;
 $effect.pre(() => {
  revision;
  const id = chosen?.id;
  const target = scroller;
  if (!target) return;
  const changed = scrollTurn !== id;
  const follow = changed || untrack(() => target.scrollHeight - target.clientHeight - target.scrollTop < 48);
  scrollTurn = id;
  if (follow) void tick().then(() => { target.scrollTop = changed && history ? 0 : target.scrollHeight; });
 });
 const totalUsage = $derived(snapshot.totals.sessions[sessionID]?.all ?? usageOf(undefined));
 const environmentQuery = $derived(project ? environmentSnapshot(project) : undefined);
 const inventory = $derived(environmentQuery?.current);
 const target = $derived(inventory?.environments.find(item => item.environment.id === (snapshot.run.task_environment || 'local')));
 const machine = $derived(inventory?.machines.find(item => item.id === target?.environment.machine));
 const canSteer = $derived(!project || target?.capabilities.steer === true);
 const steerReason = $derived(!live ? 'This run has ended. Steering is closed.' : !running ? 'Between turns. Steering is available while this agent is working.' : environmentQuery?.error ? 'Could not check steering availability. Retry below.' : project && !inventory ? 'Checking steering availability…' : !canSteer ? "This route can’t be steered." : '');
 const duration = (turn: TurnRow) => { const seconds = Math.floor(Math.max(0, turn.ended ? turn.duration || turn.ended-turn.started : clock.now-turn.started)/1000); return `${Math.floor(seconds/60)}m ${String(seconds%60).padStart(2,'0')}s`; };
 const ordinal = (turn: TurnRow) => turns.findIndex(item => item.id === turn.id)+1;
 const hasProse = $derived(messages.some(message => message.type === 'assistant' && (message.content ?? []).some((part: {type?: string; text?: string}) => part.type === 'text' && !!part.text?.trim())));
</script>
<div class="session-page">
 {#if !session}<p class="empty">Session <code>{sessionID}</code> was not found in this run.</p>
 {:else}
  <section class="agent-chat" aria-label="Agent chat">
   <header class="agent-heading"><span class="avatar" aria-hidden="true">{session.name.slice(0,1)}</span><div><h1>{definition?.role || session.name}</h1><p>{provider} · {session.model}{session.effort ? ` · ${session.effort}` : ''} · {chosen?.scope || session.scope || 'root'}{#if chosen} · Turn {ordinal(chosen)}{/if}</p></div></header>
   <div class="conversation" bind:this={scroller}>
    {#if history}<p class="history-note">Reading a recorded turn. Steering still goes to the running turn. <button onclick={() => picked = ''}>Return to live</button></p>{/if}
    {#if chosen}
     <svelte:boundary>
      <div class="prose"><ConversationTranscript {messages} {revision} {messageRevisions} {provider} model={session.model} location={machine?.name || snapshot.run.task_environment || 'Host not recorded'} />
      {#if !hasProse}<p class="empty">No assistant prose recorded for this turn yet.</p>{/if}
      {#if chosen.error}<p class="turn-error">{chosen.error}</p>{/if}
      <p class="working-line"><Pip state={chosen.unresolved ? 'needs-you' : !chosen.ended && live ? 'running' : chosen.error && !chosen.interrupted ? 'failed' : 'ended'} /><span>{chosen.unresolved ? 'Delivery unresolved' : !chosen.ended && live ? `${session.name} is working…` : `Turn ${ordinal(chosen)} · ${turnOutcome(chosen)}`} · {duration(chosen)}</span></p>
      </div>
      {#snippet failed(error)}<p class="empty" role="alert">Could not render this turn's transcript: {error instanceof Error ? error.message : String(error)}</p>{/snippet}
     </svelte:boundary>
    {:else}<p class="empty">This session has no recorded turns.</p>{/if}
   </div>
   {#if interaction}<RunQuestions run={snapshot.run.id} question={interaction} composer target={session.name} turn={running?.id} />
   {:else}<SteerBox run={snapshot.run.id} session={sessionID} ended={!running || !live} disabled={!canSteer} reason={steerReason} />{/if}
   {#if environmentQuery?.error}<p class="empty steering-error" role="alert"><button onclick={() => environmentQuery?.refresh().catch(navigateOnUnauthorized)}>Retry steering availability</button></p>{/if}
  </section>
  <aside aria-label="Agent details">
   <h2>Session</h2>
   <dl><dt>Run</dt><dd>{snapshot.run.name}</dd><dt>Role</dt><dd>{definition?.role || session.name}</dd><dt>Scope</dt><dd><button class="link" onclick={() => onscope?.(chosen?.scope || session.scope)}>{chosen?.scope || session.scope || 'root'}</button></dd><dt>Provider</dt><dd>{provider}</dd><dt>Host</dt><dd>{machine?.name || snapshot.run.task_environment || 'Not recorded'}</dd><dt>Worktree</dt><dd class="path">{session.workdir || 'Not recorded'}</dd><dt>Model</dt><dd>{session.model}</dd>{#if session.effort}<dt>Effort</dt><dd>{session.effort}</dd>{/if}</dl>
   <label class="turn-choice">Turn<select aria-label="Recorded turn" bind:value={() => chosen?.id ?? '', (id) => picked = id}>{#each turns as turn (turn.id)}<option value={turn.id}>Turn {ordinal(turn)} · {turnOutcome(turn)}</option>{/each}</select></label>
   {#if history}<button class="link" onclick={() => picked = ''}>Return to live</button>{/if}
   <UsageTable usage={totalUsage} title="Session total" />
   {#if chosen}
    <details class="facts-section"><summary>Prompt & context</summary><PromptAndContext turn={chosen} {snapshot} {onscope} /></details>
    <details class="facts-section"><summary>Structured output</summary>{#if chosen.result}<Payload label={chosen.output_type || 'result'} text={chosen.result} />{:else}<p class="empty">No structured output recorded.</p>{/if}</details>
    <details class="facts-section"><summary>Turn usage</summary>{#each Object.entries(snapshot.turn_usage[chosen.id] ?? {}) as [model, usage] (model)}<UsageTable {usage} title={model} />{:else}<p class="empty">No usage recorded for this turn.</p>{/each}</details>
   {/if}
   <section class="steers"><h2>Steers</h2>{#each session.steers ?? [] as item, index (`${item.created}-${index}`)}<div class="steer-record"><p>{item.message}</p><span>{item.landed ? 'Landed' : 'Dropped'}{item.source ? ` · ${item.source}` : ''} · {new Date(item.created).toLocaleTimeString(undefined,{hour:'2-digit',minute:'2-digit'})}</span></div>{:else}<p class="empty">No steers recorded.</p>{/each}</section>
   {#if watchers.length}<details class="facts-section"><summary>Supervisors ({watchers.length})</summary>{#each watchers as watcher (watcher.session)}<p class="watcher"><Pip state={watcher.pip} />{watcher.role} · {watcher.verdict}</p><Payload label="result" text={watcher.result} />{/each}</details>{/if}
  </aside>
 {/if}
</div>
<style>
 .session-page { display: flex; flex: 1; height: 100%; min-width: 0; min-height: 0; color: var(--foreground); background: var(--background); }
 .agent-chat { display: flex; flex: 1; flex-direction: column; min-width: 0; min-height: 0; }
 .agent-heading { display: flex; align-items: center; gap: 9px; padding: 16px 24px; flex: none; } .avatar { display: grid; place-items: center; width: 26px; height: 26px; border-radius: 7px; color: var(--foreground); background: var(--muted); font-size: 12px; }
 h1 { font-size: 13px; font-weight: 500; margin: 0; } .agent-heading p { margin: 3px 0 0; font-size: 10px; color: var(--muted-foreground); }
 .conversation { flex: 1; min-height: 0; overflow: auto; padding: 16px max(24px, calc((100% - 680px)/2)); } .prose { display: flex; flex-direction: column; gap: 20px; }
 .working-line { display: flex; align-items: center; gap: 8px; margin: 0; font-size: 11px; color: var(--muted-foreground); }
 aside { flex: 0 0 240px; width: 240px; box-sizing: border-box; padding: 20px 18px; border-left: 1px solid var(--border); overflow: auto; font-size: 11px; }
 h2 { margin: 0 0 12px; font-size: 11px; font-weight: 500; } dl { display: grid; grid-template-columns: 55px minmax(0,1fr); gap: 8px 10px; margin: 0 0 18px; } dt { color: var(--muted-foreground); } dd { margin: 0; overflow-wrap: anywhere; } .path { font-family: var(--font-mono); font-size: 10px; }
 .link { padding: 0; background: transparent; border: 0; color: inherit; font: inherit; cursor: pointer; text-align: left; } .link:hover { text-decoration: underline; } .turn-choice { display: grid; gap: 6px; margin-bottom: 18px; color: var(--muted-foreground); } select { min-width: 0; width: 100%; border: 1px solid var(--border); border-radius: 5px; padding: 5px; background: var(--background); color: var(--foreground); font: inherit; }
 .facts-section, .steers { margin-top: 22px; } .facts-section summary { cursor: pointer; margin-bottom: 10px; } .steer-record { margin-bottom: 12px; } .steer-record p { white-space: pre-wrap; overflow-wrap: anywhere; margin: 0 0 4px; line-height: 1.6; } .steer-record span { color: var(--muted-foreground); font-size: 10px; }
 .empty { color: var(--muted-foreground); font-size: 12px; line-height: 1.6; margin: 0; } .history-note { font-size: 11px; color: var(--muted-foreground); margin: 0 0 18px; } .history-note button { font: inherit; color: var(--foreground); border: 0; background: transparent; cursor: pointer; text-decoration: underline; } .turn-error { color: var(--destructive); font-size: 12px; white-space: pre-wrap; } .watcher { display: flex; align-items: center; gap: 6px; }
 @media(max-width:720px) { aside { flex-basis: 190px; width: 190px; padding: 16px 12px; } .conversation { padding: 16px; } .agent-heading { padding: 12px 16px; } }
 @media(max-width:540px) { .session-page { flex-direction: column; overflow: auto; } .agent-chat { flex: none; min-height: 350px; } .conversation { overflow: visible; } aside { width: 100%; flex: none; border-left: 0; border-top: 1px solid var(--border); } }
</style>
