<script lang="ts">
  import { onMount } from 'svelte';
  import { navigateOnUnauthorized } from '#lib/auth/unauthorized';
  import { errorMessage } from '#lib/errors';
  import ArrowUp from '@lucide/svelte/icons/arrow-up';
  import Square from '@lucide/svelte/icons/square';
  import LoaderCircle from '@lucide/svelte/icons/loader-circle';
  import { sendConversationMessage, stopConversationRequest, projectConversations } from '../../routes/conversation.remote';
  import type { Conversation } from '#lib/skgo/conversation/types';
  import type { ProjectionState } from '#lib/sessionstate';
  import ConversationTranscript from './ConversationTranscript.svelte';
  import ChatActions from './ChatActions.svelte';
  import { harnessName, modelName } from './names';

  let { project, current, active = true, onchanged }: { project: string; current: Conversation; active?: boolean; onchanged?: () => void } = $props();
  const inventory = $derived(projectConversations(project));
  const record = $derived(current);
  const form = $derived(sendConversationMessage.for(`${project}/${current.id}`));
  const projection = $derived(record.state ? JSON.parse(record.state) as ProjectionState : undefined);
  const messages = $derived(projection?.message[record.id] ?? []);
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
  $effect(() => { record.revision; scroller?.scrollTo({ top: scroller.scrollHeight }); });

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
  <div class="transcript" bind:this={scroller}>
    <div class="placement"><span title={record.worktree}>{machine} · {record.worktree}{record.branch ? ` · ${record.branch}` : ''}</span>{#if active}<ChatActions {project} current={record} {onchanged} />{/if}</div>
    {#if record.runs.length}
      <section class="runs" aria-label="Workflow runs"><span class="muted">Workflow runs</span>
        {#each record.runs as run (run.id)}<a href={`/projects/${project}/runs/${encodeURIComponent(run.id)}`}>{run.workflow} · {run.status}</a>{/each}
      </section>
    {/if}
    {#if messages.length}
      <ConversationTranscript {messages} requests={record.requests} {provider} model={modelName(record.model)} location={`${record.branch || record.worktree} · ${machine}`} />
    {:else if record.messages.length}
      {#each record.messages as message, i (i)}<article class:user={message.role === 'user'}><p>{message.text}</p></article>{/each}
    {:else}
      <div class="empty"><h2>{record.title}</h2><p>Message {provider} to get started.</p></div>
    {/if}
    {#if working}<p class="activity" role="status">{uncertain ? 'Contact lost. The request may still be running; Stop is still available.' : stopping ? 'Stopping…' : `${provider} is working…`}</p>
    {:else if record.status === 'canceled' && !messages.length}<p class="activity" role="status">Stopped</p>{/if}
    {#if record.cleanup_pending}<p class="notice" role="status">Cleanup is not confirmed. Gimbal retains ownership; use Retry cleanup to confirm it.</p>{/if}
    {#if record.error}<p class="notice" role="alert">{record.error}</p>{/if}
    {#each record.requests ?? [] as request (request.id)}
      {#if request.kind === 'prompt' && request.status === 'accepted'}<p class="activity">Queued · {request.message}</p>
      {:else if request.error && request.id !== record.active_request}<p class="notice" role="alert">{request.error}</p>{/if}
    {/each}
    {#if !record.live}<p class="notice">Saved history. Start a new conversation to continue.</p>{/if}
    {#if inventory.error}<p class="notice" role="status">Server disconnected. The request may still be running. <button onclick={() => inventory.refresh()}>Reconnect</button></p>{/if}
  </div>

  <form class="composer" {...form.enhance(async (submitted) => {
    actionError = '';
    submitted.fields.created.set(Date.now());
    try {
      if (await submitted.submit().updates(projectConversations(project))) { submitted.element.reset(); requestID = crypto.randomUUID(); created = Date.now(); onchanged?.(); }
    } catch (error) { if (!navigateOnUnauthorized(error)) actionError = errorMessage(error); }
  })}>
    <input {...form.fields.conversation.as('hidden', record.id)} />
    <input {...form.fields.id.as('hidden', requestID)} />
    <input {...form.fields.created.as('hidden', created)} />
    <textarea {...form.fields.message.as('text')} rows={2} onkeydown={keydown} placeholder={`Message ${provider}…`} aria-label={`Message ${record.title}`} disabled={!record.live || !!form.pending || stopping || uncertain}></textarea>
    <div class="composer-meta"><span title={record.model}>{provider} · {modelName(record.model)}{record.effort ? ` · ${record.effort}` : ''}</span><code title={record.worktree}>{record.branch || record.worktree} · {machine}</code>{#if working && hasMessage}<span>{canSteer ? '↩ sends now · esc stops' : '↩ queues until this turn ends · esc stops'}</span>{/if}</div>
    {#if working && (!hasMessage || uncertain)}
      <button type="button" class="send" aria-label="Stop response" onclick={stop} disabled={!canStop || stopPending}>{#if stopping || stopPending}<LoaderCircle size={16} class="spin" />{:else}<Square size={12} fill="currentColor" />{/if}</button>
    {:else}
      <button type="submit" class="send" aria-label="Send message" disabled={!record.live || !hasMessage || !!form.pending || stopping || uncertain}><ArrowUp size={18} /></button>
    {/if}
    {#each form.fields.allIssues() ?? [] as issue}<p class="notice" role="alert">{issue.message}</p>{/each}
    {#if actionError}<p class="notice" role="alert">{actionError}</p>{/if}
  </form>
</section>

<style>
  .chat { height: 100%; display: flex; flex-direction: column; min-width: 0; min-height: 0; }
  .placement { display: flex; align-items: center; justify-content: space-between; gap: 12px; font-size: 11px; color: var(--muted-foreground); }
  .placement span { overflow-wrap: anywhere; }
  .placement button { flex: none; padding: 0; border: 0; background: none; color: inherit; font: inherit; text-decoration: underline; cursor: pointer; }
  .transcript { flex: 1; min-height: 0; overflow: auto; overflow-wrap: anywhere; padding: 28px max(24px, calc((100% - 680px) / 2)); display: flex; flex-direction: column; gap: 22px; }
  p, h2 { margin: 0; }
  article p { white-space: pre-wrap; line-height: 1.65; }
  article.user { align-self: flex-end; max-width: 86%; border-radius: 18px; padding: 12px 16px; background: var(--muted); }
  .empty { margin: auto; text-align: center; color: var(--muted-foreground); }
  .empty h2 { margin-bottom: 8px; font-size: 16px; font-weight: 500; color: var(--foreground); }
  .empty p, .activity, .notice, .muted { color: var(--muted-foreground); font-size: 12px; }
  .notice { line-height: 1.5; overflow-wrap: anywhere; }
  .runs { display: grid; gap: 6px; font-size: 12px; }
  .runs a { color: inherit; text-decoration: none; }
  .composer { position: relative; flex: none; width: min(680px, calc(100% - 48px)); margin: 0 auto 20px; }
  textarea { box-sizing: border-box; width: 100%; min-height: 96px; max-height: 35vh; resize: vertical; padding: 14px 48px 38px 16px; border: 1px solid var(--border); border-radius: 18px; outline: none; color: var(--foreground); background: var(--background); font: inherit; font-size: 14px; }
  textarea:focus { border-color: var(--ring); }
  .composer-meta { position: absolute; left: 16px; right: 52px; bottom: 16px; min-width: 0; overflow: hidden; display: flex; align-items: center; justify-content: space-between; gap: 12px; color: var(--muted-foreground); font-size: 11px; pointer-events: none; }
  code { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-family: var(--font-mono); font-size: 11px; }
  .composer-meta code { flex: 1; min-width: 0; text-align: right; }
  .composer-meta span { flex: none; white-space: nowrap; }
  .send { position: absolute; right: 12px; bottom: 14px; display: grid; place-items: center; width: 30px; height: 30px; border: 0; border-radius: 50%; color: var(--primary-foreground); background: var(--primary); cursor: pointer; }
  .send:disabled { opacity: .35; cursor: default; }
  .send:focus-visible { outline: 2px solid var(--ring); outline-offset: 3px; }
  .notice button { color: inherit; background: transparent; border: 0; text-decoration: underline; cursor: pointer; }
  :global(.spin) { animation: spin 1s linear infinite; }
  @keyframes spin { to { transform: rotate(360deg); } }
  @media (prefers-reduced-motion: reduce) { :global(.spin) { animation: none; } }
  @media (max-width: 600px) { .composer-meta span { display: none; } }
</style>
