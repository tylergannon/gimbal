<script lang="ts">
  import type { Snippet } from "svelte";
  import { page } from "$app/state";
  import { resolve } from "$app/paths";
  import { docs as entries, site } from "#lib/site";

  interface Props {
    title: string;
    description: string;
    eyebrow: string;
    headings: { id: string; text: string }[];
    children: Snippet;
  }

  let { title, description, eyebrow, headings, children }: Props = $props();

  const canonical = $derived(new URL(page.url.pathname, site.origin).toString());
  const index = $derived(entries.findIndex((entry) => entry.route === page.route.id));
  const next = $derived(index >= 0 ? entries[index + 1] : undefined);
</script>

<svelte:head>
  <title>{title} · Gimble</title>
  <meta name="description" content={description} />
  <link rel="canonical" href={canonical} />
  <meta property="og:type" content="website" />
  <meta property="og:title" content={`${title} · Gimble`} />
  <meta property="og:description" content={description} />
  <meta property="og:url" content={canonical} />
  <meta property="og:image" content={`${site.base}/og.png`} />
  <meta name="twitter:card" content="summary_large_image" />
</svelte:head>

<div class="shell grid gap-12 pt-8 lg:grid-cols-[13rem_minmax(0,1fr)] xl:grid-cols-[13rem_minmax(0,1fr)_12rem]">
  <aside class="hidden lg:block">
    <div class="sticky top-24">
      <p class="text-muted-foreground mb-4 font-mono text-[0.65rem] font-semibold tracking-[0.16em] uppercase">Documentation</p>
      <nav class="space-y-1" aria-label="Documentation">
        {#each entries as entry}
          <a
            class={page.route.id === entry.route
              ? "border-primary bg-primary/10 text-foreground hover:bg-muted/60 hover:text-foreground block rounded-md border-l-2 px-3 py-2 text-sm transition-colors"
              : "border-transparent text-muted-foreground hover:bg-muted/60 hover:text-foreground block rounded-md border-l-2 px-3 py-2 text-sm transition-colors"}
            href={resolve(entry.route)}
          >{entry.title}</a>
        {/each}
      </nav>
    </div>
  </aside>

  <main class="min-w-0">
    <div class="mb-5 flex flex-wrap items-center gap-3 lg:hidden">
      {#each entries as entry}
        <a
          class={page.route.id === entry.route
            ? "border-primary/50 bg-primary/10 text-primary rounded-full border px-3 py-1.5 text-xs"
            : "border-border text-muted-foreground rounded-full border px-3 py-1.5 text-xs"}
          href={resolve(entry.route)}
        >{entry.title}</a>
      {/each}
    </div>

    <header class="border-border mb-10 border-b pb-9">
      <p class="eyebrow mb-4">{eyebrow}</p>
      <h1 class="font-serif text-4xl leading-[1.02] font-medium tracking-[-0.03em] text-balance sm:text-5xl">{title}</h1>
      <p class="text-muted-foreground mt-5 max-w-3xl text-lg leading-8">{description}</p>
    </header>

    <article class="doc-prose max-w-[52rem]">
      {@render children()}
    </article>

    {#if next}
      <a class="group border-border hover:bg-muted/40 mt-16 flex max-w-[52rem] items-center justify-between rounded-xl border px-6 py-5 transition-colors" href={resolve(next.route)}>
        <span>
          <span class="eyebrow block">Next</span>
          <span class="mt-1 block text-lg font-semibold">{next.title}</span>
        </span>
        <span class="text-muted-foreground group-hover:text-primary text-xl transition-colors" aria-hidden="true">→</span>
      </a>
    {/if}
  </main>

  <aside class="hidden xl:block">
    <div class="sticky top-24">
      <p class="text-muted-foreground mb-4 font-mono text-[0.65rem] font-semibold tracking-[0.16em] uppercase">On this page</p>
      <nav class="space-y-2 border-l pl-4" aria-label="On this page">
        {#each headings as heading}
          <a class="text-muted-foreground hover:text-foreground block text-xs leading-5 transition-colors" href={`#${heading.id}`}>{heading.text}</a>
        {/each}
      </nav>
    </div>
  </aside>
</div>
