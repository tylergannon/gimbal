<script lang="ts">
  import { onMount } from 'svelte';
  import * as Dialog from '#lib/components/ui/dialog';
  import type { Conversation } from '#lib/skgo/conversation/types';
  import { cleanupConversation, projectConversations } from '../../routes/conversation.remote';
  import { navigateOnUnauthorized } from '#lib/auth/unauthorized';
  import { errorMessage } from '#lib/errors';
  import RenameChat from './RenameChat.svelte';

  // Mounted only for the selected chat: dialogs end with this view's activation.
  let { project, current, onchanged }: { project: string; current: Conversation; onchanged?: () => void } = $props();
  let renaming = $state(false);
  let confirmEnd = $state(false);
  let ending = $state(false);
  let failure = $state('');
  onMount(() => {
    const command = (event: Event) => { if ((event as CustomEvent).detail.command === 'renameTab') renaming = true; };
    window.addEventListener('gimbal-workspace-command', command);
    return () => window.removeEventListener('gimbal-workspace-command', command);
  });
  async function endSession() {
    ending = true; failure = '';
    try {
      await cleanupConversation({ project, conversation: current.id });
      await projectConversations(project).refresh();
      confirmEnd = false;
      onchanged?.();
    } catch (error) { if (!navigateOnUnauthorized(error)) failure = errorMessage(error); }
    finally { ending = false; }
  }
</script>

{#if current.placement && current.placement.machine.id !== 'local' && (current.live || current.cleanup_pending)}
  <button type="button" onclick={() => confirmEnd = true}>{current.cleanup_pending ? 'Retry cleanup…' : 'End session…'}</button>
{/if}
{#if renaming}<RenameChat {project} conversation={current} onclose={() => renaming = false} onsaved={onchanged} />{/if}
{#if confirmEnd}
  <Dialog.Root bind:open={confirmEnd}><Dialog.Content>
    <Dialog.Header><Dialog.Title>End {current.title}?</Dialog.Title><Dialog.Description>This ends only this session on {current.placement?.machine.name ?? 'This Mac'}, including any active request, and releases its owned resources after cleanup is confirmed. Saved history and its checkout remain.</Dialog.Description></Dialog.Header>
    {#if failure}<p role="alert">{failure}</p>{/if}
    <Dialog.Footer><button type="button" onclick={() => confirmEnd = false} disabled={ending}>Cancel</button><button type="button" onclick={endSession} disabled={ending}>{ending ? 'Ending…' : 'End session'}</button></Dialog.Footer>
  </Dialog.Content></Dialog.Root>
{/if}

<style>
  button { flex: none; padding: 0; border: 0; background: none; color: inherit; font: inherit; text-decoration: underline; cursor: pointer; }
</style>
