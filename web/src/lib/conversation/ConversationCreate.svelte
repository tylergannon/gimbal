<script lang="ts">
  import type { Conversation } from '#lib/skgo/conversation/types';
  import { goto } from '$app/navigation';
  import { createConversation, projectConversations } from '../../routes/conversation.remote';
  let { project, oncreated }: { project: string; oncreated: (conversation: Conversation) => void } = $props();
  const form = $derived(createConversation.for(project));
  let provider = $state('codex');
  const defaults: Record<string, string> = { codex: 'gpt-5.6-luna', claude: 'claude-haiku-4-5-20251001', agy: 'gemini-3.8-flash-low' };
  let model = $derived(defaults[provider]);
</script>

<section class="create">
  <div class="heading"><h1>New conversation</h1><p>Choose the agent for this project.</p></div>
  <form {...form.enhance(async (submitted) => {
    if (await submitted.submit().updates(projectConversations(project)) && submitted.result) {
      oncreated(submitted.result);
      await goto(`/projects/${project}/conversations/${submitted.result.id}`);
    }
  })}>
    <label>Title <input {...form.fields.title.as('text')} placeholder="What are we working on?" /></label>
    <label>Provider <select {...form.fields.provider.as('select')} bind:value={provider}><option value="codex">Codex</option><option value="claude">Claude</option><option value="agy">Gemini</option></select></label>
    <label>Model <input {...form.fields.model.as('text')} bind:value={model} /></label>
    {#each form.fields.allIssues() ?? [] as issue}<p role="alert">{issue.message}</p>{/each}
    <button disabled={!!form.pending}>{form.pending ? 'Opening…' : 'Start conversation'}</button>
  </form>
</section>

<style>
  .create { max-width: 460px; margin: 48px auto; padding: 24px; }
  h1 { font-size: 22px; font-weight: 600; margin: 0 0 8px; }
  p { color: var(--muted-foreground); font-size: 13px; }
  form { display: grid; gap: 18px; margin-top: 28px; }
  label { display: grid; gap: 7px; font-size: 13px; }
  input, select { height: 38px; width: 100%; padding: 0 10px; border: 1px solid var(--border); border-radius: 8px; color: inherit; background: var(--background); font: inherit; }
  button { padding: 10px; border: 0; border-radius: 8px; background: var(--primary); color: var(--primary-foreground); font: inherit; cursor: pointer; }
  button:disabled { opacity: .5; }
</style>
