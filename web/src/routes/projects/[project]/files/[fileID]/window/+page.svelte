<script lang="ts">
  import { goto } from '$app/navigation';
  import { nativeMessage } from '#lib/workspace/native';
  import { page } from '$app/state';
  import FileViewer from '#lib/workspace/FileViewer.svelte';
</script>
<svelte:boundary>
  <FileViewer project={page.params.project!} id={page.params.fileID!} ondiscuss={() => {
    const path = `/projects/${page.params.project}/files/${page.params.fileID}/discuss`;
    if (window.webkit?.messageHandlers?.discussProjectFile) nativeMessage('discussProjectFile', { path });
    else void goto(path);
  }} />
  {#snippet failed(error, reset)}<p role="alert">Could not open file. <button onclick={reset}>Retry</button></p>{/snippet}
</svelte:boundary>
