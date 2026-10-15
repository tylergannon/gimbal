<script module lang="ts">
  export type Choices = { shared?: boolean; branch?: string; base?: string; harness?: { provider: string; model: string; effort?: string } };
</script>
<script lang="ts">
  import ArrowUpIcon from '@lucide/svelte/icons/arrow-up';
  import ChevronDownIcon from '@lucide/svelte/icons/chevron-down';
  import FolderIcon from '@lucide/svelte/icons/folder';
  import GitBranchIcon from '@lucide/svelte/icons/git-branch';
  import * as Popover from '#lib/components/ui/popover';
  import { Switch } from '#lib/components/ui/switch';
  import type { Setup } from '#lib/skgo/conversation/types';
  import HarnessPicker from './HarnessPicker.svelte';
  import EnvironmentPicker from '#lib/environments/EnvironmentPicker.svelte';
  let { setup, inventory, environment, choices, pending, canStart, onchoose, onenvironment }: {
    setup?: Setup; inventory?: Setup['environments']; environment: string; choices: Choices;
    pending: boolean; canStart: boolean; onchoose: (patch: Partial<Choices>) => void; onenvironment: (id: string) => void;
  } = $props();
  // One active-only owner gives the three pickers mutual exclusion and a short lifetime.
  let picker = $state('');
  const harnesses = $derived(setup?.harnesses ?? []);
  const choice = $derived(choices.harness);
  const isolated = $derived(!!setup?.git && setup.capabilities.worktree && !choices.shared);
  const branch = $derived(choices.branch ?? '');
  const base = $derived(choices.base ?? '');
</script>
    <fieldset class="choices" aria-label="Session setup" disabled={pending}>
      {#if setup && choice}<HarnessPicker bind:open={() => picker === 'harness', open => picker = open ? 'harness' : ''} {harnesses} {choice} states={setup.harness_states} machine={setup.machine} onpick={(next) => onchoose({ harness: next })} />{/if}
      {#if inventory || setup}
        <EnvironmentPicker bind:open={() => picker === 'environment', open => picker = open ? 'environment' : ''} snapshot={(inventory ?? setup!.environments)} selected={environment} directory={setup?.directory ?? inventory?.environments.find(row => row.environment.id === environment)?.environment.directory ?? ''} machine={setup?.machine ?? inventory?.machines.find(item => item.id === inventory?.environments.find(row => row.environment.id === environment)?.environment.machine)?.name ?? 'Environment'} onpick={onenvironment} />
      {/if}
      {#if setup}
        <Popover.Root bind:open={() => picker === 'checkout', open => picker = open ? 'checkout' : ''}>
          <Popover.Trigger class="chip" aria-label="Worktree">
            {#if isolated}<GitBranchIcon size={14} aria-hidden="true" />New worktree{:else}<FolderIcon size={14} aria-hidden="true" />{setup.git ? 'Shared checkout' : 'No worktree'}{/if}
            <ChevronDownIcon size={14} aria-hidden="true" />
          </Popover.Trigger>
          <Popover.Content align="start" collisionPadding={8} class="w-80 gap-3">
            <label class="toggle">Create a worktree <Switch aria-label="Create a worktree" bind:checked={() => isolated, (checked) => onchoose({ shared: !checked })} disabled={!setup.git || !setup.capabilities.worktree || pending} /></label>
            {#if isolated}
              <label class="field"><span>Branch</span><input aria-label="New branch" bind:value={() => branch, (value) => onchoose({ branch: value })} placeholder="Generated when the session starts" disabled={pending} /></label>
              <label class="field"><span>From</span><select aria-label="Base branch" bind:value={() => base, (value) => onchoose({ base: value })} disabled={pending}>
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
      <button class="send" aria-label="Start session" disabled={pending || !canStart}><ArrowUpIcon size={16} /></button>
    </fieldset>
<style>
  .choices { border: 0; margin: 0; display: flex; align-items: center; gap: 4px; min-width: 0; padding: 6px 8px 8px; }
  .choices :global(.chip) { display: inline-flex; flex: none; align-items: center; gap: 6px; min-width: 0; max-width: 300px; height: 28px; padding: 0 8px; border: 0; border-radius: 6px; color: var(--foreground); background: none; font: inherit; font-size: 13px; white-space: nowrap; cursor: pointer; }
  .choices :global(.chip:hover), .choices :global(.chip[aria-expanded='true']) { background: var(--muted); }
  .choices :global(.chip svg) { flex: none; color: var(--muted-foreground); }
  .choices :global(.chip.environment) { flex: 0 1 auto; }
  .choices :global(.chip-label) { overflow: hidden; text-overflow: ellipsis; }
  .send { display: grid; flex: none; place-items: center; width: 28px; height: 28px; margin-left: auto; border: 0; border-radius: 999px; color: var(--primary-foreground); background: var(--primary); cursor: pointer; }
  .send:disabled { color: var(--muted-foreground); background: var(--muted); cursor: default; }
  .note { margin: 0; padding: 0 2px; color: var(--muted-foreground); font-size: 12px; line-height: 1.5; }
  .toggle { display: flex; align-items: center; justify-content: space-between; font-size: 13px; font-weight: 500; }
  .field { display: grid; gap: 6px; font-size: 13px; }
  .field input, .field select { box-sizing: border-box; width: 100%; min-width: 0; height: 34px; padding: 0 10px; border: 1px solid var(--border); border-radius: 8px; color: var(--foreground); background: var(--background); font: inherit; }

</style>
