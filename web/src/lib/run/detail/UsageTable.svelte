<script lang="ts">
	import type { Usage } from '../../observation/index.svelte.js';

	let { usage, title }: { usage: Usage; title?: string } = $props();

	const rows = $derived([
		['input', usage.input.toLocaleString()],
		['output', usage.output.toLocaleString()],
		['reasoning', usage.reasoning.toLocaleString()],
		['cache read', usage.cache_read.toLocaleString()],
		['cache write', usage.cache_write.toLocaleString()],
		['cost', `$${usage.stated_cost.toFixed(2)}`]
	] as const);
</script>

<div class="usage-table">
	{#if title}<div class="title">{title}</div>{/if}
	<dl>
		{#each rows as [label, value] (label)}
			<dt>{label}</dt>
			<dd>{value}</dd>
		{/each}
	</dl>
</div>

<style>
	.usage-table {
		min-width: 0;
	}
	.title {
		margin-bottom: 6px;
		color: var(--foreground);
		font-size: 13px;
		font-weight: 600;
	}
	dl {
		display: grid;
		grid-template-columns: 1fr auto;
		gap: 4px 12px;
		margin: 0;
	}
	dt {
		color: var(--status-muted);
		font-size: 13px;
		line-height: 20px;
	}
	dd {
		margin: 0;
		color: var(--foreground);
		font-family: var(--font-mono);
		font-size: 13px;
		line-height: 20px;
		text-align: right;
	}
</style>
