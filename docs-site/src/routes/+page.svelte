<script lang="ts">
  import { asset, resolve } from "$app/paths";
  import InstallPrompt from "#lib/components/InstallPrompt.svelte";

  const title = "Gimble — agent workflows in Go";
  const description =
    "Gimble is a Go library for running agent workflows as ordinary Go code.";
  const canonical = "https://tylergannon.github.io/gimble/";
</script>

<svelte:head>
  <title>{title}</title>
  <meta name="description" content={description} />
  <link rel="canonical" href={canonical} />
  <meta property="og:type" content="website" />
  <meta property="og:title" content={title} />
  <meta property="og:description" content={description} />
  <meta property="og:url" content={canonical} />
</svelte:head>

<main>
  <section class="shell flex min-h-[calc(100vh-4rem)] items-center py-16 sm:py-24">
    <div class="w-full">
      <div class="mb-12 flex items-center gap-3">
        <span class="bg-primary size-2 animate-pulse rounded-full"></span>
        <p class="eyebrow">Agent workflows, written as Go</p>
      </div>

      <h1 class="max-w-5xl font-serif text-[clamp(4rem,12vw,9rem)] leading-[0.78] font-medium tracking-[-0.065em]">
        Agent workflows<br /><em class="text-primary font-normal">in Go.</em>
      </h1>

      <div class="mt-12 grid gap-8 lg:grid-cols-[0.8fr_0.7fr_1.2fr] lg:items-end">
        <div class="max-w-md space-y-3 text-lg leading-8">
          <p>Gimble is a Go library for running agent work from ordinary Go code.</p>
          <p class="text-muted-foreground">You write the workflow. Gimble runs the agent sessions, keeps their work inside named scopes, and records what happened.</p>
        </div>
        <figure class="surface overflow-hidden rounded-2xl p-3" aria-label="Gimble mascot">
          <img
            src={asset("gimble-mascot.png")}
            alt="Gimble, a friendly gyroscopic guide, points along a trail toward a goal"
            class="aspect-square w-full rounded-xl object-cover"
            width="768"
            height="768"
          />
        </figure>
        <InstallPrompt />
      </div>

      <div class="border-border mt-14 grid border-y sm:grid-cols-2">
        <a class="group border-border hover:bg-muted/40 flex items-center justify-between gap-4 border-b px-1 py-6 transition-colors sm:border-r sm:border-b-0 sm:px-6" href={resolve("/docs/guide")}>
          <span>
            <span class="eyebrow block">01 / Learn</span>
            <span class="mt-2 block text-lg font-semibold">Read the guide</span>
          </span>
          <span class="text-muted-foreground group-hover:text-primary text-xl transition-colors" aria-hidden="true">↗</span>
        </a>
        <a class="group hover:bg-muted/40 flex items-center justify-between gap-4 px-1 py-6 transition-colors sm:px-6" href={resolve("/docs/about")}>
          <span>
            <span class="eyebrow block">02 / Understand</span>
            <span class="mt-2 block text-lg font-semibold">About Gimble</span>
          </span>
          <span class="text-muted-foreground group-hover:text-primary text-xl transition-colors" aria-hidden="true">↗</span>
        </a>
      </div>
    </div>
  </section>
</main>
