<script lang="ts">
  import { untrack } from 'svelte';
  import type { ConfigDocument } from '#lib/skgo/environments/types';
  import { saveEnvironmentConfig } from '#lib/environments.remote';
  import { errorMessage } from '#lib/errors';
  let { document, generation = 0, onsaved, onreload }: { document: ConfigDocument; generation?: number; onsaved: () => void; onreload: () => void } = $props();
  const original = untrack(() => document);
  const form = untrack(() => saveEnvironmentConfig.for(`${original.path}/${original.revision}/${generation}`));
  let error = $state('');
</script>

<form class="environment-form" {...form.enhance(async submitted => {
  error = '';
  try { if (await submitted.submit()) onsaved(); } catch (reason) { error = errorMessage(reason); }
})}>
  <p><code>{original.path}</code></p>
  <input {...form.fields.revision.as('hidden', original.revision)} />
  {#each original.issues as issue}<p class="error" role="alert">{issue.line ? `Line ${issue.line}: ` : ''}{issue.entry ? `${issue.entry}: ` : ''}{issue.message}</p>{/each}
  <label>JSON<textarea {...form.fields.text.as('text', original.text)} rows={22} spellcheck="false" aria-label="Environment configuration JSON" disabled={!!form.pending}></textarea></label>
  {#if error}<p class="error" role="alert">{error}</p>{/if}
  {#each form.fields.allIssues() ?? [] as issue}<p class="error" role="alert">{issue.message}</p>{/each}
  <footer><button type="button" onclick={onreload} disabled={!!form.pending}>Reload file (discard edits)</button><button class="primary" disabled={!!form.pending}>{form.pending ? 'Saving…' : 'Save file'}</button></footer>
</form>

<style>textarea { font-family: var(--font-mono) !important; font-size: 12px !important; } code { overflow-wrap: anywhere; }</style>
