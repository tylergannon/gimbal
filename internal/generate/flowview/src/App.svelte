<script lang="ts">
  import { SvelteFlowProvider } from '@xyflow/svelte';
  import Workflow from './Workflow.svelte';
  import type { SourcePage } from './types';
  let {page}: {page:SourcePage}=$props();
</script>
<SvelteFlowProvider><Workflow {page}/></SvelteFlowProvider>
