<script lang="ts">
  let { state }: { state: string } = $props();
  const working = $derived(state === 'connecting' || state === 'starting' || state === 'pending');
</script>

{#if state !== 'connected' && state !== 'completed'}
  <span class="state-glyph" class:working class:dashed={state === 'not-started'} class:problem={state === 'error' || state === 'failed' || state === 'warning'} aria-hidden="true"></span>
{/if}

<style>
  .state-glyph { display: inline-block; width: 10px; height: 10px; flex: 0 0 10px; border: 1.5px solid var(--muted-foreground); border-radius: 50%; vertical-align: -1px; }
  .dashed { border-style: dashed; }
  .problem { border-color: var(--destructive); }
  .working { border-color: var(--chart-2); border-right-color: transparent; animation: turn 1s linear infinite; }
  @keyframes turn { to { transform: rotate(360deg); } }
  @media (prefers-reduced-motion: reduce) { .working { animation: none; border-style: dotted; } }
</style>
