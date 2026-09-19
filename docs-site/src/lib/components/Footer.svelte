<script lang="ts">
  import { asset, resolve } from "$app/paths";
  import { docs, site } from "#lib/site";

  const year = new Date().getUTCFullYear();
</script>

<footer class="border-border mt-24 border-t">
  <div class="shell grid gap-8 py-10 sm:grid-cols-[auto_1fr] sm:items-start">
    <img class="size-16 rounded-xl" src={asset("gimble-mascot.png")} alt="" width="64" height="64" loading="lazy" />
    <div>
      <nav class="flex flex-wrap gap-x-6 gap-y-2 text-sm" aria-label="Footer">
        {#each docs as entry}
          <a class="text-muted-foreground hover:text-foreground transition-colors" href={resolve(entry.route)}>{entry.title}</a>
        {/each}
        <a class="text-muted-foreground hover:text-foreground transition-colors" href={site.repo}>GitHub</a>
      </nav>
      <p class="text-muted-foreground mt-5 font-mono text-xs">Gimble · {site.category} · MIT · {year}</p>
    </div>
  </div>
</footer>
