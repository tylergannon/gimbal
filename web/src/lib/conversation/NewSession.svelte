<script lang="ts">
  import { navigateOnUnauthorized } from '#lib/auth/unauthorized';
  import { errorMessage } from '#lib/errors';
  import ArrowUpIcon from '@lucide/svelte/icons/arrow-up';
  import CheckIcon from '@lucide/svelte/icons/check';
  import ChevronDownIcon from '@lucide/svelte/icons/chevron-down';
  import FolderIcon from '@lucide/svelte/icons/folder';
  import GitBranchIcon from '@lucide/svelte/icons/git-branch';
  import MonitorIcon from '@lucide/svelte/icons/monitor';
  import * as Popover from '#lib/components/ui/popover';
  import { Switch } from '#lib/components/ui/switch';
  import type { Conversation } from '#lib/skgo/conversation/types';
  import { goto } from '$app/navigation';
  import { conversationSetup, createConversation, projectConversations } from '../../routes/conversation.remote';
  import HarnessPicker from './HarnessPicker.svelte';
  import { harnessName } from './names';

  let { project, name, oncreated }: { project: string; name: string; oncreated: (conversation: Conversation) => void } = $props();
  const form = $derived(createConversation.for(project));
  const setupQuery = $derived(conversationSetup(project));
  const conversationsQuery = $derived(projectConversations(project));
  const setup = $derived(setupQuery.current);
  const recent = $derived(conversationsQuery.current ?? []);
  let actionError = $state('');
  let picked = $state<{ provider: string; model: string }>();
  // Defaults follow this project's last session, then the first harness's cheap model.
  const choice = $derived(picked
    ?? (recent[0] ? { provider: recent[0].provider, model: recent[0].model }
    : setup?.harnesses[0] && { provider: setup.harnesses[0].id, model: setup.harnesses[0].default }));
  const starts = $derived(recent.filter((item, index) =>
    recent.findIndex(other => other.provider === item.provider && other.model === item.model) === index).slice(0, 2));
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
    {/if}
    <textarea {...form.fields.message.as('text')} rows={3} aria-label="First message" disabled={!!form.pending}
      placeholder="Describe the work. Your first message starts the session."
      onkeydown={(event) => { if (event.key === 'Enter' && !event.shiftKey && !event.isComposing) { event.preventDefault(); event.currentTarget.form?.requestSubmit(); } }}></textarea>
    <div class="choices">
      {#if setup && choice}
        <HarnessPicker harnesses={setup.harnesses} {choice} onpick={(next) => (picked = next)} />
        <Popover.Root>
          <Popover.Trigger class="chip environment" aria-label="Environment" title={setup.directory}>
            <MonitorIcon size={14} aria-hidden="true" /><span class="chip-label">This Mac · {setup.directory}</span><ChevronDownIcon size={14} aria-hidden="true" />
          </Popover.Trigger>
          <Popover.Content align="start" collisionPadding={8} class="w-72 gap-1 p-1.5">
            <h3>This Mac</h3>
            <div class="option selected">
              <span class="label">{setup.directory}<small>{setup.git ? `Git · ${setup.branch}` : 'Not a Git repository'}</small></span>
              <CheckIcon size={14} aria-hidden="true" />
            </div>
            <p class="note">Sessions run on this Mac, in this project's directory.</p>
          </Popover.Content>
        </Popover.Root>
        <Popover.Root>
          <Popover.Trigger class="chip" aria-label="Worktree">
            {#if setup.git}<GitBranchIcon size={14} aria-hidden="true" />New worktree{:else}<FolderIcon size={14} aria-hidden="true" />No worktree{/if}
            <ChevronDownIcon size={14} aria-hidden="true" />
          </Popover.Trigger>
          <Popover.Content align="start" collisionPadding={8} class="w-80 gap-3">
            <label class="toggle">Create a worktree <Switch checked={setup.git} disabled /></label>
            {#if setup.git}
              <div class="field"><span>Branch</span><code>gimbal/conversation-…</code></div>
              <div class="field"><span>From</span><code><GitBranchIcon size={13} aria-hidden="true" />{setup.branch}</code></div>
              <p class="note">A separate checkout on This Mac. Other sessions can't change its files. You merge the branch when you're done.</p>
            {:else}
              <p class="note">{setup.directory} is not a Git repository, so this session works in the directory itself. Other sessions there can change the same files.</p>
            {/if}
          </Popover.Content>
        </Popover.Root>
      {/if}
      <button class="send" aria-label="Start session" disabled={!!form.pending || !choice}><ArrowUpIcon size={16} /></button>
    </div>
  </form>
  {#if actionError}<p class="issue" role="alert">{actionError}</p>{/if}
  {#each form.fields.allIssues() ?? [] as issue (issue.message)}<p class="issue" role="alert">{issue.message}</p>{/each}
  {#if starts.length > 0}
    <div class="start-like">
      <span>Start like</span>
      {#each starts as item (item.id)}
        <button type="button" onclick={() => (picked = { provider: item.provider, model: item.model })}>{item.title} · {harnessName(item.provider)} on This Mac</button>
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
  .choices { display: flex; align-items: center; gap: 4px; min-width: 0; padding: 6px 8px 8px; }
  .choices :global(.chip) { display: inline-flex; flex: none; align-items: center; gap: 6px; min-width: 0; max-width: 300px; height: 28px; padding: 0 8px; border: 0; border-radius: 6px; color: var(--foreground); background: none; font: inherit; font-size: 13px; white-space: nowrap; cursor: pointer; }
  .choices :global(.chip:hover), .choices :global(.chip[aria-expanded='true']) { background: var(--muted); }
  .choices :global(.chip svg) { flex: none; color: var(--muted-foreground); }
  .choices :global(.chip.environment) { flex: 0 1 auto; }
  .choices :global(.chip-label) { overflow: hidden; text-overflow: ellipsis; }
  .send { display: grid; flex: none; place-items: center; width: 28px; height: 28px; margin-left: auto; border: 0; border-radius: 999px; color: var(--primary-foreground); background: var(--primary); cursor: pointer; }
  .send:disabled { color: var(--muted-foreground); background: var(--muted); cursor: default; }
  h3 { margin: 4px 6px 2px; color: var(--muted-foreground); font-size: 12px; font-weight: 400; }
  .option { display: flex; align-items: center; gap: 10px; padding: 6px 8px; border-radius: 6px; font-size: 13px; }
  .option.selected { background: var(--muted); }
  .label { display: grid; flex: 1; min-width: 0; overflow-wrap: anywhere; }
  small { color: var(--muted-foreground); font-size: 11px; }
  .note { margin: 0; padding: 0 2px; color: var(--muted-foreground); font-size: 12px; line-height: 1.5; }
  .toggle { display: flex; align-items: center; justify-content: space-between; font-size: 13px; font-weight: 500; }
  .field { display: grid; gap: 6px; font-size: 13px; }
  .field code { display: flex; align-items: center; gap: 6px; height: 34px; padding: 0 10px; border: 1px solid var(--border); border-radius: 8px; color: var(--muted-foreground); font-family: var(--font-sans); font-size: 13px; }
  .issue { margin: 8px 2px 0; color: var(--destructive); font-size: 12px; }
  .start-like { display: flex; flex-wrap: wrap; align-items: baseline; gap: 4px 18px; margin-top: 12px; padding: 0 2px; color: var(--muted-foreground); font-size: 12px; }
  .start-like button { padding: 0; border: 0; color: var(--foreground); opacity: .75; background: none; font: inherit; cursor: pointer; }
  .start-like button:hover { opacity: 1; text-decoration: underline; }
  @media (max-height: 640px) { .new-session { padding-top: 40px; } }
</style>
