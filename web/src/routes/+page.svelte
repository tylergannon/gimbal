<script lang="ts">
  import { createProject } from "#lib/workspace.remote";
  import { page } from "$app/state";
  import { goto } from "$app/navigation";
  let { data }: { data: { projects: { id: string; path: string }[] } } =
    $props();
</script>

<svelte:head><title>Projects — Gimbal</title></svelte:head>
<section class="projects">
  <h1>Projects</h1>
  {#each data.projects as project (project.id)}
    <a href={`/projects/${project.id}${page.data.desktop ? "/recordings" : ""}`}
      >{project.path}</a
    >
  {:else}
    <p>No projects admitted.</p>
  {/each}
  <form
    {...createProject.enhance(async (form) => {
      if (await form.submit()) {
        if (form.result) await goto(`/projects/${form.result.id}/recordings`);
      }
    })}
  >
    <label
      >Project directory <input
        {...createProject.fields.path.as("text")}
        placeholder="/absolute/path/to/project"
        required
      /></label
    ><button disabled={!!createProject.pending}>Add project</button>
  </form>
  {#each createProject.fields.allIssues() ?? [] as issue}<p role="alert">
      {issue.message}
    </p>{/each}
</section>

<style>
  .projects {
    max-width: 760px;
    margin: 3rem auto;
    padding: 0 1.5rem;
  }
  .projects a {
    display: block;
    padding: 0.75rem;
    color: var(--status-live);
  }
</style>
