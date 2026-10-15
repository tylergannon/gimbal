<script lang="ts">
  import { page } from '$app/state';
  import { goto } from '$app/navigation';
  import { watchRuns } from '../../../lib/runs.remote';
  import ProjectRuns from '#lib/workspace/ProjectRuns.svelte';
  import type { RunCardItem } from '#lib/run/RunsList.svelte';
  import type { InterviewRow } from '#lib/observation';
  let { data } = $props();
  const query = $derived(watchRuns(page.params.project!));
  const runs = $derived((query.current ?? data) as { items: RunCardItem[]; attention: InterviewRow[]; now: number });
</script>
<ProjectRuns data={runs} error={query.error} onretry={() => query.reconnect()} project={page.params.project!} onopen={path => void goto(path)} />
