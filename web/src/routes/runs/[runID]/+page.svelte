<script lang="ts">
	import RunViewer from '#lib/observation/RunViewer.svelte';
	import type { RunSnapshot } from '#lib/observation/index.js';

	// The page is the run's viewer. Everything it shows comes from the
	// snapshot the Go load handed over and the frames the store pushes after
	// it, so there is nothing else to fetch here.
	let { data }: { data: { snapshot: RunSnapshot } } = $props();
</script>

<RunViewer snapshot={data.snapshot} />
