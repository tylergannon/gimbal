<script lang="ts">
  import { onMount } from "svelte";
  import { goto } from "$app/navigation";
  import { FolderOpen } from "@lucide/svelte";
  import * as Card from "#lib/components/ui/card/index.js";
  import { Button, buttonVariants } from "#lib/components/ui/button/index.js";
  import { Input } from "#lib/components/ui/input/index.js";
  import { createProject } from "../workspace.remote";
  import OpenFolder from "./OpenFolder.svelte";

  let { desktop }: { desktop: boolean } = $props();
  // The server knows the desktop by its proxy header; only the window has the folder panel.
  let native = $derived(desktop);
  onMount(() => {
    native = desktop && !!window.webkit?.messageHandlers?.chooseProjectFolder;
  });
</script>

<div class="grid min-h-full place-items-center px-6 py-12">
  <Card.Root class="w-full max-w-md">
    <Card.Header>
      <span class="mb-2 grid size-9 place-items-center rounded-[10px] bg-muted text-foreground" aria-hidden="true"
        ><FolderOpen size={18} /></span
      >
      <Card.Title class="text-base">Open a project folder</Card.Title>
      <Card.Description>
        Gimbal works inside a folder on this {native ? "Mac" : "machine"}, usually a repository. Its chats, files, recordings
        and runs stay with it, under <code class="font-mono text-xs">.gimbal</code>.
      </Card.Description>
    </Card.Header>
    <Card.Content>
      {#if native}
        <OpenFolder class={buttonVariants({ size: "lg" })}><FolderOpen aria-hidden="true" />Open folder…</OpenFolder>
      {:else}
        <form
          class="grid gap-2"
          {...createProject.enhance(async (form) => {
            if ((await form.submit()) && form.result) await goto(`/projects/${form.result.id}`, { invalidateAll: true });
          })}
        >
          <label class="text-sm font-medium" for="project-path">Project directory</label>
          <div class="flex gap-2">
            <Input
              id="project-path"
              class="font-mono"
              {...createProject.fields.path.as("text")}
              placeholder="/absolute/path/to/project"
              required
            />
            <Button type="submit" disabled={!!createProject.pending}>Add project</Button>
          </div>
          {#each createProject.fields.allIssues() ?? [] as issue (issue.message)}
            <p class="text-sm text-destructive" role="alert">{issue.message}</p>
          {/each}
        </form>
      {/if}
    </Card.Content>
  </Card.Root>
</div>
