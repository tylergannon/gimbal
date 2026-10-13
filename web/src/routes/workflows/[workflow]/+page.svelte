<script lang="ts">
  import WorkflowRun from "#lib/workflow-design/WorkflowRun.svelte";
  import WorkflowPage from "#lib/workflow-design/WorkflowPage.svelte";
  import type { SourcePage } from "#lib/workflow-design/types";
  let { data } = $props();
  let workflow = $derived(JSON.parse(data.source_json) as SourcePage);
</script>

<div class="flex h-full flex-col">
  <details class="max-h-[50%] flex-none overflow-auto border-b px-6 py-3 text-sm">
    <summary class="cursor-pointer font-medium">Run {data.name}</summary>
    <WorkflowRun workflow={data} />
  </details>
  {#key workflow.name}<WorkflowPage initial={workflow} />{/key}
</div>
