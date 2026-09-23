<script lang="ts">
  import { Check, Copy } from "@lucide/svelte";

  const command = "git clone https://github.com/tylergannon/gimble && cd gimble && just build";
  let copied = $state(false);

  async function copy() {
    await navigator.clipboard.writeText(command);
    copied = true;
    window.setTimeout(() => (copied = false), 1800);
  }
</script>

<div class="border-border bg-card flex items-center gap-3 rounded-lg border py-2 pr-2 pl-4" data-testid="install-prompt">
  <span class="text-primary font-mono text-sm select-none" aria-hidden="true">$</span>
  <code class="text-foreground min-w-0 flex-1 overflow-x-auto text-[0.8rem] whitespace-nowrap">{command}</code>
  <button
    class="text-muted-foreground hover:text-foreground hover:bg-muted grid size-8 shrink-0 place-items-center rounded-md transition-colors"
    onclick={copy}
    aria-label={copied ? "Install command copied" : "Copy install command"}
  >
    {#if copied}
      <Check class="size-4" aria-hidden="true" />
    {:else}
      <Copy class="size-4" aria-hidden="true" />
    {/if}
  </button>
  <p class="sr-only" aria-live="polite">{copied ? "Install command copied to clipboard." : ""}</p>
</div>
