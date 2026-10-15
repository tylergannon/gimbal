<script lang="ts">
  import { onMount } from 'svelte';
  import { SvelteMap } from 'svelte/reactivity';
  import FileTextIcon from '@lucide/svelte/icons/file-text';
  import XIcon from '@lucide/svelte/icons/x';
  import { navigateOnUnauthorized } from '#lib/auth/unauthorized';
  import { errorMessage } from '#lib/errors';
  import type { Conversation } from '#lib/skgo/conversation/types';
  import { goto } from '$app/navigation';
  import { conversationSetup, createConversation, projectConversations } from '../../routes/conversation.remote';
  import { harnessName } from './names';
  import SessionSetup, { type Choices } from './SessionSetup.svelte';
  import { environmentSnapshot } from '#lib/environments.remote';

  let { project, name, active = true, oncreated, fileContext, oncontextremoved }: {
    project: string; name: string; active?: boolean; oncreated: (conversation: Conversation) => void;
    fileContext?: { id: string; name: string }; oncontextremoved?: () => void;
  } = $props();
  const form = $derived(createConversation.for(project));
  const conversationsQuery = $derived(projectConversations(project));
  const recent = $derived(conversationsQuery.current ?? []);
  let environment = $state('local');
  let touched = false;
  let defaults: Conversation[] = [];
  function pickEnvironment(id: string) {
    choose({});
    environment = id;
    const previous = defaults.find(item => (item.environment || 'local') === id);
    if (!choices.has(id) && previous) choices.set(id, setupOf(previous));
  }
  const setupQuery = $derived(conversationSetup({ project, environment }));
  const inventory = $derived(environmentSnapshot(project));
  onMount(() => {
    let mounted = true;
    void (async () => {
      try {
        await conversationsQuery;
        if (!mounted) return;
        defaults = conversationsQuery.current ?? [];
        if (!touched && defaults[0]) applySetup(defaults[0]);
      } catch (error) { if (!navigateOnUnauthorized(error)) actionError = errorMessage(error); }
    })();
    const timer = setInterval(() => {
      if (active) void inventory.refresh().catch(navigateOnUnauthorized);
    }, 2000);
    return () => { mounted = false; clearInterval(timer); };
  });
  const setup = $derived(setupQuery.current);
  let actionError = $state('');
  // Draft choices outlive the active-only controls.
  const choices = new SvelteMap<string, Choices>();
  const selected = $derived(choices.get(environment) ?? {});
  function choose(patch: Partial<Choices>) {
    touched = true;
    choices.set(environment, { shared: sharedCheckout, base, harness: choice, ...selected, ...patch });
  }
  const sharedCheckout = $derived(selected.shared ?? false);
  const isolated = $derived(!!setup?.git && setup.capabilities.worktree && !sharedCheckout);
  const suggestedBranch = $derived((form.fields.message.value() ?? '').split('\n')[0]
    .toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '').slice(0, 48).replace(/-$/, ''));
  const branch = $derived(selected.branch ?? (suggestedBranch ? `gimbal/${suggestedBranch}` : ''));
  const base = $derived(selected.base ?? setup?.base ?? '');
  const harnesses = $derived(setup?.harnesses ?? []);
  const usable = $derived(harnesses.filter(item => !setup?.harness_states.some(state => state.id === item.id && (state.state === 'missing' || state.state === 'sign-in-required'))));
  const choice = $derived(selected.harness ?? ((usable[0] ?? harnesses[0]) && { provider: (usable[0] ?? harnesses[0]).id, model: (usable[0] ?? harnesses[0]).default, effort: '' }));
  const canStart = $derived(!!choice && usable.some(item => item.id === choice.provider) && (!choice.effort || !!harnesses.find(item => item.id === choice.provider)?.efforts.find(item => item.model === choice.model)?.levels.includes(choice.effort)));
  const setupKey = (item: Conversation) => JSON.stringify([item.provider, item.model, item.effort || '', item.environment || 'local', !!item.shared_checkout, item.base || '']);
  const starts = $derived(recent.filter((item, index) => recent.findIndex(other => setupKey(other) === setupKey(item)) === index).slice(0, 3));
  function setupOf(item: Conversation): Choices {
    return { shared: !!item.shared_checkout, base: item.base || undefined,
      harness: { provider: item.provider, model: item.model, effort: item.effort || '' } };
  }
  export function applySetup(item: Conversation) {
    touched = true;
    environment = item.environment || 'local';
    choices.set(item.environment || 'local', setupOf(item));
  }

</script>

