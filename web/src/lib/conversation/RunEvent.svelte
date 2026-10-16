<script lang="ts">
  import type { WorkspaceTab } from '#lib/shell/types';
  import type { Run } from '#lib/skgo/conversation/types';
  import Pip from '#lib/run/Pip.svelte';
  const labels = {queued:'Queued',running:'Running','needs-you':'Needs you',ended:'Ended',failed:'Failed',cancelled:'Cancelled'};
  let { project, run, state }: { project:string; run:Run; state?:WorkspaceTab['status'] } = $props();
  const status = $derived(state ?? (run.status === 'running' ? 'running' : run.status === 'failed' ? 'failed' : run.status === 'cancelled' ? 'cancelled' : 'ended'));
</script>
<details class="run-event" open><summary><span>›</span><span>Run</span><code>Started {run.workflow}</code><span class="state"><Pip state={status} />{labels[status]}</span></summary><div class="body"><a href={`/projects/${project}/runs/${encodeURIComponent(run.id)}`}>Open the run tab</a><code>{run.id}</code>{#if run.error}<p>{run.error}</p>{/if}</div></details>
<style>
 .run-event { border:1px solid var(--border); border-radius:7px; overflow:hidden; color:var(--muted-foreground); font:12px/18px var(--font-sans); } summary { display:flex; align-items:center; gap:8px; padding:8px 12px; list-style:none; cursor:pointer; } summary::-webkit-details-marker { display:none; } code { font:12px var(--font-mono); overflow-wrap:anywhere; } summary>code { color:var(--foreground); flex:1; } .state { display:inline-flex; align-items:center; gap:6px; } .body { display:flex; flex-wrap:wrap; gap:8px 16px; padding:10px 12px; border-top:1px solid var(--border); background:var(--muted); } a { color:var(--foreground); } p { margin:0; color:var(--destructive); }
</style>
