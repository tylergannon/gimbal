<script lang="ts">
  import { navigateOnUnauthorized } from "#lib/auth/unauthorized";
  import { onMount, tick } from "svelte";
  import { beforeNavigate, goto, refreshAll } from "$app/navigation";
  import { page } from "$app/state";
  import { openProject } from "../window.remote";
  import { chooseProjectFolder } from "./native";
  import { watchRecording } from "../transcript.remote";
  import { nativeMessage, type NativeStatus } from "./native";
  let { dock = true }: { dock?: boolean } = $props();
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
  const available = $derived(!stream || (!!current && current.id === status.sessionID && (stream.connected || (stream.done && ['stopped', 'interrupted'].includes(current.status))) && current.sequence >= status.sequence));
  let approvedPath: string | undefined;
  let arrival = $state<string>();
  beforeNavigate((navigation) => {
    if (!native || !navigation.to) return;
    const path = navigation.to.url.pathname;
    if (path === approvedPath) { approvedPath = undefined; return; }
    const target = navigation.to.params?.project;
    const owner = !['idle', 'stopped'].includes(status.state) ? status.project : page.params.project;
    if (!target || target === owner) return;
    navigation.cancel();
    nativeMessage('requestProject', { path });
  });
  async function navigate(event: Event) {
    const { path } = (event as CustomEvent<{ path: string }>).detail;
    approvedPath = path;
    try {
      await goto(path, { reset: false });
      await tick();
      arrival = path.split('/')[2];
    } catch (error) {
      approvedPath = undefined;
      nativeMessage('projectNavigationComplete', { project: path.split('/')[2], error: 'Could not open the destination workspace.' });
    }
  }
  async function command(event: Event) {
    if ((event as CustomEvent<{ command: string }>).detail.command !== 'openProjectFolder') return;
    const path = await chooseProjectFolder();
    if (!path) return;
    try {
      const project = await openProject({ path });
      await refreshAll();
      nativeMessage('requestProject', { path: `/projects/${project.id}/recordings`, projectPath: path });
    } catch (error) {
      nativeMessage('projectAdmissionError', { error: `Could not open ${path}` });
    }
  }
  $effect(() => {
    if (!arrival || page.params.project !== arrival) return;
    nativeMessage('projectNavigationComplete', { project: arrival });
    arrival = undefined;
  });
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
  $effect(() => {
    if (stream?.error) void stream.catch(navigateOnUnauthorized);
  });
  onMount(() => {
    native = !!window.webkit?.messageHandlers?.bridgeReady;
    if (!native) return;
    window.voiceNotesNativeStatus = (next) => {
      status = next;
      window.dispatchEvent(new CustomEvent('gimbal-recording-status', { detail: next }));
    };
    if (window.voiceNotesLatestStatus) status = window.voiceNotesLatestStatus;
    window.addEventListener('gimbal-project-navigation', navigate);
    window.addEventListener('gimbal-workspace-command', command);
    nativeMessage("bridgeReady", {});
    return () => {
      window.removeEventListener('gimbal-project-navigation', navigate);
      window.removeEventListener('gimbal-workspace-command', command);
      delete window.voiceNotesNativeStatus;
    };
  });
</script>

{#if native && status.error}<p class="recording-error" role="alert">{status.error}</p>{/if}

{#if dock && native && status.sessionID}
  <aside aria-label="Active recording" class="recording-dock">
    <a href={`/projects/${status.project}/recordings/${status.sessionID}`}
      >Recording · {status.state}</a
    >
    <span
      >{status.pending
        ? "Pending · waiting for server"
        : (stream?.connected || (stream?.done && current && ['stopped', 'interrupted'].includes(current.status))) && current && current.id === status.sessionID && current.sequence >= status.sequence
          ? "Saved on server"
          : "Server disconnected · showing last saved text"}</span
    >
    {#if stream && !stream.connected && !stream.done}<button
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
  .recording-error { margin: 0; padding: 8px 16px; color: var(--destructive); background: var(--background); font-size: 12px; }
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
