<script lang="ts">
  import { navigateOnUnauthorized } from "#lib/auth/unauthorized";
  import { errorMessage } from "#lib/errors";
  import { projectFiles, uploadFile } from "../workspace.remote";
  import FileViewer from "./FileViewer.svelte";
  let { project }: { project: string } = $props();
  const files = $derived(await projectFiles(project));
  const form = $derived(uploadFile.for(project));
  let selected = $state("");
  let actionError = $state("");
</script>

<aside class="library" aria-label="Project files">
  {#if actionError}<p role="alert">{actionError}</p>{/if}
  <h2>Files</h2>
  <form
    {...form.enhance(async (submitted) => {
      try {
        if (await submitted.submit().updates(projectFiles(project))) {
          if (submitted.result) selected = submitted.result.id;
        }
      } catch (error) {
        if (!navigateOnUnauthorized(error)) actionError = errorMessage(error);
      }
    })}
  >
    <input {...form.fields.project.as("hidden", project)} /><input
      aria-label="Choose project file"
      {...form.fields.file.as("file")}
      required
    /><button disabled={!!form.pending}
      >{form.pending ? "Uploading…" : "Upload"}</button
    >
  </form>
  {#each form.fields.allIssues() ?? [] as issue}<p role="alert">
      {issue.message}
    </p>{/each}
  <nav>
    {#each files as file (file.id)}<button onclick={() => (selected = file.id)}
        >{file.name}</button
      >{:else}<p>No files yet.</p>{/each}
  </nav>
  {#if selected}<svelte:boundary onerror={(error) => { navigateOnUnauthorized(error); }}>
      <FileViewer {project} id={selected} />{#snippet failed(error, reset)}<p
          role="alert"
        >
          Could not open file. <button onclick={reset}>Retry</button>
        </p>{/snippet}
    </svelte:boundary>{/if}
</aside>

<style>
  .library {
    overflow: auto;
    width: 32%;
    min-width: 16rem;
    border-left: 1px solid var(--map-line);
    padding: 1rem;
    display: flex;
    flex-direction: column;
    gap: 0.6rem;
  }
  h2 {
    font-size: 1.1rem;
    margin: 0;
  }
  form {
    display: flex;
    flex-wrap: wrap;
    gap: 0.4rem;
  }
  input {
    max-width: 100%;
    font-size: 0.75rem;
  }
  nav {
    display: flex;
    flex-wrap: wrap;
    gap: 0.4rem;
  }
  button {
    padding: 0.35rem 0.6rem;
    font-size: 0.8rem;
  }
  p {
    font-size: 0.85rem;
  }
</style>
