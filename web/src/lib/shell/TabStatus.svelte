<script lang="ts">
  import type { WorkspaceTab } from './types.js';
  import Pip from '#lib/run/Pip.svelte';
  let { status }: { status: WorkspaceTab['status'] } = $props();
  const labels = { queued: 'Queued', running: 'Running', 'needs-you': 'Needs you', ended: 'Ended', failed: 'Failed', cancelled: 'Cancelled' };
</script>

{#if status}<span class="status" role="img" aria-label={labels[status]}><Pip state={status} /></span>{/if}

<style>
  .status { display:inline-flex; align-items:center; flex:none; }
</style>
