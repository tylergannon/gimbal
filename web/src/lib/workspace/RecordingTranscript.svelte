<script lang="ts">
  import { navigateOnUnauthorized } from "#lib/auth/unauthorized";
  import { errorMessage } from "#lib/errors";
  import RecordingControls from "./RecordingControls.svelte";
  import { List } from "@lucide/svelte";
  import { untrack } from 'svelte';
  import { markRequirements, recordingSnapshot } from "../transcript.remote";
  import { recordingTime, type NativeStatus } from './native';
  let { project, id, active = true, nativeStatus }: { project: string; id: string; active?: boolean; nativeStatus?: NativeStatus } = $props();
  const query = $derived(recordingSnapshot({ project, id }));
  const record = $derived(query.current);
  const local = $derived(nativeStatus?.project === project && nativeStatus.sessionID === id ? nativeStatus : undefined);
  const savedThrough = $derived(record?.finalized.at(-1)?.endMs);
  $effect(() => {
    if (!active) return;
    const current = query;
    if (untrack(() => current.ready)) void current.refresh().catch(navigateOnUnauthorized);
  });
  $effect(() => {
    if (!active) return;
    const current = query;
    const timer = setInterval(() => { void current.refresh().catch(navigateOnUnauthorized); }, 1000);
    return () => clearInterval(timer);
  });
  let marking = $state(false);
  let markError = $state("");
  async function mark() {
    marking = true;
    markError = "";
    try { await markRequirements({ project, id }); await query.refresh(); }
    catch (error) { if (!navigateOnUnauthorized(error)) markError = errorMessage(error); }
    finally { marking = false; }
  }
  $effect(() => {
    if (query.error) void query.catch(navigateOnUnauthorized);
  });
</script>

<section aria-label="Saved transcript" class="transcript">
  {#if record}
    <h1>Recorded conversation</h1>
    <p>{new Date(record.created).toLocaleString()} · {record.status}</p>
    {#if record.requirements}
      <p class="requirements"><List size={12} aria-hidden="true" /><span>Requirements discussion · feeds <a href={`/projects/${project}/promises`}>Promises</a></span></p>
    {:else}
      <p class="requirements"><button disabled={marking} onclick={mark}>{marking ? "Marking…" : "Treat as requirements discussion"}</button><span>Its promises are added to this project's Promises tab. Recordings can't be unmarked.</span></p>
      {#if markError}<p class="error" role="alert">{markError}</p>{/if}
    {/if}
    <p class="saving">
      {query.error ? 'Server disconnected · showing last saved text' : 'Saved on server'}{#if savedThrough != null} · words through {recordingTime(savedThrough)}{/if}{#if query.error && local?.state === 'recording'} · capture continues locally{:else if query.error && local?.state === 'paused'} · capture is paused locally{/if}
    </p>
    {#each record.finalized as chunk, i (`final-${chunk.startMs}-${i}`)}<p class="chunk"><time>{recordingTime(chunk.startMs)}</time><span>{chunk.text}</span></p>{/each}
    {#each record.provisional as chunk, i (`provisional-${chunk.startMs}-${i}`)}<p class="chunk provisional"><time>{recordingTime(chunk.startMs)}</time><span>{chunk.text}</span></p>{/each}
    {#if !record.finalized.length && !record.provisional.length}<p>
        Waiting for speech…
      </p>{/if}
  {:else}<p>
      {query.error
        ? "Could not load recording. Reconnect to the server."
        : "Loading transcript…"}
    </p>{/if}
  {#if query.error}<button onclick={() => query.refresh()}
      >Reconnect</button
    >{/if}
  <RecordingControls {project} {id} />
</section>

<style>
  .transcript {
    height: 100%;
    box-sizing: border-box;
    min-width: 0;
    padding: 28px max(24px, calc((100% - 760px) / 2));
    overflow: auto;
  }
  h1 {
    font-size: 1.4rem;
  }
  .saving {
    font-size: 0.8rem;
    color: var(--muted-foreground);
  }
  .requirements { display: flex; flex-wrap: wrap; align-items: center; gap: 0.3rem 0.6rem; margin: 0.6rem 0; color: var(--muted-foreground); font-size: 0.8rem; }
  .requirements a { color: inherit; }
  .requirements button { padding: 0.3rem 0.7rem; border: 1px solid var(--border); border-radius: 6px; background: var(--background); color: var(--foreground); font: inherit; cursor: pointer; }
  .requirements button:disabled { opacity: 0.5; cursor: default; }
  .error { color: var(--destructive); font-size: 0.8rem; }
  .chunk {
    display: grid;
    grid-template-columns: 46px minmax(0, 1fr);
    gap: 12px;
    white-space: pre-wrap;
    font-size: 15px;
    line-height: 1.625;
    margin: 18px 0;
  }
  .provisional { color: var(--muted-foreground); }
  time {
    padding-top: 3px;
    font: 11px/1.625 var(--font-mono);
    color: var(--muted-foreground);
  }
</style>
