<script lang="ts">
  import { page } from "$app/state";
  import FileViewer from "#lib/workspace/FileViewer.svelte";
</script>

<svelte:boundary>
  <FileViewer
    project={page.params.project!}
    id={page.params.fileID!}
  />{#snippet pending()}<p>Loading document…</p>{/snippet}
</svelte:boundary>
