<script lang="ts">
  import { navigateOnUnauthorized } from "#lib/auth/unauthorized";
  import { errorMessage } from "#lib/errors";
  import { List } from "@lucide/svelte";
  import { markRequirements, watchRecording } from "../transcript.remote";
  let { project, id }: { project: string; id: string } = $props();
  const stream = $derived(watchRecording({ project, id }));
  const record = $derived(stream.current);
  let marking = $state(false);
  let markError = $state("");
  async function mark() {
    marking = true;
    markError = "";
    try {
      await markRequirements({ project, id });
      stream.reconnect();
    } catch (error) {
      if (!navigateOnUnauthorized(error)) markError = errorMessage(error);
    } finally {
      marking = false;
    }
  }
  $effect(() => {
    if (stream.error) void stream.catch(navigateOnUnauthorized);
  });
</script>

<section aria-label="Saved transcript" class="transcript">
  {#if record}
    <h1>Recorded conversation</h1>
    <p>{new Date(record.created).toLocaleString()} · {record.status}</p>
    {#if record.requirements}<p class="requirements">
        <List size={12} aria-hidden="true" /><span
          >Requirements discussion · feeds <a href={`/projects/${project}/promises`}>Promises</a></span
        >
      </p>
    {:else}<p class="requirements">
        <button disabled={marking} onclick={mark}
          >{marking ? "Marking…" : "Treat as requirements discussion"}</button
        >{#if markError}<span class="error" role="alert">{markError}</span>{/if}Its promises
        are added to this project's Promises tab. Recordings can't be unmarked.
      </p>{/if}
    <p class="saving">
      {stream.connected
        ? "Saved on server"
        : "Server disconnected · showing last saved text"}
    </p>
    {#each [...record.finalized, ...record.provisional] as chunk, i (`${chunk.startMs}-${i}`)}<p
        class="chunk"
      >
        <time>{(chunk.startMs / 1000).toFixed(1)}s</time>{chunk.text}
      </p>{/each}
    {#if !record.finalized.length && !record.provisional.length}<p>
        Waiting for speech…
      </p>{/if}
  {:else}<p>
      {stream.error
        ? "Could not load recording. Reconnect to the server."
        : "Loading transcript…"}
    </p>{/if}
  {#if !stream.connected}<button onclick={() => stream.reconnect()}
      >Reconnect</button
    >{/if}
</section>

<style>
  .transcript {
    flex: 1;
    min-width: 0;
    padding: 1.8rem;
    overflow: auto;
  }
  h1 {
    font-size: 1.4rem;
  }
  .requirements {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.3rem 0.6rem;
    margin: 0.6rem 0;
    color: var(--muted-foreground);
    font-size: 0.8rem;
  }
  .requirements a {
    color: inherit;
  }
  .requirements button {
    padding: 0.3rem 0.7rem;
    border: 1px solid var(--border);
    border-radius: 6px;
    background: var(--background);
    color: var(--foreground);
    font: inherit;
    font-size: 0.8rem;
    cursor: pointer;
  }
  .requirements button:disabled {
    opacity: 0.5;
    cursor: default;
  }
  .error {
    color: var(--destructive);
    font-size: 0.8rem;
  }
  .saving {
    font-size: 0.8rem;
    color: var(--muted-foreground);
  }
  .chunk {
    white-space: pre-wrap;
    font-size: 1.15rem;
    line-height: 1.8;
  }
  time {
    display: block;
    font-size: 0.7rem;
    color: var(--muted-foreground);
  }
</style>
