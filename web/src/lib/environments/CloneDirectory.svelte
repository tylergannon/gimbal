<script lang="ts">
  import { onMount } from 'svelte';
  import { cloneEnvironment } from '#lib/environments.remote';
  import { errorMessage } from '#lib/errors';
  let { project, machine, directory, origin, oncloned }: { project: string; machine: string; directory: string; origin: string; oncloned: () => void } = $props();
  const form = $derived(cloneEnvironment.for(`${project}/${machine}/${directory}`));
  let operation = $state('');
  let error = $state('');
  onMount(() => { operation = crypto.randomUUID(); });
</script>

<form class="environment-form" {...form.enhance(async submitted => { error = ''; try { if (await submitted.submit()) oncloned(); } catch (reason) { error = errorMessage(reason); } })}>
  <input {...form.fields.project.as('hidden', project)} /><input {...form.fields.machine.as('hidden', machine)} /><input {...form.fields.directory.as('hidden', directory)} /><input {...form.fields.origin.as('hidden', origin)} /><input {...form.fields.operation.as('hidden', operation)} />
  <p>This directory is empty or missing. Clone <code>{origin}</code> here using this machine's existing Git credentials.</p>
  <button type="submit" disabled={!!form.pending || !operation}>{form.pending ? 'Cloning…' : 'Clone project here'}</button>
  {#if error}<p class="error" role="alert">{error} Retrying retains this clone operation's identity.</p>{/if}
</form>
