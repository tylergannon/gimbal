<script lang="ts">
 import WorkflowRun from '#lib/workflow-design/WorkflowRun.svelte';
 import WorkflowPage from '#lib/workflow-design/WorkflowPage.svelte';
 import type {SourcePage} from '#lib/workflow-design/types';
 let {data}=$props();
 let workflow=$derived(JSON.parse(data.source_json) as SourcePage);
</script>
<details class="launch"><summary>Run {data.name}</summary><WorkflowRun workflow={data}/></details>
{#key workflow.name}<WorkflowPage initial={workflow}/>{/key}

<style>.launch{padding:12px 24px;border-bottom:1px solid #b1b4b6}.launch summary{font-weight:600;cursor:pointer}</style>
