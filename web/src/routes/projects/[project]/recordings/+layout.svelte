<script lang="ts">
  import { page } from "$app/state";
  import ProjectWorkspace from "#lib/workspace/ProjectWorkspace.svelte";
  let { children } = $props();
</script>

{#key page.params.project}<ProjectWorkspace
    project={page.params.project!}
    id={page.params.recordingID ?? ""}
  />{/key}
{@render children()}
