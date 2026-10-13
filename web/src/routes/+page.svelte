<script lang="ts">
  import { navigateOnUnauthorized } from "#lib/auth/unauthorized";
  import { errorMessage } from "#lib/errors";
  import { createProject } from "#lib/workspace.remote";
  import { page } from "$app/state";
  import { goto } from "$app/navigation";
  import { APP_NAME } from "#lib/app.js";
  import { Button } from "#lib/components/ui/button/index.js";
  import { Input } from "#lib/components/ui/input/index.js";
  let { data }: { data: { projects: { id: string; path: string }[] } } =
    $props();
  let actionError = $state("");
</script>

<svelte:head><title>Projects — {APP_NAME}</title></svelte:head>
<section class="projects mx-auto max-w-3xl px-6 py-10 text-sm">
  {#if actionError}<p role="alert">{actionError}</p>{/if}
  <h1 class="text-2xl font-semibold tracking-tight">Projects</h1>
  <p class="mt-1 text-muted-foreground">
    A project is a local directory with its recordings, files, conversations
    and runs.
  </p>
  <div class="mt-6 grid gap-px">
    {#each data.projects as project (project.id)}
      <a
        class="flex min-w-0 flex-col rounded-lg px-3 py-2 hover:bg-accent"
        href={`/projects/${project.id}${page.data.desktop ? "/recordings" : ""}`}
        ><span class="font-medium">{project.path.split("/").at(-1)}</span><span
          class="truncate font-mono text-xs text-muted-foreground"
          >{project.path}</span
        ></a
      >
    {:else}
      <p class="px-3 py-2 text-muted-foreground">No projects admitted.</p>
    {/each}
  </div>
  <form
    class="mt-8 grid gap-2 border-t pt-6"
    {...createProject.enhance(async (form) => {
      try {
        if (await form.submit()) {
          if (form.result) await goto(`/projects/${form.result.id}/recordings`);
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
