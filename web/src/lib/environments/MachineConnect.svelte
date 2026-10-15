<script lang="ts">
  import { onMount, untrack } from 'svelte';
  import * as Dialog from '#lib/components/ui/dialog';
  import { saveMachine, sshHosts, watchMachineConnection, closeMachineConnection } from '#lib/environments.remote';
  import { errorMessage } from '#lib/errors';
  import type { ConnectionProgress } from '#lib/skgo/environments/types';
  import type { MachineConfiguration } from './configuration';
  import ConnectionChecks from './ConnectionChecks.svelte';

  let { revision, machine, onclose, onsaved, onconfig }: { revision: string; machine?: MachineConfiguration; onclose: () => void; onsaved: (id: string) => void; onconfig: () => void } = $props();
  const observedRevision = untrack(() => revision);
  const form = untrack(() => saveMachine.for(machine?.id ?? 'new-machine'));
  let id = $state(untrack(() => machine?.id ?? ''));
  let kind = $state(untrack(() => machine?.kind ?? 'ssh'));
  let error = $state('');
  let changedAfterCheck = $state(false);
  let showingChecks = $state(false);
  let previousAttempt = $state('');
  let checked = $state<ConnectionProgress>();
  let terminal = $state(false);
  let view = $state('');
  const connection = $derived(id && view ? watchMachineConnection({ machine: id, view }) : undefined);
  $effect(() => { const token = view; if (token) return () => { void closeMachineConnection(token).catch(() => {}); }; });
  const progress = $derived(showingChecks && connection?.current?.attempt !== previousAttempt ? connection?.current : checked);
  const canConnect = $derived(showingChecks && progress?.complete && progress.can_connect && !changedAfterCheck && !form.pending);
  const hosts = sshHosts();
  onMount(() => { if (!id) id = crypto.randomUUID(); view = crypto.randomUUID(); });
  function openTerminal() {
    const host = form.fields.host.value() ?? machine?.ssh?.host ?? '';
    const handler = (window as Window & { webkit?: { messageHandlers?: { openMachineTerminal?: { postMessage: (body: { host: string; port: number }) => void } } } }).webkit?.messageHandlers?.openMachineTerminal;
    if (kind === 'ssh' && host && handler) handler.postMessage({ host, port: form.fields.port.value() ?? machine?.ssh?.port ?? 0 });
    else terminal = true;
  }
</script>

