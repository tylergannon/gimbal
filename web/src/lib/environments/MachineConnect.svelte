<script lang="ts">
  import { onMount, untrack } from 'svelte';
  import * as Dialog from '#lib/components/ui/dialog';
  import { saveMachine, sshHosts } from '#lib/environments.remote';
  import { errorMessage } from '#lib/errors';
  import type { MachineConfiguration } from './configuration';

  let { revision, machine, onclose, onsaved, onconfig }: { revision: string; machine?: MachineConfiguration; onclose: () => void; onsaved: (id: string) => void; onconfig: () => void } = $props();
  const observedRevision = untrack(() => revision);
  const form = untrack(() => saveMachine.for(machine?.id ?? 'new-machine'));
  let id = $state(untrack(() => machine?.id ?? ''));
  let kind = $state(untrack(() => machine?.kind ?? 'ssh'));
  let error = $state('');
  let connectAnyway = $state(false);
  const hosts = sshHosts();
  onMount(() => { if (!id) id = crypto.randomUUID(); });
</script>

<Dialog.Root open onOpenChange={(open) => { if (!open) onclose(); }}>
  <Dialog.Content class="max-w-lg">
    <Dialog.Header><Dialog.Title>{machine ? 'Edit machine' : 'Add a machine'}</Dialog.Title><Dialog.Description>Gimbal uses your existing network and SSH trust. Harness sign-in belongs to the machine where it runs.</Dialog.Description></Dialog.Header>
    <form class="environment-form" {...form.enhance(async (submitted) => {
      error = '';
      try { if (await submitted.submit() && submitted.result) onsaved(submitted.result.id); }
      catch (reason) { error = errorMessage(reason); }
    })}>
      <input {...form.fields.revision.as('hidden', observedRevision)} />
      <input {...form.fields.id.as('hidden', id)} />
      <input {...form.fields.kind.as('hidden', kind)} />
      <input {...form.fields.connect_anyway.as('hidden', connectAnyway)} />
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
      <p>Queues and other plugin routes are configured in <button type="button" class="text-button" onclick={onconfig}>the configuration file</button>. Their settings are preserved unchanged.</p>
      {#if error}<p role="alert" class="error">{error}</p><label class="radio"><input type="checkbox" bind:checked={connectAnyway} />Save even if unreachable; session choices stay disabled.</label>{/if}
      {#each form.fields.allIssues() ?? [] as issue}<p role="alert" class="error">{issue.message}</p>{/each}
      <footer><button type="button" onclick={onclose} disabled={!!form.pending}>Cancel</button><button class="primary" type="submit" disabled={!!form.pending || !id}>{form.pending ? 'Connecting…' : connectAnyway ? 'Connect anyway' : 'Connect'}</button></footer>
    </form>
  </Dialog.Content>
</Dialog.Root>
