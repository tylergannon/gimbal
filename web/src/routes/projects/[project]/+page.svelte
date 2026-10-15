<script lang="ts">
  import { page } from "$app/state";
  import { goto } from "$app/navigation";
  import type { RunCardItem } from "#lib/run/RunsList.svelte";
  import type { InterviewRow } from "#lib/observation";
  let { data }: { data: { items: RunCardItem[]; attention: InterviewRow[]; now: number } } = $props();
  import ProjectRuns from "#lib/workspace/ProjectRuns.svelte";
</script>
<ProjectRuns initial={data} project={page.params.project!} onopen={path => void goto(path)} onopenguide={name => void goto(`/projects/${page.params.project}/workflows/${encodeURIComponent(name)}`)} onstate={() => {}} />
