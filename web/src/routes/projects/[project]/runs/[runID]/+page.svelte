<script lang="ts">
  import { page } from "$app/state";
  import RunView from "#lib/run/RunView.svelte";
  import type { RunSnapshot } from "#lib/observation";
  let { data }: { data: { snapshot: RunSnapshot; graph: string; source?: string } } = $props();
</script>
<RunView project={page.params.project!} {data} />
