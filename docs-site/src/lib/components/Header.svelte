<script lang="ts">
  import { page } from "$app/state";
  import { asset, resolve } from "$app/paths";

  const nav = [
    { route: "/docs/guide" as const, label: "Guide" },
    { route: "/docs/about" as const, label: "About" },
  ];
</script>

<header class="border-border/80 sticky top-0 z-50 border-b bg-[oklch(0.135_0.01_55/0.88)] backdrop-blur-xl">
  <div class="shell flex h-16 items-center justify-between gap-5">
    <a class="group inline-flex items-center gap-3" href={resolve("/")} aria-label="Gimble home">
      <span class="border-primary/50 bg-primary/10 text-primary grid size-8 place-items-center rounded-md border">
        <img class="size-4" src={asset("favicon.svg")} alt="" />
      </span>
      <span class="font-mono text-sm font-bold tracking-[0.16em] uppercase">Gimble</span>
    </a>

    <nav class="hidden items-center gap-6 sm:flex" aria-label="Primary navigation">
      {#each nav as item}
        <a
          class:text-primary={page.route.id === item.route}
          class:text-muted-foreground={page.route.id !== item.route}
          class="hover:text-foreground text-sm transition-colors"
          href={resolve(item.route)}
        >{item.label}</a>
      {/each}
    </nav>

    <a class="text-muted-foreground hover:text-foreground inline-flex items-center gap-2 text-sm transition-colors" href="https://github.com/tylergannon/gimble">
      <svg class="size-4" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
        <path d="M12 .7a12 12 0 0 0-3.8 23.4c.6.1.8-.3.8-.6v-2.3c-3.3.7-4-1.4-4-1.4-.5-1.4-1.3-1.8-1.3-1.8-1.1-.7.1-.7.1-.7 1.2.1 1.8 1.2 1.8 1.2 1.1 1.8 2.8 1.3 3.5 1 .1-.8.4-1.3.8-1.6-2.7-.3-5.5-1.3-5.5-5.9 0-1.3.5-2.4 1.2-3.2-.1-.3-.5-1.5.1-3.2 0 0 1-.3 3.3 1.2a11.5 11.5 0 0 1 6 0c2.3-1.5 3.3-1.2 3.3-1.2.7 1.7.3 2.9.1 3.2.8.8 1.2 1.9 1.2 3.2 0 4.6-2.8 5.6-5.5 5.9.4.4.8 1.1.8 2.2v3.3c0 .3.2.7.8.6A12 12 0 0 0 12 .7Z" />
      </svg>
      <span class="hidden md:inline">GitHub</span>
    </a>
  </div>
</header>
