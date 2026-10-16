<script lang="ts">
  import { page } from '$app/state';
  import { goto } from '$app/navigation';
  import { watchRuns } from '#lib/runs.remote';
  import type { ComponentProps } from 'svelte';
  import ProjectRuns from '#lib/workspace/ProjectRuns.svelte';
  let { data } = $props();
  const query = $derived(watchRuns(page.params.project!));
  const runs = $derived((query.current ?? data) as ComponentProps<typeof ProjectRuns>['data']);
</script>
<ProjectRuns data={runs} error={query.error} onretry={() => query.reconnect()} project={page.params.project!} onopen={path => void goto(path)} onopenguide={name => void goto(`/projects/${page.params.project}/workflows/${encodeURIComponent(name)}`)} />
