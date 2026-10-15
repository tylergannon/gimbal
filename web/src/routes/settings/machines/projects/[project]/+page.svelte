<script lang="ts">
  import { page } from '$app/state';
  import Machines from '#lib/environments/Machines.svelte';
</script>
<Machines project={page.params.project} projects={page.data.workspaceProjects} />
