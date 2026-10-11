<script lang="ts">
  import { viewFile } from "../workspace.remote";
  import { nativeMessage } from "./native";
  let { project, id }: { project: string; id: string } = $props();
  const view = $derived(await viewFile({ project, id }));
  function separate() {
    const path = `/projects/${project}/files/${id}`;
    if (window.webkit?.messageHandlers?.openFile)
      nativeMessage("openFile", { path });
    else window.open(path, "_blank");
  }
</script>

<section class="viewer" aria-label="Document viewer">
  <header>
    <strong>{view.file.name}</strong><button onclick={separate}
      >Separate window</button
    ><a href={view.url} target="_blank" rel="noreferrer">Open / download</a
    ><button onclick={() => viewFile({ project, id }).refresh()}
      >Refresh link</button
    >
  </header>
  {#if view.file.mediaType.startsWith("image/")}<img
      src={view.url}
      alt={view.file.name}
    />
  {:else if view.html}<article>{@html view.html}</article>
  {:else if view.file.mediaType.startsWith("text/")}<pre>{view.text}</pre>
  {:else if view.file.mediaType === "application/pdf"}<iframe
      title={view.file.name}
      src={view.url}
    ></iframe>
  {:else}<p>Use Open / download to view this file.</p>{/if}
</section>

<style>
  .viewer {
    height: 100%;
    display: flex;
    flex-direction: column;
    min-width: 0;
  }
  header {
    display: flex;
    flex-wrap: wrap;
    gap: 0.5rem;
    align-items: center;
    padding: 0.7rem;
    border-bottom: 1px solid var(--map-line);
  }
  strong {
    flex: 1;
    font-size: 0.85rem;
  }
  a {
    color: var(--status-live);
    font-size: 0.8rem;
  }
  button {
    font-size: 0.75rem;
  }
  iframe {
    border: 0;
    flex: 1;
    min-height: 30rem;
    width: 100%;
  }
  img {
    max-width: 100%;
    max-height: 75vh;
    object-fit: contain;
  }
  article {
    padding: 1rem;
    overflow: auto;
  }
  pre {
    white-space: pre-wrap;
    padding: 1rem;
    overflow: auto;
  }
</style>
