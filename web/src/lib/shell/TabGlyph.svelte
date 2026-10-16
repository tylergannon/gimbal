<script lang="ts">
  import { FileText, MessageSquare, Workflow, ListTree, Mic, ListChecks, BookOpen } from '@lucide/svelte';
  import type { WorkspaceTab } from './types.js';
  let { tab }: { tab: WorkspaceTab } = $props();
  const Icon = $derived(tab.kind === 'run' ? Workflow : tab.kind === 'runs' ? ListTree : tab.kind === 'chat' ? MessageSquare : tab.kind === 'recording' ? Mic : tab.kind === 'promises' ? ListChecks : tab.kind === 'guide' ? BookOpen : FileText);
</script>
<Icon size={14} style="flex-shrink: 0;" aria-hidden="true" />
