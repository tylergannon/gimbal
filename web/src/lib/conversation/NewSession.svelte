<script lang="ts">
  import { untrack } from 'svelte';
  import { SvelteMap } from 'svelte/reactivity';
  import FileTextIcon from '@lucide/svelte/icons/file-text';
  import XIcon from '@lucide/svelte/icons/x';
  import { navigateOnUnauthorized } from '#lib/auth/unauthorized';
  import { errorMessage } from '#lib/errors';
  import ArrowUpIcon from '@lucide/svelte/icons/arrow-up';
  import ChevronDownIcon from '@lucide/svelte/icons/chevron-down';
  import FolderIcon from '@lucide/svelte/icons/folder';
  import GitBranchIcon from '@lucide/svelte/icons/git-branch';
  import * as Popover from '#lib/components/ui/popover';
  import { Switch } from '#lib/components/ui/switch';
  import type { Conversation } from '#lib/skgo/conversation/types';
  import { goto } from '$app/navigation';
  import { conversationSetup, createConversation, projectConversations } from '../../routes/conversation.remote';
  import HarnessPicker from './HarnessPicker.svelte';
  import { harnessName } from './names';
  import { environmentSnapshot } from '#lib/environments.remote';
  import EnvironmentPicker from '#lib/environments/EnvironmentPicker.svelte';

  let { project, name, active = true, oncreated, fileContext, oncontextremoved, startLike }: {
    project: string; name: string; active?: boolean; oncreated: (conversation: Conversation) => void;
    fileContext?: { id: string; name: string }; oncontextremoved?: () => void; startLike?: { conversation: string; nonce: number };
  } = $props();
  const form = $derived(createConversation.for(project));
  const conversationsQuery = $derived(projectConversations(project));
  const recent = $derived(conversationsQuery.current ?? []);
  let environment = $state('local');
  let initialized = $state(false);
  let appliedSetup = $state(0);
  function pickEnvironment(id: string) { environment = id; initialized = true; }
  const setupQuery = $derived(conversationSetup({ project, environment }));
  const inventory = $derived(environmentSnapshot(project));
  $effect(() => {
    if (!active) return;
    const query = inventory;
    if (untrack(() => query.ready)) void query.refresh().catch(navigateOnUnauthorized);
    const timer = setInterval(() => { void query.refresh().catch(navigateOnUnauthorized); }, 2000);
    return () => clearInterval(timer);
  });
  const setup = $derived(setupQuery.current);
  let actionError = $state('');
  let picker = $state('');
  $effect(() => { if (!active) picker = ''; });
  type Choices = { shared?: boolean; branch?: string; base?: string; harness?: { provider: string; model: string; effort?: string } };
  const choices = new SvelteMap<string, Choices>();
  const selected = $derived(choices.get(environment) ?? {});
  function choose(patch: Partial<Choices>) { choices.set(environment, { ...selected, ...patch }); }
  const sharedCheckout = $derived(selected.shared ?? false);
  const isolated = $derived(!!setup?.git && setup.capabilities.worktree && !sharedCheckout);
  const suggestedBranch = $derived((form.fields.message.value() ?? '').split('\n')[0]
    .toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '').slice(0, 48).replace(/-$/, ''));
  const branch = $derived(selected.branch ?? (suggestedBranch ? `gimbal/${suggestedBranch}` : ''));
  const base = $derived(selected.base ?? setup?.base ?? '');
  const harnesses = $derived(setup?.harnesses ?? []);
  const usable = $derived(harnesses.filter(item => !setup?.harness_states.some(state => state.id === item.id && (state.state === 'missing' || state.state === 'sign-in-required'))));
  const choice = $derived(selected.harness ?? ((usable[0] ?? harnesses[0]) && { provider: (usable[0] ?? harnesses[0]).id, model: (usable[0] ?? harnesses[0]).default, effort: '' }));
  // Capture defaults once into the draft. Later background activity may reorder
  // recent sessions, but cannot change a draft's setup.
  $effect(() => {
    if (!conversationsQuery.ready) return;
    if (startLike && appliedSetup !== startLike.nonce) {
      const item = recent.find(item => item.id === startLike.conversation);
      if (item) untrack(() => like(item));
      appliedSetup = startLike.nonce;
    } else if (!initialized) {
      const item = recent[0];
      if (item) untrack(() => like(item));
      initialized = true;
    }
    if (setup && !choices.has(environment)) {
      const item = untrack(() => recent.find(item => (item.environment || 'local') === environment));
      const first = usable[0] ?? harnesses[0];
      choices.set(environment, item ? setupOf(item) : { shared: false, base: setup.base,
        harness: first && { provider: first.id, model: first.default, effort: '' } });
    }
  });
  const canStart = $derived(!!choice && usable.some(item => item.id === choice.provider) && (!choice.effort || !!harnesses.find(item => item.id === choice.provider)?.efforts.find(item => item.model === choice.model)?.levels.includes(choice.effort)));
  const setupKey = (item: Conversation) => JSON.stringify([item.provider, item.model, item.effort || '', item.environment || 'local', !!item.shared_checkout, item.base || '']);
  const starts = $derived(recent.filter((item, index) => recent.findIndex(other => setupKey(other) === setupKey(item)) === index).slice(0, 3));
  function setupOf(item: Conversation): Choices {
    return { shared: !!item.shared_checkout, base: item.base || undefined,
      harness: { provider: item.provider, model: item.model, effort: item.effort || '' } };
  }
  function like(item: Conversation) {
    pickEnvironment(item.environment || 'local');
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
    <fieldset class="choices" aria-label="Session setup" disabled={!!form.pending}>
      {#if setup && choice}<HarnessPicker bind:open={() => picker === 'harness', (open) => picker = open ? 'harness' : ''} {harnesses} {choice} states={setup.harness_states} machine={setup.machine} onpick={(next) => choose({ harness: next })} />{/if}
      {#if inventory.current || setup}
        <EnvironmentPicker bind:open={() => picker === 'environment', (open) => picker = open ? 'environment' : ''} snapshot={(inventory.current ?? setup!.environments)} selected={environment} directory={setup?.directory ?? inventory.current?.environments.find(row => row.environment.id === environment)?.environment.directory ?? ''} machine={setup?.machine ?? inventory.current?.machines.find(item => item.id === inventory.current?.environments.find(row => row.environment.id === environment)?.environment.machine)?.name ?? 'Environment'} onpick={pickEnvironment} />
      {/if}
      {#if setup}
        <Popover.Root bind:open={() => picker === 'checkout', (open) => picker = open ? 'checkout' : ''}>
          <Popover.Trigger class="chip" aria-label="Worktree">
            {#if isolated}<GitBranchIcon size={14} aria-hidden="true" />New worktree{:else}<FolderIcon size={14} aria-hidden="true" />{setup.git ? 'Shared checkout' : 'No worktree'}{/if}
            <ChevronDownIcon size={14} aria-hidden="true" />
          </Popover.Trigger>
          <Popover.Content align="start" collisionPadding={8} class="w-80 gap-3">
            <label class="toggle">Create a worktree <Switch aria-label="Create a worktree" bind:checked={() => isolated, (checked) => choose({ shared: !checked })} disabled={!setup.git || !setup.capabilities.worktree || !!form.pending} /></label>
            {#if isolated}
              <label class="field"><span>Branch</span><input aria-label="New branch" bind:value={() => branch, (value) => choose({ branch: value })} placeholder="Generated when the session starts" disabled={!!form.pending} /></label>
              <label class="field"><span>From</span><select aria-label="Base branch" bind:value={() => base, (value) => choose({ base: value })} disabled={!!form.pending}>
                {#each setup.bases as ref (ref)}<option value={ref}>{ref}</option>{/each}
              </select></label>
              <p class="note">A separate checkout on {setup.machine}. Other sessions can't change its files. You merge the branch when you're done.</p>
            {:else if setup.git && !setup.capabilities.worktree}
              <p class="note">{setup.machine} requires its configured working directory{setup.directory ? ` (${setup.directory})` : ''}. A separate worktree is unavailable; sessions share those files.</p>
            {:else if setup.git}
              <p class="note">This session works in {setup.directory}, in the existing checkout. You and other sessions there can change the same files.</p>
            {:else if setup.unborn}
              <p class="note">{setup.directory} has no Git commit yet, so this session works in the directory itself. Other sessions there can change the same files.</p>
            {:else}
              <p class="note">{setup.directory} is not a Git repository, so this session works in the directory itself. Other sessions there can change the same files.</p>
            {/if}
          </Popover.Content>
        </Popover.Root>
      {/if}
      <button class="send" aria-label="Start session" disabled={!!form.pending || !canStart || !setup || !!setupQuery.error}><ArrowUpIcon size={16} /></button>
    </fieldset>
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
        <button type="button" disabled={!!form.pending} onclick={() => like(item)}>{item.title} · {harnessName(item.provider)} on {item.placement?.machine.name ?? 'This Mac'}{item.effort ? ` · ${item.effort}` : ''}</button>
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
  .choices { border: 0; margin: 0; display: flex; align-items: center; gap: 4px; min-width: 0; padding: 6px 8px 8px; }
  .choices :global(.chip) { display: inline-flex; flex: none; align-items: center; gap: 6px; min-width: 0; max-width: 300px; height: 28px; padding: 0 8px; border: 0; border-radius: 6px; color: var(--foreground); background: none; font: inherit; font-size: 13px; white-space: nowrap; cursor: pointer; }
  .choices :global(.chip:hover), .choices :global(.chip[aria-expanded='true']) { background: var(--muted); }
  .choices :global(.chip svg) { flex: none; color: var(--muted-foreground); }
  .choices :global(.chip.environment) { flex: 0 1 auto; }
  .choices :global(.chip-label) { overflow: hidden; text-overflow: ellipsis; }
  .file-context { display: flex; align-items: center; gap: 7px; width: fit-content; max-width: calc(100% - 28px); margin: 12px 14px 0; padding: 5px 7px; background: var(--muted); border-radius: 6px; font-size: 12px; }
  .file-context span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .file-context button { display: flex; padding: 2px; border: 0; background: none; color: var(--muted-foreground); cursor: pointer; }
  .send { display: grid; flex: none; place-items: center; width: 28px; height: 28px; margin-left: auto; border: 0; border-radius: 999px; color: var(--primary-foreground); background: var(--primary); cursor: pointer; }
  .send:disabled { color: var(--muted-foreground); background: var(--muted); cursor: default; }
  .note { margin: 0; padding: 0 2px; color: var(--muted-foreground); font-size: 12px; line-height: 1.5; }
  .toggle { display: flex; align-items: center; justify-content: space-between; font-size: 13px; font-weight: 500; }
  .field { display: grid; gap: 6px; font-size: 13px; }
  .field input, .field select { box-sizing: border-box; width: 100%; min-width: 0; height: 34px; padding: 0 10px; border: 1px solid var(--border); border-radius: 8px; color: var(--foreground); background: var(--background); font: inherit; }
  .issue { margin: 8px 2px 0; color: var(--destructive); font-size: 12px; }
  .start-like { display: flex; flex-wrap: wrap; align-items: baseline; gap: 4px 18px; margin-top: 12px; padding: 0 2px; color: var(--muted-foreground); font-size: 12px; }
  .start-like button { padding: 0; border: 0; color: var(--foreground); opacity: .75; background: none; font: inherit; cursor: pointer; }
  .start-like button:hover { opacity: 1; text-decoration: underline; }
  @media (max-height: 640px) { .new-session { padding-top: 40px; } }
</style>
