<script lang="ts">
  import { page } from '$app/state';
  import FileViewer from '#lib/workspace/FileViewer.svelte';
</script>
<svelte:boundary>
  <FileViewer project={page.params.project!} id={page.params.fileID!} />
  {#snippet failed(error, reset)}<p role="alert">Could not open file. <button onclick={reset}>Retry</button></p>{/snippet}
</svelte:boundary>
