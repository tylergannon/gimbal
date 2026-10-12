<script lang="ts">
  import { FileText, MessageSquare } from '@lucide/svelte';
  import type { WorkspaceTab } from './types.js';
  let { tab }: { tab: WorkspaceTab } = $props();
</script>

{#if tab.kind === 'chat'}<MessageSquare size={14} aria-hidden="true" />{:else}<FileText size={14} aria-hidden="true" />{/if}
