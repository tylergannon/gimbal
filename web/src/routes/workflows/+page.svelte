<script lang="ts">
  import { APP_NAME } from "#lib/app.js";
  let { data } = $props();
</script>

<svelte:head><title>Workflows — {APP_NAME}</title></svelte:head>
<section class="mx-auto max-w-3xl px-6 py-10 text-sm">
  <h1 class="text-2xl font-semibold tracking-tight">Workflows</h1>
  <p class="mt-1 text-muted-foreground">
    Understand a workflow, review its steps, and learn how to run it.
  </p>
  <ul class="mt-6 grid gap-px">
    {#each data.workflows as workflow (workflow.name)}<li>
        <a
          class="block rounded-lg px-3 py-2 hover:bg-accent"
          href={`/workflows/${workflow.name}`}
          ><span class="font-medium">{workflow.name}</span>
          <span class="block text-muted-foreground">{workflow.summary}</span></a
        >
      </li>{/each}
  </ul>
</section>
