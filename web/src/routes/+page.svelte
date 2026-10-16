<script lang="ts">
  import { navigateOnUnauthorized } from "#lib/auth/unauthorized";
  import { errorMessage } from "#lib/errors";
  import { createProject } from "#lib/workspace.remote";
  import { page } from "$app/state";
  import { goto } from "$app/navigation";
  import { APP_NAME } from "#lib/app.js";
  import { Button } from "#lib/components/ui/button/index.js";
  import { Input } from "#lib/components/ui/input/index.js";
  import ProjectList from "#lib/workspace/ProjectList.svelte";
  import ProjectsEmpty from "#lib/workspace/ProjectsEmpty.svelte";
  import type { ProjectChoice } from "#lib/skgo/host/types";
  let { data }: { data: { projects: ProjectChoice[] } } =
    $props();
  let actionError = $state("");
</script>

<svelte:head><title>Projects — {APP_NAME}</title></svelte:head>
{#if data.projects.length === 0}
  <ProjectsEmpty desktop={page.data.desktop} />
{:else}
<section class="projects mx-auto max-w-3xl px-6 py-10 text-sm">
  {#if actionError}<p role="alert">{actionError}</p>{/if}
  <h1 class="text-2xl font-semibold tracking-tight">Projects</h1>
  <p class="mt-1 text-muted-foreground">
    A project is a local directory with its recordings, files, conversations
    and runs.
  </p>
  <ProjectList
    projects={data.projects}
    href={(project) => `/projects/${project.id}`}
  />
  <form
    class="mt-8 grid gap-2 border-t pt-6"
    {...createProject.enhance(async (form) => {
      try {
        if (await form.submit()) {
          if (form.result) await goto(`/projects/${form.result.id}`);
        }
      } catch (error) {
        if (!navigateOnUnauthorized(error)) actionError = errorMessage(error);
      }
    })}
  >
    <label class="grid gap-2 font-medium"
      >Project directory <Input
        class="font-mono"
        {...createProject.fields.path.as("text")}
        placeholder="/absolute/path/to/project"
        required
      /></label
    >
    <Button class="justify-self-start" type="submit" disabled={!!createProject.pending}
      >Add project</Button
    >
    {#each createProject.fields.allIssues() ?? [] as issue}<p
        class="text-destructive"
        role="alert"
      >
        {issue.message}
      </p>{/each}
  </form>
</section>
{/if}
