<script lang="ts">
  import { SvelteFlowProvider } from '@xyflow/svelte';
  import Workflow from './Workflow.svelte';
  import type { SourcePage } from './types';
  import type { ViewState } from './view-state';
  let { page, initialView, oncommit }: {
    page: SourcePage;
    initialView: ViewState;
    oncommit: (state: ViewState, replace: boolean) => Promise<void>;
  } = $props();
  let viewer: Workflow;
  export async function show(page: SourcePage, view: ViewState) {
    await viewer?.show(page, view);
  }
</script>

<SvelteFlowProvider><Workflow bind:this={viewer} {page} {initialView} {oncommit}/></SvelteFlowProvider>
