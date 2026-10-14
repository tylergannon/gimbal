<script lang="ts">
  import { onMount } from 'svelte';
  import { recordingControl, recordingTime, type NativeStatus } from './native';
  let { project, id }: { project: string; id: string } = $props();
  let status = $state<NativeStatus>();
  const owns = $derived(status?.project === project && status.sessionID === id && !['idle', 'stopped'].includes(status.state));
  function changed(event: Event) { status = (event as CustomEvent<NativeStatus>).detail; }
  onMount(() => {
    status = window.voiceNotesLatestStatus;
    window.addEventListener('gimbal-recording-status', changed);
    return () => window.removeEventListener('gimbal-recording-status', changed);
  });
</script>

{#if owns && status}
  <div class="controls" aria-label="This recording's controls">
    <span class:paused={status.state === 'paused'} class="dot" aria-hidden="true"></span>
    <span>{status.state === 'stopping' ? 'Saving…' : status.state} · {recordingTime(status.elapsedMs)}</span>
    <span class="source">{status.source}</span>
    {#if status.canPause}<button onclick={() => status && recordingControl(status, 'pause')}>Pause</button>{/if}
    {#if status.canResume}<button onclick={() => status && recordingControl(status, 'resume')}>Resume</button>{/if}
    {#if status.canStop}<button onclick={() => status && recordingControl(status, 'stop')}>Stop</button>{/if}
  </div>
{/if}

<style>
  .controls { position: sticky; bottom: 0; display: flex; align-items: center; flex-wrap: wrap; gap: 12px; padding: 12px 0; background: var(--background); color: var(--muted-foreground); font-size: 12px; font-variant-numeric: tabular-nums; }
  .dot { width: 8px; height: 8px; border-radius: 50%; background: var(--destructive); }
  .dot.paused { background: transparent; border: 2px solid var(--destructive); }
  button { border: 1px solid var(--border); border-radius: 6px; padding: 4px 10px; background: var(--background); color: var(--foreground); cursor: pointer; }
</style>
