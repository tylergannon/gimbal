<script lang="ts">
  import { page } from '$app/state';
  import Machines from '#lib/environments/Machines.svelte';
</script>
<Machines projects={page.data.workspaceProjects} />
