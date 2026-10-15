<script lang="ts">
  import * as Dialog from '#lib/components/ui/dialog';
  import { navigateOnUnauthorized } from '#lib/auth/unauthorized';
  import { errorMessage } from '#lib/errors';
  import { renameConversation, projectConversations } from '../../routes/conversation.remote';

  let { project, conversation, onclose }: {
    project: string; conversation: { id: string; title: string }; onclose: () => void;
  } = $props();
  const form = $derived(renameConversation.for(`${project}/${conversation.id}`));
  let failure = $state('');
</script>

<Dialog.Root open onOpenChange={(open) => { if (!open) onclose(); }}><Dialog.Content>
  <Dialog.Header><Dialog.Title>Rename chat</Dialog.Title><Dialog.Description>Choose a name for this chat.</Dialog.Description></Dialog.Header>
  <form {...form.enhance(async (submitted) => {
    failure = '';
    try { if (await submitted.submit().updates(projectConversations(project))) onclose(); }
    catch (error) { if (!navigateOnUnauthorized(error)) failure = errorMessage(error); }
  })}>
    <input {...form.fields.project.as('hidden', project)} /><input {...form.fields.conversation.as('hidden', conversation.id)} />
    <label>Name<input {...form.fields.title.as('text', conversation.title)} required disabled={!!form.pending} /></label>
    {#if failure}<p role="alert">{failure}</p>{/if}
    {#each form.fields.allIssues() ?? [] as issue}<p role="alert">{issue.message}</p>{/each}
    <Dialog.Footer><button type="button" disabled={!!form.pending} onclick={onclose}>Cancel</button><button type="submit" disabled={!!form.pending}>{form.pending ? 'Renaming…' : 'Rename'}</button></Dialog.Footer>
  </form>
</Dialog.Content></Dialog.Root>

<style>
  label { display: grid; gap: 6px; font-size: 13px; margin-bottom: 18px; }
  input { padding: 8px 10px; border: 1px solid var(--border); border-radius: 6px; color: inherit; background: var(--background); font: inherit; }
  p { color: var(--destructive); font-size: 12px; margin: 8px 0; }
</style>
