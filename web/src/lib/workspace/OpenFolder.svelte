<script lang="ts">
  import { type Snippet } from "svelte";
  import { goto, refreshAll } from "$app/navigation";
  import { openProject } from "../window.remote";
  import { chooseProjectFolder } from "./native";

  let { class: className, children }: { class?: string; children?: Snippet } = $props();
  let pending = $state(false);
  let error = $state("");

  async function open() {
    error = "";
    const path = await chooseProjectFolder();
    if (!path) return;
    pending = true;
    try {
      const project = await openProject({ path });
      if (window.webkit?.messageHandlers?.requestProject) {
        await refreshAll();
        window.webkit.messageHandlers.requestProject.postMessage({ path: `/projects/${project.id}`, projectPath: path });
      } else await goto(`/projects/${project.id}`, { invalidateAll: true });
    } catch (cause) {
      error = (cause as { body?: { message?: string } }).body?.message ?? `Could not open ${path}`;
    } finally {
      pending = false;
    }
  }
</script>

<button type="button" class={className} disabled={pending} onclick={open}>
  {#if children}{@render children()}{:else}Open folder…{/if}
</button>
{#if error}<p class="error" role="alert">{error}</p>{/if}

<style>
  .error {
    margin: 6px 0 0;
    color: var(--destructive);
    font-size: 12px;
  }
</style>
