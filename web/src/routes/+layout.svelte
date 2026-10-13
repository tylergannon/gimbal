<script lang="ts">
  import "../app.css";
  import { navigating, page } from "$app/state";
  import ProjectWorkspace from "#lib/workspace/ProjectWorkspace.svelte";
  import RecorderBridge from "#lib/workspace/RecorderBridge.svelte";
  import favicon from "#lib/assets/favicon.svg";
  import RunsLoading from "#lib/run/RunsLoading.svelte";

  let { children, data } = $props();
  const project = $derived(page.params.project);
  const root = $derived(project ? `/projects/${project}` : "/");
  const loadingRuns = $derived(
    navigating.to?.url.pathname === root &&
      navigating.from?.url.pathname !== root,
  );
</script>

<svelte:head><link rel="icon" href={favicon} /></svelte:head>
<div class={{ desktop: data.desktop }}>
  {#if data.desktop && !project}<aside class="project-sidebar">
      <h2>Gimbal</h2>
      {#each data.workspaceProjects as item (item.id)}<a
          href={`/projects/${item.id}/recordings`}
          >{item.path.split("/").at(-1)}</a
        >{/each}<a href="/">Add project</a>
    </aside>{/if}
  <div class="content">
    {#if !project}<nav class="site-nav">
      <a href="/">Projects</a><a href="/workflows">Workflows</a>{#if project}<a
          href={`${root}/recordings`}>Workspace</a
        ><a href={`${root}/conversations`}>Conversations</a><a href={root}
          >Runs</a
        ><a href={`${root}/about`}>About</a>{/if}
    </nav>{/if}
    <main>
      {#if project}
        {#key project}<ProjectWorkspace {project} name={data.workspaceProjects.find((item: { id: string }) => item.id === project)?.path.split("/").at(-1) ?? "Project"} projects={data.workspaceProjects}>
          {#if loadingRuns}
            <div class="runs-page"><RunsLoading /></div>
          {:else}
            {@render children()}
          {/if}
        </ProjectWorkspace>{/key}
      {:else if loadingRuns}
        <div class="runs-page"><RunsLoading /></div>
      {:else}
        {@render children()}
      {/if}
    </main>
    <RecorderBridge dock={!project} />
  </div>
</div>

<style>
  .desktop {
    display: flex;
    min-height: 100vh;
  }
  .content {
    flex: 1;
    min-width: 0;
    height: 100vh;
    display: flex;
    flex-direction: column;
  }
  .project-sidebar {
    width: 10rem;
    padding: 1rem;
    background: var(--card);
    border-right: 1px solid var(--map-line);
  }
  .project-sidebar h2 {
    font-size: 1rem;
  }
  .project-sidebar a {
    display: block;
    padding: 0.6rem;
    color: var(--foreground);
    text-decoration: none;
  }
  .desktop nav.site-nav {
    padding: 0.5rem 1rem;
    font-size: 0.85rem;
  }
  :global(body) {
    margin: 0;
    font: 16px/1.5 var(--font-sans);
    color: var(--foreground);
    background: var(--background);
  }

  nav {
    display: flex;
    flex-wrap: wrap;
    gap: 1.25rem;
    padding: 0.9rem 2rem;
    border-bottom: 1px solid var(--map-line);
    background: var(--card);
  }

  nav a {
    color: var(--status-live);
    text-decoration: none;
  }

  nav a:hover {
    color: var(--foreground);
  }

  main {
    flex: 1;
    min-height: 0;
    overflow: auto;
    min-width: 0;
  }

  .runs-page {
    display: flex;
    min-width: 0;
    justify-content: center;
    padding: 0 32px;
  }
</style>
