<script lang="ts">
  let { data }: { data: { projects: { id: string; path: string }[] } } = $props();
</script>

<svelte:head><title>Projects — Gimble</title></svelte:head>
<section class="projects">
  <h1>Projects</h1>
  {#each data.projects as project (project.id)}
    <a href={`/projects/${project.id}`}>{project.path}</a>
  {:else}
    <p>No projects admitted.</p>
  {/each}
</section>

<style>
  .projects { max-width: 760px; margin: 3rem auto; padding: 0 1.5rem; }
  .projects a { display: block; padding: .75rem; color: var(--status-live); }
</style>
