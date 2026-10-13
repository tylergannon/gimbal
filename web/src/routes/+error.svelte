<script lang="ts">
	import { goto } from "$app/navigation";
	import { page } from '$app/state';
  $effect(() => {
    if (page.status === 401) void goto("/auth/login");
  });
</script>

<h1 data-testid="title">Error {page.status}</h1>
<p data-testid="error-message">{page.error?.message}</p>