<section class="new-session">
  <h1>New session in {name}</h1>
  <form class="composer" {...form.enhance(async (submitted) => {
    actionError = '';
    try {
      if (await submitted.submit().updates(projectConversations(project)) && submitted.result) {
        oncreated(submitted.result);
        await goto(`/projects/${project}/conversations/${submitted.result.id}`);
      }
    } catch (error) {
      if (!navigateOnUnauthorized(error)) actionError = errorMessage(error);
    }
  })}>
    {#if choice}
      <input {...form.fields.provider.as('hidden', choice.provider)} />
      <input {...form.fields.model.as('hidden', choice.model)} />
      <input {...form.fields.effort.as('hidden', choice.effort || '')} />
    {/if}
    <input {...form.fields.environment.as('hidden', environment)} />
    <input {...form.fields.file_id.as('hidden', fileContext?.id ?? '')} />
    <input {...form.fields.shared_checkout.as('hidden', !isolated)} />
    <input {...form.fields.branch.as('hidden', isolated ? branch : '')} />
    <input {...form.fields.base.as('hidden', isolated ? base : '')} />
    {#if fileContext}<div class="file-context"><FileTextIcon size={14} aria-hidden="true" /><span>{fileContext.name}</span><button type="button" aria-label="Remove file context" onclick={oncontextremoved} disabled={!!form.pending}><XIcon size={12} /></button></div>{/if}
    <textarea {...form.fields.message.as('text')} rows={3} aria-label="First message" disabled={!!form.pending}
      placeholder="Describe the work. Your first message starts the session."
      onkeydown={(event) => { if (event.key === 'Enter' && !event.shiftKey && !event.isComposing) { event.preventDefault(); if (canStart && !form.pending) event.currentTarget.form?.requestSubmit(); } }}></textarea>
    {#if active}
    <SessionSetup {setup} inventory={inventory.current} {environment}
      choices={{shared: sharedCheckout, branch, base, harness: choice}} pending={!!form.pending}
      canStart={canStart && !!setup && !setupQuery.error} onchoose={choose} onenvironment={pickEnvironment} />
    {/if}
  </form>
  {#if setupQuery.error}<p class="issue" role="alert">{errorMessage(setupQuery.error)} <button type="button" onclick={() => setupQuery.refresh()}>Retry setup</button></p>{/if}
  {#if setup && !canStart}<p class="issue" role="status">No usable harness is available on {setup.machine}. Open Machines to inspect sign-in and installation details.</p>{/if}
  {#if setup?.harness_states.some(item => item.id === choice?.provider && item.state === 'unknown')}<p class="note">Harness sign-in on {setup.machine} is unverified. The first request will report any sign-in needed there.</p>{/if}
  {#if actionError}<p class="issue" role="alert">{actionError}</p>{/if}
  {#each form.fields.allIssues() ?? [] as issue (issue.message)}<p class="issue" role="alert">{issue.message}</p>{/each}
  {#if starts.length > 0}
    <div class="start-like">
      <span>Start like</span>
      {#each starts as item (item.id)}
        <button type="button" disabled={!!form.pending} onclick={() => applySetup(item)}>{item.title} · {harnessName(item.provider)} on {item.placement?.machine.name ?? 'This Mac'}{item.effort ? ` · ${item.effort}` : ''}</button>
      {/each}
    </div>
  {/if}
</section>

<style>
  .new-session { width: min(720px, calc(100% - 48px)); margin: 0 auto; padding: 72px 0 24px; }
  h1 { margin: 0 0 14px; font-size: 15px; font-weight: 600; }
  .composer { border: 1px solid var(--border); border-radius: 14px; background: var(--background); box-shadow: var(--shadow-xs, 0 1px 2px rgb(0 0 0 / .05)); }
  .composer:focus-within { border-color: var(--ring); }
  textarea { box-sizing: border-box; display: block; width: 100%; min-height: 76px; max-height: 40vh; resize: none; padding: 14px 16px 4px; border: 0; outline: none; color: var(--foreground); background: transparent; font: inherit; }
  .file-context { display: flex; align-items: center; gap: 7px; width: fit-content; max-width: calc(100% - 28px); margin: 12px 14px 0; padding: 5px 7px; background: var(--muted); border-radius: 6px; font-size: 12px; }
  .file-context span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .file-context button { display: flex; padding: 2px; border: 0; background: none; color: var(--muted-foreground); cursor: pointer; }
  .issue { margin: 8px 2px 0; color: var(--destructive); font-size: 12px; }
  .start-like { display: flex; flex-wrap: wrap; align-items: baseline; gap: 4px 18px; margin-top: 12px; padding: 0 2px; color: var(--muted-foreground); font-size: 12px; }
  .start-like button { padding: 0; border: 0; color: var(--foreground); opacity: .75; background: none; font: inherit; cursor: pointer; }
  .start-like button:hover { opacity: 1; text-decoration: underline; }
  @media (max-height: 640px) { .new-session { padding-top: 40px; } }
</style>
