<script lang="ts">
 import WorkflowPage from '#lib/workflow-design/WorkflowPage.svelte';
 import type {SourcePage} from '#lib/workflow-design/types';
 let {data}=$props();
 let workflow=$derived(JSON.parse(data.source_json) as SourcePage);
</script>
{#key workflow.name}<WorkflowPage initial={workflow}/>{/key}
