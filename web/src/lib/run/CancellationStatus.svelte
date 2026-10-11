<script lang="ts">
  import type { RunRow } from '../observation/index.js';
  let { cancellation }: { cancellation: NonNullable<RunRow['cancellation']> } = $props();
</script>

<details class="cancellation">
  <summary>
    {#if cancellation.cleanup}
      <span>Scope cleanup {cancellation.cleanup}</span>
    {/if}
  </summary>
  <div class="history">
    {#if cancellation.by}<p>Requested by {cancellation.by}: {cancellation.reason}</p>{/if}
    {#if cancellation.cleanup_error}<p class="unconfirmed">Scope cleanup: {cancellation.cleanup_error}</p>{/if}
  </div>
</details>

<style>
  .cancellation { font-size: 12px; }
  summary { cursor: pointer; }
  summary span { display: block; }
  .unconfirmed { color: var(--destructive); }
  .history { position: absolute; z-index: 30; max-width: 420px; padding: 12px; background: var(--card); border: 1px solid var(--border); border-radius: 6px; box-shadow: 0 4px 12px #0002; }
  p { margin: 4px 0; }
</style>
