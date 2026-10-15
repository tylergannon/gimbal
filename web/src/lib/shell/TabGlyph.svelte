<script lang="ts">
  import { FileText, MessageSquare, Workflow } from '@lucide/svelte';
  import type { WorkspaceTab } from './types.js';
  let { tab }: { tab: WorkspaceTab } = $props();
</script>

{#if tab.kind === 'run'}<Workflow size={14} style="flex-shrink: 0;" aria-hidden="true" />{:else if tab.kind === 'chat'}<MessageSquare size={14} style="flex-shrink: 0;" aria-hidden="true" />{:else}<FileText size={14} style="flex-shrink: 0;" aria-hidden="true" />{/if}
