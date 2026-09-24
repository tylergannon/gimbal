<script lang="ts">
	let { value }: { value: unknown } = $props();

	const record = (item: unknown): item is Record<string, unknown> =>
		typeof item === 'object' && item !== null && !Array.isArray(item);
	const label = (key: string) =>
		key.replace(/[_-]+/g, ' ').replace(/([a-z])([A-Z])/g, '$1 $2').replace(/^./, (c) => c.toUpperCase());
	const title = (item: unknown, index: number) => {
		if (!record(item)) return `Item ${index + 1}`;
		for (const key of ['name', 'title', 'label', 'id']) {
			if (typeof item[key] === 'string' && item[key]) return String(item[key]);
		}
		return `Item ${index + 1}`;
	};
	const fields = (item: Record<string, unknown>, card: boolean) =>
		Object.entries(item).filter(([key]) => !card || !['name', 'title', 'label', 'id'].includes(key));
</script>

{#snippet node(item: unknown, card = false)}
	{#if Array.isArray(item)}
		{#if item.length === 0}
			<span class="empty">None</span>
		{:else}
			<div class="items">
				{#each item as entry, index (index)}
					<div class="item">
						<div class="item-title">{title(entry, index)}</div>
						{@render node(entry, true)}
					</div>
				{/each}
			</div>
		{/if}
	{:else if record(item)}
		{#if Object.keys(item).length === 0}
			<span class="empty">No details</span>
		{:else}
			<dl class="fields">
				{#each fields(item, card) as [key, child] (key)}
					<div class="field">
						<dt>{label(key)}</dt>
						<dd>{@render node(child)}</dd>
					</div>
				{/each}
			</dl>
		{/if}
	{:else if item === null}
		<span class="empty">None</span>
	{:else if typeof item === 'boolean'}
		<span class="boolean">{item ? 'Yes' : 'No'}</span>
	{:else}
		<span class="value">{String(item)}</span>
	{/if}
{/snippet}

<div class="structured">{@render node(value)}</div>

<style>
	.structured { min-width: 0; padding: 12px; color: var(--foreground); font-size: 14px; line-height: 1.5; }
	.fields { display: grid; gap: 12px; min-width: 0; margin: 0; }
	.field { min-width: 0; }
	dt { margin-bottom: 3px; color: var(--status-muted); font-size: 12px; font-weight: 600; letter-spacing: .02em; }
	dd { min-width: 0; margin: 0; }
	.value { white-space: pre-wrap; overflow-wrap: anywhere; }
	.empty { color: var(--status-muted); font-style: italic; }
	.boolean { display: inline-block; padding: 1px 7px; border: 1px solid var(--map-line); border-radius: 999px; }
	.items { display: grid; gap: 10px; min-width: 0; }
	.item { min-width: 0; padding: 12px; background: var(--surface); border: 1px solid var(--map-line); border-radius: 8px; }
	.item-title { margin-bottom: 8px; font-weight: 650; line-height: 1.35; overflow-wrap: anywhere; }
	.item :global(.fields) { gap: 10px; }
</style>
