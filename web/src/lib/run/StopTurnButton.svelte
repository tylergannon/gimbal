<script lang="ts">
  import { stopTurn } from '../../routes/control.remote';
  import { navigateOnUnauthorized } from '#lib/auth/unauthorized';
  import { errorMessage } from '#lib/errors';
  let { run, turn, onfeedback }: { run: string; turn: string; onfeedback?: (message: string) => void } = $props();
  let stopping = $state(false);
  let feedback = $state('');
</script>
<button type="button" disabled={stopping} onclick={async () => { stopping = true; try { const result = await stopTurn({ run, turn }); feedback = result.accepted ? 'Stop accepted. Waiting for the turn to finish.' : 'This turn did not accept the stop request.'; } catch (error) { if (!navigateOnUnauthorized(error)) feedback = errorMessage(error); } finally { stopping = false; onfeedback?.(feedback); } }}>Stop turn</button>
{#if feedback && !onfeedback}<span role="status">{feedback}</span>{/if}
<style>
  button { flex:none; padding:5px 10px; border:1px solid var(--border); border-radius:5px; background:var(--background); color:var(--foreground); font:12px/18px var(--font-sans); cursor:pointer; } button:disabled { opacity:.5; } span { font-size:12px; color:var(--muted-foreground); }
</style>
