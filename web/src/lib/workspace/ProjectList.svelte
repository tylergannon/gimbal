<script lang="ts">
  import GripVerticalIcon from "@lucide/svelte/icons/grip-vertical";
  import { invalidateAll } from "$app/navigation";
  import { navigateOnUnauthorized } from "#lib/auth/unauthorized";
  import { errorMessage } from "#lib/errors";
  import type { ProjectChoice } from "#lib/skgo/host/types";
  import { reorderProjects } from "#lib/workspace.remote";

  let { projects, href }: { projects: ProjectChoice[]; href: (project: ProjectChoice) => string } =
    $props();
  // Shows a move at once; the server keeps the order for every window and the next launch.
  let order = $derived(projects);
  let dragging = $state<string>();
  let over = $state<number>();
  let error = $state("");

  const name = (project: ProjectChoice) => project.path.split("/").at(-1) ?? project.path;

  async function move(id: string, to: number) {
    const from = order.findIndex((project) => project.id === id);
    if (from < 0 || to < 0 || to >= order.length || from === to) return;
    const previous = order;
    const next = [...order];
    next.splice(to, 0, ...next.splice(from, 1));
    order = next;
    error = "";
    try {
      await reorderProjects({ ids: next.map((project) => project.id) });
      await invalidateAll();
    } catch (cause) {
      order = previous;
      if (!navigateOnUnauthorized(cause)) error = errorMessage(cause);
    }
  }
</script>

{#if error}<p class="text-destructive" role="alert">{error}</p>{/if}
<ol class="mt-6 grid gap-px" aria-label="Projects">
  {#each order as project, index (project.id)}
    <li
      class={["row flex min-w-0 items-center rounded-lg hover:bg-accent", over === index && dragging !== project.id && "over"]}
      draggable="true"
      ondragstart={(event) => {
        dragging = project.id;
        event.dataTransfer?.setData("text/plain", project.path);
      }}
      ondragover={(event) => {
        if (!dragging) return;
        event.preventDefault();
        over = index;
      }}
      ondragleave={() => {
        if (over === index) over = undefined;
      }}
      ondrop={(event) => {
        event.preventDefault();
        if (dragging) void move(dragging, index);
        dragging = over = undefined;
      }}
      ondragend={() => (dragging = over = undefined)}
    >
      <button
        type="button"
        class="grip flex h-9 w-7 flex-none cursor-grab items-center justify-center text-muted-foreground"
        aria-label={`Move ${name(project)}`}
        title="Drag to reorder, or use the arrow keys"
        onkeydown={(event) => {
          const to = { ArrowUp: index - 1, ArrowDown: index + 1 }[event.key];
          if (to === undefined) return;
          event.preventDefault();
          void move(project.id, to);
        }}><GripVerticalIcon size={14} aria-hidden="true" /></button
      >
      <a class="flex min-w-0 flex-1 flex-col py-2 pr-3" href={href(project)} draggable="false"
        ><span class="font-medium">{name(project)}</span><span
          class="truncate font-mono text-xs text-muted-foreground">{project.path}</span
        ></a
      >
    </li>
  {/each}
</ol>

<style>
  .row.over {
    box-shadow: inset 0 2px 0 var(--primary);
  }
  .grip:active {
    cursor: grabbing;
  }
</style>
