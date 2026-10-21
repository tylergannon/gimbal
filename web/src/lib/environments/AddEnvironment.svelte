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
  const checking = $derived(inspectAt?.machine === machine && inspectAt?.directory === path && !!inspection?.loading);
  const machineState = $derived(snapshot.machines.find(item => item.id === machine));
  const localOrigin = $derived(snapshot.environments.find(row => row.environment.project === project && row.environment.machine === 'local')?.directory.origin ?? '');
  let browse = $state(false);
  let browser = $state<{ showDirectory: (path: string) => void }>();
  let error = $state('');
  onMount(() => { if (!id) id = crypto.randomUUID(); });
  async function inspect(directory = path) {
    error = '';
    inspectAt = { project, machine, directory };
    browser?.showDirectory(directory);
    const query = inspectEnvironment(inspectAt);
    try { await query.refresh(); }
    catch (reason) { if (!query.error) error = errorMessage(reason); }
  }
</script>

<Dialog.Root open onOpenChange={(open) => { if (!open) onclose(); }}>
  <Dialog.Content class="sm:max-w-2xl max-h-[90dvh] flex flex-col gap-4 overflow-hidden" style={browse ? 'height: min(760px, 90dvh)' : undefined}>
    <Dialog.Header><Dialog.Title>{environment ? 'Edit environment' : 'Add an environment'}</Dialog.Title><Dialog.Description>Choose a directory on this machine, then check it before adding it to the project.</Dialog.Description></Dialog.Header>
    <form class="environment-form" {...form.enhance(async submitted => { error = ''; try { if (await submitted.submit() && submitted.result) onsaved(submitted.result.environment.id); } catch (reason) { error = errorMessage(reason); } })}>
      <input {...form.fields.revision.as('hidden', observedRevision)} /><input {...form.fields.id.as('hidden', id)} /><input {...form.fields.project.as('hidden', project)} /><input {...form.fields.machine.as('hidden', machine)} />
      <div class="location-fields"><label>Machine<select bind:value={machine} disabled={!!form.pending}>{#each snapshot.machines as item (item.id)}<option value={item.id} disabled={item.state === 'unreachable' || item.state === 'error'}>{item.name}{item.state === 'not-started' ? ' · starts when needed' : ''}</option>{/each}</select></label>
      <label>Directory<input {...form.fields.directory.as('text', environment?.directory ?? '')} required placeholder="/srv/project" disabled={!!form.pending} /></label>
      </div>
      <div class="tools"><button type="button" onclick={() => inspect()} disabled={!path || !!form.pending || checking}>{checking ? 'Checking…' : 'Check directory'}</button><button type="button" onclick={() => (browse = !browse)} disabled={!machineState?.capabilities.browse}>{browse ? 'Hide browser' : 'Browse directories'}</button></div>
      {#if browse}{#key machine}<DirectoryBrowser bind:this={browser} {project} {machine} path={path || '/'} onchoose={(next) => { form.fields.directory.set(next); void inspect(next); }} />{/key}{/if}
      {#if finding}
        <div class="finding" role="status">
          {#if !finding.exists}<p>The directory does not exist yet.</p>
          {:else if finding.git}<p>Git repository on <strong>{finding.branch}</strong>. {finding.git && !finding.unborn && !finding.fixed_workdir ? 'Sessions here can use worktrees.' : 'Sessions share this directory; worktrees are unavailable.'}</p>
          {:else}<p>{finding.unborn ? 'This Git repository has no commit yet.' : 'Not a Git repository.'} Sessions share this directory; worktrees are unavailable.</p>{/if}
          {#if finding.origin}<p>Origin: {finding.origin}. {localOrigin ? originIdentity(finding.origin) === originIdentity(localOrigin) ? 'This matches the project on This Mac.' : `This differs from the project on This Mac (${localOrigin}).` : 'No local origin is available for comparison.'}</p>{/if}
          {#if finding.fixed_workdir}<p>The daemon requires <strong>{finding.execution_directory || finding.fixed_workdir}</strong>. Other directories cannot be used by sessions.</p>{/if}
        </div>
      {:else if inspectAt?.machine === machine && inspectAt?.directory === path && inspection?.error}<p class="error" role="alert">{errorMessage(inspection.error)}</p>{:else}<p role="status">{checking ? 'Checking directory…' : 'Check the selected directory to see its Git and worktree options. This does not change files.'}</p>{/if}
      {#if error}<p class="error" role="alert">{error}</p>{/if}{#each form.fields.allIssues() ?? [] as issue}<p class="error" role="alert">{issue.message}</p>{/each}
      <footer><button type="button" onclick={onclose} disabled={!!form.pending}>Cancel</button><button class="primary" disabled={!!form.pending || !finding?.exists || !id || (!!finding.fixed_workdir && finding.execution_directory !== finding.path)}>{form.pending ? 'Adding…' : environment ? 'Save environment' : 'Add environment'}</button></footer>
    </form>
    {#if finding && (!finding.exists || finding.empty) && localOrigin}
      <div class="clone-option">{#key `${machine}/${path}`}<CloneDirectory {project} {machine} directory={path} origin={localOrigin} oncloned={() => inspection?.refresh()} />{/key}</div>
    {/if}
  </Dialog.Content>
</Dialog.Root>

<style>
  .environment-form { display: flex; flex: 1; flex-direction: column; min-height: 0; gap: 12px; }
  .location-fields { display: grid; flex: none; grid-template-columns: minmax(120px, 1fr) minmax(0, 2fr); gap: 12px; }
  .tools { display: flex; flex: none; gap: 8px; }
  .finding { display: grid; flex: none; max-height: min(120px, 20dvh); overflow-y: auto; gap: 5px; padding: 10px; border: 1px solid var(--border); border-radius: 8px; }
  .environment-form > p, footer { flex: none; }
  .clone-option { flex: none; max-height: 100px; overflow-y: auto; }
  @media (max-width: 480px) { .location-fields { grid-template-columns: minmax(0, 1fr); gap: 8px; } }
</style>
