<script lang="ts">
  import { onMount, tick } from "svelte";
  import { page } from "$app/state";
  import { watchRecording } from "../transcript.remote";
  import { nativeMessage, type NativeStatus } from "./native";
  let status = $state<NativeStatus>({
    sessionID: "",
    sequence: 0,
    pending: false,
    state: "idle",
    source: "",
    project: "",
  });
  let native = $state(false);
  let stream = $state<ReturnType<typeof watchRecording>>();
  const current = $derived(stream?.current);
  $effect(() => {
    if (native)
      nativeMessage("selectProject", { project: page.params.project ?? "" });
  });
  $effect(() => {
    if (!native || !status.sessionID || !status.project) return;
    stream = watchRecording({ project: status.project, id: status.sessionID });
  });
  $effect(() => {
    if (!native) return;
    const available =
      !stream ||
      (!!current && stream.connected && current.sequence >= status.sequence);
    nativeMessage("displayAvailability", {
      available,
      error: available
        ? undefined
        : "Transcript disconnected or waiting for saved text",
    });
    if (current) {
      const id = current.id,
        sequence = current.sequence,
        closed = ["stopped", "interrupted"].includes(current.status);
      void tick().then(() =>
        nativeMessage("snapshotApplied", { sessionID: id, sequence, closed }),
      );
    }
  });
  onMount(() => {
    native = !!window.webkit?.messageHandlers?.bridgeReady;
    if (!native) return;
    window.voiceNotesNativeStatus = (next) => {
      status = next;
    };
    if (window.voiceNotesLatestStatus) status = window.voiceNotesLatestStatus;
    nativeMessage("bridgeReady", {});
    return () => {
      delete window.voiceNotesNativeStatus;
    };
  });
</script>

{#if native && status.sessionID}
  <aside aria-label="Active recording" class="recording-dock">
    <a href={`/projects/${status.project}/recordings/${status.sessionID}`}
      >Recording · {status.state}</a
    >
    <span
      >{status.pending
        ? "Pending · waiting for server"
        : stream?.connected && current && current.sequence >= status.sequence
          ? "Saved on server"
          : "Server disconnected · showing last saved text"}</span
    >
    {#if stream && !stream.connected}<button
        onclick={() => nativeMessage("reconnect", {})}>Reconnect</button
      >{/if}
    {#if status.error}<p role="alert">{status.error}</p>{/if}
    {#if current}<p>
        {current.finalized.map((c) => c.text).join("")}{current.provisional
          .map((c) => c.text)
          .join("")}
      </p>{/if}
  </aside>
{/if}

<style>
  .recording-dock {
    padding: 0.7rem 1.5rem;
    border-top: 1px solid var(--map-line);
    background: var(--card);
  }
  a {
    color: var(--status-live);
  }
  span {
    float: right;
    font-size: 0.85rem;
  }
  p {
    white-space: pre-wrap;
    margin: 0.3rem 0;
    max-height: 7rem;
    overflow: auto;
  }
</style>