<Dialog.Root open onOpenChange={(open) => { if (!open) onclose(); }}>
  <Dialog.Content class="max-w-lg max-h-[90vh] overflow-auto">
    <Dialog.Header><Dialog.Title>{showingChecks ? `Connecting to ${form.fields.name.value() || machine?.name || 'machine'}` : machine ? 'Edit machine' : 'Add a machine'}</Dialog.Title><Dialog.Description>{showingChecks ? 'Each line records what Gimbal actually checked. Nothing is saved until the checks pass or you confirm a nonfatal warning.' : 'Gimbal uses your existing network and SSH trust. Harness sign-in belongs to the machine where it runs.'}</Dialog.Description></Dialog.Header>
    <form class="environment-form" oninput={() => (changedAfterCheck = true)} {...form.enhance(async (submitted) => {
      error = '';
      if (!submitted.fields.connect_anyway.value()) { previousAttempt = connection?.current?.attempt ?? ''; checked = undefined; changedAfterCheck = false; }
      showingChecks = true;
      try { if (await submitted.submit() && submitted.result) { changedAfterCheck = false; checked = submitted.result; if (submitted.result.saved) onsaved(submitted.result.id); } }
      catch (reason) { error = errorMessage(reason); }
    })}>
      <input {...form.fields.revision.as('hidden', observedRevision)} />
      <input {...form.fields.id.as('hidden', id)} />
      <input {...form.fields.attempt.as('hidden', progress?.attempt ?? '')} />
      <input {...form.fields.kind.as('hidden', kind)} />
      <div class="connection-fields" hidden={showingChecks}>
      <label>Name<input {...form.fields.name.as('text', machine?.name ?? '')} required placeholder="Build box" disabled={!!form.pending} /></label>
      <fieldset disabled={!!form.pending}><legend>Connection</legend>
        <label class="radio"><input type="radio" bind:group={kind} value="ssh" />Over SSH <span>Recommended</span></label>
        <p>Uses system SSH configuration, agent and known hosts. Gimbal tunnels to the daemon on loopback and can start an already installed Gimbal there.</p>
        {#if kind === 'ssh'}
          <label>Host<input {...form.fields.host.as('text', machine?.ssh?.host ?? '')} required placeholder="build-box" list="ssh-host-suggestions" /></label>
          <datalist id="ssh-host-suggestions">{#each hosts.current ?? [] as host (host)}<option value={host}></option>{/each}</datalist>
          <div class="pair"><label>SSH port (0 uses SSH config)<input {...form.fields.port.as('number', machine?.ssh?.port ?? 0)} min="0" max="65535" /></label><label>Daemon port<input {...form.fields.daemon_port.as('number', machine?.ssh?.daemon_port ?? 8081)} min="1" max="65535" /></label></div>
          <label>Workspace root (optional)<input {...form.fields.workspace_root.as('text', machine?.ssh?.workspace_root ?? '')} placeholder="Use the daemon's default" /></label>
        {:else}<input {...form.fields.workspace_root.as('hidden', '')} /><input {...form.fields.host.as('hidden', '')} /><input {...form.fields.port.as('hidden', 0)} /><input {...form.fields.daemon_port.as('hidden', 0)} />{/if}
        <label class="radio"><input type="radio" bind:group={kind} value="address" />By address</label>
        {#if kind === 'address'}<p>Anyone who can reach this daemon can execute trusted requests as its user. Use an address on a trusted private network.</p><label>Daemon address<input {...form.fields.address.as('url', machine?.address ?? '')} required placeholder="http://studio.internal:8081" /></label>{:else}<input {...form.fields.address.as('hidden', '')} />{/if}
      </fieldset>
      <label>Gimbal executable<input {...form.fields.executable.as('text', machine?.executable ?? '')} placeholder="gimbal" disabled={!!form.pending} /></label>
      <p>Queues and other plugin routes are configured in <button type="button" class="text-button" onclick={onconfig}>environments.json</button>. Their settings are preserved unchanged.</p>
      </div>
      {#if showingChecks}<ConnectionChecks {progress} />{/if}
      {#if connection?.error}<p class="error" role="alert">Connection check updates were lost. <button type="button" class="text-button" onclick={() => connection?.reconnect()}>Reconnect updates</button></p>{/if}
      {#if error}<p role="alert" class="error">{error}</p>{/if}
      {#if progress?.checks.some(check => check.state === 'warning' || check.fatal)}<button type="button" class="text-button" onclick={openTerminal}>{kind === 'ssh' ? 'Open in Terminal' : 'Terminal instructions'}</button>{/if}
      {#if showingChecks}<button type="button" class="text-button" onclick={onconfig}>Open configuration</button>{/if}
      {#if terminal}<p role="status">{kind === 'ssh' ? `Open Terminal and connect to ${form.fields.host.value() ?? machine?.ssh?.host ?? ''} with SSH to inspect Gimbal and harness sign-in.` : 'Open a terminal on the machine running the daemon to inspect Gimbal and harness sign-in.'}</p>{/if}
      {#each form.fields.allIssues() ?? [] as issue}<p role="alert" class="error">{issue.message}</p>{/each}
      <footer>
        {#if progress?.saved}
          <p role="status">Machine saved.</p><button type="button" class="primary" onclick={() => onsaved(id)}>Done</button>
        {:else}
          {#if showingChecks}<button type="button" onclick={() => { showingChecks = false; changedAfterCheck = true; }} disabled={!!form.pending}>Edit settings</button>{/if}
          <button type="button" onclick={onclose}>Close</button>
          {#if canConnect}<button {...form.fields.connect_anyway.as('submit', false)}>Check again</button>{/if}
          <button class="primary" {...form.fields.connect_anyway.as('submit', !!canConnect)} disabled={!!form.pending || !id || (showingChecks && !!progress?.attempt && !progress.complete)}>{form.pending ? 'Checking connection…' : canConnect ? 'Connect anyway' : showingChecks ? 'Check again' : 'Connect'}</button>
        {/if}
      </footer>
    </form>
  </Dialog.Content>
</Dialog.Root>

<style>
  .connection-fields { display: grid; gap: 14px; }
  .connection-fields[hidden] { display: none; }
</style>
