<script lang="ts">
  import { onMount } from "svelte";
  import { recordingHistory } from "../transcript.remote";
  let { project }: { project: string } = $props();
  let after = $state("");
  const history = $derived(await recordingHistory({ project, after }));
  onMount(() => {
    const timer = setInterval(
      () => void recordingHistory({ project, after }).refresh(),
      5000,
    );
    return () => clearInterval(timer);
  });
</script>

<aside aria-label="Recordings" class="history">
  <h2>Recordings</h2>
  <p>Use the macOS recording controls to capture a new conversation.</p>
  {#each history.items as item (item.id)}
    <a href={`/projects/${project}/recordings/${item.id}`}
      ><time>{new Date(item.created).toLocaleString()}</time><small
        >{item.status} · updated {new Date(
          item.updated,
        ).toLocaleTimeString()}</small
      ></a
    >
  {:else}<p>No recordings yet.</p>{/each}
  <div class="pager">
    {#if after}<button onclick={() => (after = "")}>Newest</button
      >{/if}{#if history.next}<button onclick={() => (after = history.next)}
        >Older recordings</button
      >{/if}
  </div>
</aside>

<style>
  .history {
    overflow: auto;
    padding: 1rem;
    width: 13rem;
    flex-shrink: 0;
    border-right: 1px solid var(--map-line);
  }
  h2 {
    font-size: 1.1rem;
  }
  p,
  small {
    font-size: 0.8rem;
    color: var(--muted-foreground);
  }
  a {
    display: block;
    padding: 0.7rem 0;
    color: var(--foreground);
    text-decoration: none;
  }
  small {
    display: block;
  }
  .pager {
    display: flex;
    gap: 0.5rem;
  }
  button {
    padding: 0.4rem;
  }
</style>
