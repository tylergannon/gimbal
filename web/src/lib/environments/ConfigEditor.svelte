<script lang="ts">
  import * as Dialog from '#lib/components/ui/dialog';
  import { environmentConfig } from '#lib/environments.remote';
  import { errorMessage } from '#lib/errors';
  import ConfigForm from './ConfigForm.svelte';
  let { onclose, onsaved }: { onclose: () => void; onsaved: () => void } = $props();
  const document = environmentConfig();
  let reload = $state(0);
</script>

<Dialog.Root open onOpenChange={(open) => { if (!open) onclose(); }}>
  <Dialog.Content class="max-w-3xl max-h-[90vh] overflow-auto">
    <Dialog.Header><Dialog.Title>Environment configuration</Dialog.Title><Dialog.Description>This is the same file used by Machines and environment setup. Plugin settings remain opaque.</Dialog.Description></Dialog.Header>
    {#if document.current}
      {#key reload}<ConfigForm document={document.current} generation={reload} {onsaved} onreload={async () => { await document.refresh(); reload++; }} />{/key}
    {:else if document.error}<p role="alert">{errorMessage(document.error)} <button onclick={() => document.refresh()}>Retry</button></p>
    {:else}<p role="status">Reading configuration…</p>{/if}
  </Dialog.Content>
</Dialog.Root>
