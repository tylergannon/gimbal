<script lang="ts">
  import { navigateOnUnauthorized } from "#lib/auth/unauthorized";
  import { watchRecording } from "../transcript.remote";
  let { project, id }: { project: string; id: string } = $props();
  const stream = $derived(watchRecording({ project, id }));
  const record = $derived(stream.current);
  $effect(() => {
    if (stream.error) void stream.catch(navigateOnUnauthorized);
  });
</script>

<section aria-label="Saved transcript" class="transcript">
  {#if record}
    <h1>Recorded conversation</h1>
    <p>{new Date(record.created).toLocaleString()} · {record.status}</p>
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
