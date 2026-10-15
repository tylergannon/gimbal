<script lang="ts">
  import { onMount, untrack } from 'svelte';
  import * as Dialog from '#lib/components/ui/dialog';
  import { saveEnvironment, inspectEnvironment } from '#lib/environments.remote';
  import { errorMessage } from '#lib/errors';
  import type { Snapshot, EnvironmentInfo } from '#lib/skgo/environments/types';
  import DirectoryBrowser from './DirectoryBrowser.svelte';
  import CloneDirectory from './CloneDirectory.svelte';
  import { originIdentity } from './navigation';
  let { snapshot, project, environment, machineID = '', onclose, onsaved }: { snapshot: Snapshot; project: string; environment?: EnvironmentInfo; machineID?: string; onclose: () => void; onsaved: (id: string) => void } = $props();
  const observedRevision = untrack(() => snapshot.revision);
  const form = untrack(() => saveEnvironment.for(environment?.id ?? `${project}/new`));
  let id = $state(untrack(() => environment?.id ?? ''));
  let machine = $state(untrack(() => environment?.machine ?? (machineID || snapshot.machines.find(item => item.id !== 'local')?.id || 'local')));
  const path = $derived(form.fields.directory.value() ?? environment?.directory ?? '');
  let inspectAt = $state<{ project: string; machine: string; directory: string }>();
  const inspection = $derived(inspectAt ? inspectEnvironment(inspectAt) : undefined);
  const finding = $derived(inspectAt?.machine === machine && inspectAt?.directory === path ? inspection?.current : undefined);
  const machineState = $derived(snapshot.machines.find(item => item.id === machine));
  const localOrigin = $derived(snapshot.environments.find(row => row.environment.project === project && row.environment.machine === 'local')?.directory.origin ?? '');
  let browse = $state(false);
  let error = $state('');
  onMount(() => { if (!id) id = crypto.randomUUID(); });
  function inspect() { inspectAt = { project, machine, directory: path }; }
</script>

<Dialog.Root open onOpenChange={(open) => { if (!open) onclose(); }}>
  <Dialog.Content class="max-w-2xl max-h-[90vh] overflow-auto">
    <Dialog.Header><Dialog.Title>{environment ? 'Edit environment' : 'Add an environment'}</Dialog.Title><Dialog.Description>An environment is a directory on a machine. Inspect it before adding it to this project.</Dialog.Description></Dialog.Header>
    <form class="environment-form" {...form.enhance(async submitted => { error = ''; try { if (await submitted.submit() && submitted.result) onsaved(submitted.result.environment.id); } catch (reason) { error = errorMessage(reason); } })}>
      <input {...form.fields.revision.as('hidden', observedRevision)} /><input {...form.fields.id.as('hidden', id)} /><input {...form.fields.project.as('hidden', project)} /><input {...form.fields.machine.as('hidden', machine)} />
      <label>Machine<select bind:value={machine} disabled={!!form.pending}>{#each snapshot.machines as item (item.id)}<option value={item.id} disabled={item.state === 'unreachable' || item.state === 'error' || item.state === 'starting' || item.state === 'connecting'}>{item.name}{item.state === 'not-started' ? ' · starts when needed' : ''}</option>{/each}</select></label>
      <label>Directory<input {...form.fields.directory.as('text', environment?.directory ?? '')} required placeholder="/srv/project" disabled={!!form.pending} /></label>
      <div class="tools"><button type="button" onclick={inspect} disabled={!path || !!form.pending}>Inspect directory</button><button type="button" onclick={() => (browse = !browse)} disabled={!machineState?.capabilities.browse}>{browse ? 'Hide browser' : 'Browse directories'}</button></div>
      {#if browse}{#key machine}<DirectoryBrowser {project} {machine} path={path || '/'} onchoose={(next) => { form.fields.directory.set(next); inspectAt = { project, machine, directory: next }; }} />{/key}{/if}
      {#if finding}
        <div class="finding" role="status">
          {#if !finding.exists}<p>The directory does not exist yet.</p>
          {:else if finding.git}<p>Git repository on <strong>{finding.branch}</strong>. {finding.git && !finding.unborn && !finding.fixed_workdir ? 'Sessions here can use worktrees.' : 'Sessions share this directory; worktrees are unavailable.'}</p>
          {:else}<p>{finding.unborn ? 'This Git repository has no commit yet.' : 'Not a Git repository.'} Sessions share this directory; worktrees are unavailable.</p>{/if}
          {#if finding.origin}<p>Origin: {finding.origin}. {localOrigin ? originIdentity(finding.origin) === originIdentity(localOrigin) ? 'This matches the project on This Mac.' : `This differs from the project on This Mac (${localOrigin}).` : 'No local origin is available for comparison.'}</p>{/if}
          {#if finding.fixed_workdir}<p>The daemon requires <strong>{finding.execution_directory || finding.fixed_workdir}</strong>. Other directories cannot be used by sessions.</p>{/if}
        </div>
      {:else if inspectAt && inspection?.error}<p class="error" role="alert">{errorMessage(inspection.error)}</p>{:else if inspectAt && !finding}<p role="status">{inspection?.loading ? 'Inspecting directory…' : 'Inspect the current directory before adding it.'}</p>{/if}
      {#if error}<p class="error" role="alert">{error}</p>{/if}{#each form.fields.allIssues() ?? [] as issue}<p class="error" role="alert">{issue.message}</p>{/each}
      <footer><button type="button" onclick={onclose} disabled={!!form.pending}>Cancel</button><button class="primary" disabled={!!form.pending || !finding?.exists || !id || (!!finding.fixed_workdir && finding.execution_directory !== finding.path)}>{form.pending ? 'Adding…' : environment ? 'Save environment' : 'Add environment'}</button></footer>
    </form>
    {#if finding && (!finding.exists || finding.empty) && localOrigin}
      {#key `${machine}/${path}`}<CloneDirectory {project} {machine} directory={path} origin={localOrigin} oncloned={() => inspection?.refresh()} />{/key}
    {/if}
  </Dialog.Content>
</Dialog.Root>

<style>.tools { display: flex; gap: 8px; } .finding { display: grid; gap: 5px; padding: 12px; border: 1px solid var(--border); border-radius: 8px; }</style>
