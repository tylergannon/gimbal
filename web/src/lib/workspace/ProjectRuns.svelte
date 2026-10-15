<script lang="ts">
  import { navigateOnUnauthorized } from "#lib/auth/unauthorized";
  import { watchRuns } from "../runs.remote";
  import WorkflowLauncher from "./WorkflowLauncher.svelte";
  import RunsList, { type RunCardItem } from "#lib/run/RunsList.svelte";
  import { untrack } from "svelte";
  import type { InterviewRow, RunRow } from "#lib/observation";
  import type { WorkspaceTab } from "#lib/shell/types";
  let { project, initial, onopen, onopenguide, onstate }: { project: string; initial?: { items: RunCardItem[]; attention: InterviewRow[]; now: number }; onopen: (path: string) => void; onopenguide: (name: string) => void; onstate: (tabs: WorkspaceTab[]) => void } = $props();
  const stream = $derived(watchRuns(project));
  // Retain the route preload while the live query establishes its first update.
  const preload = untrack(() => initial);
  // The list is the same registry read used by the run routes.
  const data = $derived((stream.current ?? preload) as { items: RunCardItem[]; attention: InterviewRow[]; now: number } | undefined);
  $effect(() => {
    if (data) { const tabs = data.items.map(({ run }) => ({ id: `/projects/${project}/runs/${encodeURIComponent(run.id)}`, title: run.name, kind: "run", status: data.attention.some(question => question.run === run.id && question.status === "pending") ? "attention" : run.status === "running" ? "working" : undefined } as WorkspaceTab)); untrack(() => onstate(tabs)); }
    if (stream.error) void stream.catch(navigateOnUnauthorized);
  });
  function open(run: RunRow) { onopen(`/projects/${project}/runs/${encodeURIComponent(run.id)}`); }
</script>
<section class="project-runs" aria-label="Project runs">
  <div class="launch"><WorkflowLauncher {project} {onopen} {onopenguide} /></div>
  {#if stream.error}
    <p role="alert">Could not read runs. <button onclick={() => stream.reconnect()}>Retry</button></p>
  {:else if !data}<p role="status">Loading runs…</p>
  {:else if !data.items.length}<div class="empty"><h1>Empty project</h1><h2>No runs yet</h2><p>Choose a workflow to see its purpose and start a run in this project.</p><p>Use Run a workflow above to get started.</p></div>
  {:else}<RunsList items={data.items} attention={data.attention} now={data.now} onopenrun={open} />{/if}
</section>
<style>
 .project-runs { height: 100%; overflow: auto; padding: 20px 24px; }
 .launch { display: inline-block; margin-bottom: 16px; font-size: 13px; color: var(--foreground); }
 .empty { max-width: 560px; margin: 32px auto; }
 .empty h1 { font-size: 20px; }
 .empty p { color: var(--muted-foreground); }
</style>
