<script lang="ts">
  import { resolve } from "$app/paths";
</script>

<svelte:head>
  <title>Moved · Gimble</title>
  <meta http-equiv="refresh" content={`0; url=${resolve("/docs/quickstart")}`} />
  <link rel="canonical" href="https://tylergannon.github.io/gimble/docs/quickstart/" />
</svelte:head>

<main class="shell py-24">
  <p class="text-muted-foreground">The guide moved to the <a class="text-primary underline" href={resolve("/docs/quickstart")}>quickstart</a>.</p>
</main>
