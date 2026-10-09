<script lang="ts">
  import { SvelteFlowProvider } from '@xyflow/svelte';
  import Workflow from '../../../../web/src/lib/workflow-design/Workflow.svelte';
  import type { SourcePage } from '../../../../web/src/lib/workflow-design/types';
  let {page}: {page:SourcePage}=$props();
</script>
<SvelteFlowProvider><Workflow {page} title/></SvelteFlowProvider>
