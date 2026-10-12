<script lang="ts">
  import type { WorkspaceTab } from './types.js';
  let { status }: { status: WorkspaceTab['status'] } = $props();
  const labels = { working: 'Working', attention: 'Needs you', unread: 'Unread reply' };
</script>

{#if status}<span class="status {status}" role="img" aria-label={labels[status]}></span>{/if}

<style>
  .status { width: 6px; height: 6px; border-radius: 50%; flex: none; background: var(--foreground); }
  .unread { background: transparent; border: 1.5px solid var(--foreground); }
  .attention { width: 8px; height: 8px; background: transparent; border: 2px solid var(--chart-2); }
</style>
