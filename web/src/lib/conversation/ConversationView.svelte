<script lang="ts">
  import { onMount } from 'svelte';
  import { navigateOnUnauthorized } from '#lib/auth/unauthorized';
  import { errorMessage } from '#lib/errors';
  import ArrowUp from '@lucide/svelte/icons/arrow-up';
  import Square from '@lucide/svelte/icons/square';
  import Search from '@lucide/svelte/icons/search';
  import MoreHorizontal from '@lucide/svelte/icons/ellipsis';
  import GitBranch from '@lucide/svelte/icons/git-branch';
  import Pip from '#lib/run/Pip.svelte';
  import SessionBinding from './SessionBinding.svelte';
  import SessionTurn from './SessionTurn.svelte';
  import RunEvent from './RunEvent.svelte';
  import { sessionTurns, runEvents } from './sessionTurns';
  import { conversationStatus } from '#lib/workspace/conversations';
  import LoaderCircle from '@lucide/svelte/icons/loader-circle';
  import { sendConversationMessage, stopConversationRequest, projectConversations } from '../../routes/conversation.remote';
  import type { WorkspaceTab } from '#lib/shell/types';
  import type { Conversation } from '#lib/skgo/conversation/types';
  import type { ProjectionState } from '#lib/sessionstate';
  import ConversationTranscript from './ConversationTranscript.svelte';
  import ChatActions from './ChatActions.svelte';
  import { harnessName, modelName } from './names';

  let { project, current, active = true, runStatuses = {}, onchanged }: { project: string; current: Conversation; active?: boolean; runStatuses?:Record<string,WorkspaceTab['status']>; onchanged?: () => void } = $props();
  const inventory = $derived(projectConversations(project));
  const record = $derived(current);
  const form = $derived(sendConversationMessage.for(`${project}/${current.id}`));
  const projection = $derived(record.state ? JSON.parse(record.state) as ProjectionState : undefined);
  const messages = $derived(projection?.message[record.id] ?? []);
  const grouped = $derived(sessionTurns(record,messages));
  const associated = $derived(new Set(grouped.turns.flatMap(turn => turn.messages.flatMap(message => Object.values(runEvents(message,record,turn.request)).flat().map(run => run.id)))));
  const unassigned = $derived(record.runs.filter(run => !associated.has(run.id)));
  let finding = $state(false);
  let findText = $state('');
  const working = $derived(!!record.active_request);
  const stopping = $derived(record.status === 'stopping');
  const draft = $derived(form.fields.message.value() ?? '');
  const hasMessage = $derived(!!draft.trim());
  const canStop = $derived(working && !stopping);
  const machine = $derived(record.placement?.machine.name ?? 'This Mac');
  const uncertain = $derived(record.status === 'unresolved' || record.contact === 'unreachable');
  const canSteer = $derived(record.capabilities?.steer ?? !record.placement);
  const provider = $derived(harnessName(record.provider));
  let actionError = $state('');
  let stopPending = $state(false);
  let requestID = $state('');
  let created = $state(0);
  let scroller = $state<HTMLDivElement>();
  // DOM-only following; scrolling up leaves the reader's position intact.
  let follow = $state(true);
  $effect(() => { record.revision; if (follow) scroller?.scrollTo({ top: scroller.scrollHeight }); });
  function find() { const match = [...(scroller?.querySelectorAll('p,code') ?? [])].find(element => element.textContent?.toLowerCase().includes(findText.toLowerCase())); match?.scrollIntoView({block:'center'}); }

  async function stop() {
    if (!canStop || stopPending) return;
    stopPending = true;
    actionError = '';
    try {
      await stopConversationRequest({ conversation: record.id, id: crypto.randomUUID(), created: Date.now(), target: record.active_request });
      await inventory.refresh();
      onchanged?.();
    } catch (error) {
      if (!navigateOnUnauthorized(error)) actionError = errorMessage(error);
    } finally { stopPending = false; }
  }
  function keydown(event: KeyboardEvent) {
    if (event.isComposing) return;
    if (event.key === 'Enter' && !event.shiftKey) {
      event.preventDefault();
      if (hasMessage && !form.pending && !stopping) (event.currentTarget as HTMLTextAreaElement).form?.requestSubmit();
    } else if (event.key === 'Escape' && canStop) { event.preventDefault(); void stop(); }
  }
  onMount(() => {
    requestID = crypto.randomUUID(); created = Date.now();
    const command = (event: Event) => { if (!active) return; const value = (event as CustomEvent).detail.command; if (value === 'stopResponse') void stop(); };
    const shortcut = (event: KeyboardEvent) => {
      if (active && (event.metaKey || event.ctrlKey) && event.key === '.' && canStop) { event.preventDefault(); void stop(); }
    };
    void inventory.catch(navigateOnUnauthorized);
    window.addEventListener('gimbal-workspace-command', command);
    window.addEventListener('keydown', shortcut);
    return () => { window.removeEventListener('gimbal-workspace-command', command); window.removeEventListener('keydown', shortcut); };
  });
</script>

<section class="chat" aria-label={record.title}>
  <div class="transcript" bind:this={scroller} onscroll={() => { if (scroller) follow = scroller.scrollHeight - scroller.scrollTop - scroller.clientHeight < 64; }}>
    <header class="session-head">
      <div class="title-row"><h1><button title="Rename session" onclick={() => window.dispatchEvent(new CustomEvent('gimbal-workspace-command',{detail:{command:'renameTab'}}))}>{record.title}</button></h1><span class="status"><Pip state={conversationStatus(record)} />{working ? stopping ? 'Stopping' : 'Running' : record.status === 'unresolved' ? 'Needs you' : record.status === 'canceled' ? 'Stopped' : record.error ? 'Failed' : !record.live ? 'Ended' : 'Ready'}{#if working}<span class="quiet">turn {grouped.turns.find(turn => turn.request.id === record.active_request)?.ordinal ?? grouped.turns.length}</span>{/if}</span></div>
      <div class="bindings">{#if active}<SessionBinding {project} {record} {onchanged} />{:else}<span class="quiet">{provider} · {modelName(record.model)}</span>{/if}<details class="binding"><summary title={`${record.branch || record.worktree} · ${machine}`}><GitBranch size={14} /><span class="binding-label">{record.branch || 'Shared checkout'} · {machine}</span><span>⌄</span></summary><div class="binding-menu"><b>Working directory</b><code>{record.worktree}</code><span>Location is fixed for this session.</span></div></details><button class="icon" aria-label="Find in session" onclick={() => finding = !finding}><Search size={15} /></button><details class="more"><summary aria-label="More session actions"><MoreHorizontal size={17} /></summary><div class="binding-menu"><button onclick={() => window.dispatchEvent(new CustomEvent('gimbal-workspace-command',{detail:{command:'renameTab'}}))}>Rename session…</button>{#if active}<ChatActions {project} current={record} {onchanged} />{/if}<code>{record.id}</code></div></details></div>
      {#if finding}<form class="find" onsubmit={event => { event.preventDefault(); find(); }}><input aria-label="Find in session" placeholder="Find in session…" bind:value={findText} /><button>Find</button></form>{/if}
    </header>
    {#if messages.length}
      {#if grouped.history.length}<ConversationTranscript messages={grouped.history} {provider} model="" location="" compact />{/if}
      {#each grouped.turns as turn (turn.request.id)}<SessionTurn {project} {record} {turn} {runStatuses} />{/each}
    {:else if record.messages.length}
      {#each record.messages as message, i (i)}<article class:user={message.role === 'user'}><p>{message.text}</p></article>{/each}
    {:else}
      <div class="empty"><h2>{record.title}</h2><p>Message {provider} to get started.</p></div>
    {/if}
    {#if unassigned.length}<section class="unassigned" aria-label="Runs from this session"><p class="quiet">Runs from this session · source turn unavailable</p>{#each unassigned as run (run.id)}<RunEvent {project} {run} state={runStatuses[run.id]} />{/each}</section>{/if}
    {#if working}<p class="activity" role="status">{uncertain ? 'Contact lost. The request may still be running; Stop is still available.' : stopping ? 'Stopping…' : `${provider} is working…`}</p>
    {:else if record.status === 'canceled' && !messages.length}<p class="activity" role="status">Stopped</p>{/if}
    {#if record.cleanup_pending}<p class="notice" role="status">Cleanup is not confirmed. Gimbal retains ownership; use Retry cleanup to confirm it.</p>{/if}
    {#if record.error}<p class="notice" role="alert">{record.error}</p>{/if}
    {#each record.requests ?? [] as request (request.id)}
      {#if request.kind === 'prompt' && request.status === 'accepted'}<p class="activity">Queued · {request.message}</p>
      {:else if request.status === 'unprocessed'}<p class="activity">Unprocessed · {request.message}</p>
      {:else if request.error && request.error !== record.error && request.id !== record.active_request}<p class="notice" role="alert">{request.error}</p>{/if}
    {/each}
    {#if !record.live}<p class="notice">Saved history. Start a new conversation to continue.</p>{/if}
    {#if inventory.error}<p class="notice" role="status">Server disconnected. The request may still be running. <button onclick={() => inventory.refresh()}>Reconnect</button></p>{/if}
  </div>

  <form class="composer" {...form.enhance(async (submitted) => {
    actionError = '';
    try {
      if (await submitted.submit().updates(projectConversations(project))) { submitted.element.reset(); requestID = crypto.randomUUID(); created = Date.now(); onchanged?.(); }
    } catch (error) { if (!navigateOnUnauthorized(error)) actionError = errorMessage(error); }
  })}>
    <input {...form.fields.conversation.as('hidden', record.id)} />
    <input {...form.fields.id.as('hidden', requestID)} />
    <input {...form.fields.created.as('hidden', created)} />
    <div class="mode"><b>{!record.live ? 'Saved history' : working ? canSteer ? 'Steer into current turn' : 'Reply after current turn' : 'Reply'}</b><span>{!record.live ? 'Start a new session to continue' : working ? canSteer ? 'Sends to the running agent now' : 'Queued until this turn ends' : 'Starts the next turn in this session'}</span></div>
    <textarea {...form.fields.message.as('text')} rows={2} onkeydown={keydown} placeholder={working && canSteer ? 'Steer the current turn…' : `Message ${provider}…`} aria-label={`Message ${record.title}`} disabled={!record.live || !!form.pending || stopping || uncertain}></textarea>
    <div class="composer-bottom"><span>{stopping ? 'Stopping…' : !record.live ? 'Saved history' : uncertain ? 'Connection needs attention' : '↵ send · ⇧↵ newline'}</span><div class="actions">{#if working}<button type="button" class="stop" aria-label="Stop response" onclick={stop} disabled={!canStop || stopPending}>{#if stopping || stopPending}<LoaderCircle size={13} class="spin" />{:else}<Square size={10} fill="currentColor" />{/if}Stop turn</button>{/if}<button type="submit" class="send" aria-label="Send message" disabled={!record.live || !hasMessage || !!form.pending || stopping || uncertain}>Send<ArrowUp size={14} /></button></div></div>
    {#each form.fields.allIssues() ?? [] as issue}<p class="notice" role="alert">{issue.message}</p>{/each}
    {#if actionError}<p class="notice" role="alert">{actionError}</p>{/if}
  </form>
</section>

<style>
 .chat { height:100%; display:flex; flex-direction:column; min-width:0; min-height:0; color:var(--foreground); }
 .transcript { flex:1; min-height:0; overflow:auto; overflow-wrap:anywhere; padding:24px max(32px,calc((100% - 860px)/2)); display:flex; flex-direction:column; gap:24px; }
 .session-head { display:flex; flex-direction:column; gap:14px; } .title-row,.bindings { display:flex; align-items:center; flex-wrap:wrap; gap:12px; } h1 { margin:0; font:600 20px/26px var(--font-sans); } h1 button { border:0; padding:0; text-align:left; background:none; color:inherit; font:inherit; cursor:pointer; } .status { display:inline-flex; align-items:center; gap:6px; font-size:13px; } .quiet { color:var(--muted-foreground); font-size:12px; } .binding,.more { position:relative; } .binding summary,.icon { display:flex; align-items:center; gap:8px; padding:6px 10px; border:1px solid var(--border); border-radius:6px; background:var(--background); color:var(--foreground); font:13px/18px var(--font-sans); cursor:pointer; list-style:none; } summary::-webkit-details-marker { display:none; } .binding summary>span { color:var(--muted-foreground); } .binding summary { max-width:330px; } .binding .binding-label { min-width:0; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; color:var(--foreground); } .binding summary :global(svg) { flex:none; } .binding-menu { position:absolute; top:38px; left:0; z-index:20; width:290px; display:grid; gap:10px; padding:14px; border:1px solid var(--border); border-radius:7px; background:var(--background); box-shadow:0 8px 24px #0002; font-size:12px; } .binding-menu code { font:11px var(--font-mono); overflow-wrap:anywhere; } .binding-menu button { color:var(--foreground); } .binding-menu button { background:none; border:0; padding:0; font:inherit; text-align:left; cursor:pointer; } .more summary { display:grid; place-items:center; width:30px; height:30px; cursor:pointer; list-style:none; } .more .binding-menu { left:auto; right:0; } .find { display:flex; gap:8px; } .find input { flex:1; padding:7px; border:1px solid var(--border); border-radius:5px; color:var(--foreground); background:var(--background); } .find button { border:1px solid var(--border); border-radius:5px; color:var(--foreground); background:var(--background); padding:4px 12px; }
 p,h2 { margin:0; } article p { white-space:pre-wrap; font:15px/22px var(--font-sans); } article.user { align-self:flex-end; max-width:86%; border-radius:12px; padding:10px 14px; background:var(--muted); } .empty { margin:auto; text-align:center; color:var(--muted-foreground); } .empty h2 { margin-bottom:8px; font-size:16px; font-weight:500; color:var(--foreground); } .empty p,.activity,.notice { color:var(--muted-foreground); font-size:12px; line-height:18px; } .unassigned { display:grid; gap:10px; } .notice button { color:inherit; background:none; border:0; text-decoration:underline; cursor:pointer; }
 .composer { flex:none; box-sizing:border-box; width:min(860px,calc(100% - 64px)); margin:0 auto 20px; border:1px solid var(--border); border-radius:10px; background:var(--background); overflow:hidden; } .composer:focus-within { border-color:var(--ring); } .mode { display:flex; flex-wrap:wrap; gap:8px; padding:10px 14px 0; font:12px/18px var(--font-sans); } .mode b { font-weight:500; } .mode span { color:var(--muted-foreground); } textarea { box-sizing:border-box; display:block; width:100%; min-height:72px; max-height:35vh; resize:vertical; padding:12px 14px; border:0; outline:none; color:var(--foreground); background:transparent; font:14px/20px var(--font-sans); } .composer-bottom { display:flex; align-items:center; justify-content:space-between; gap:12px; padding:0 12px 10px; color:var(--muted-foreground); font-size:11px; } .actions { display:flex; align-items:center; gap:8px; } .send,.stop { display:inline-flex; align-items:center; justify-content:center; gap:6px; height:30px; padding:0 10px; border:1px solid var(--border); border-radius:6px; color:var(--foreground); background:var(--background); font:12px var(--font-sans); cursor:pointer; } .send { border-color:var(--foreground); background:var(--foreground); color:var(--background); } button:disabled { opacity:.4; cursor:default; } .composer>.notice { padding:0 14px 10px; } :global(.spin) { animation:spin 1s linear infinite; } @keyframes spin { to { transform:rotate(360deg); } } @media(prefers-reduced-motion:reduce) { :global(.spin) { animation:none; } } @media(max-width:700px) { .transcript { padding:20px; } .composer { width:calc(100% - 40px); } }
</style>
