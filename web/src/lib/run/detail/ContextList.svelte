<script lang="ts">
	import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
	import type { ContextRow } from './contextOf.js';
	import { valueSize, valueText } from './contextOf.js';
	import Payload from './Payload.svelte';

	let {
		rows,
		onscope,
		maxHeight = 260
	}: {
		rows: ContextRow[];
		onscope?: (scopeKey: string) => void;
		maxHeight?: number;
	} = $props();

	// Owned by this list, not by the rows array, so toggling one row survives
	// a parent re-render even when `rows` is a freshly computed array.
	let openKeys = $state<Record<string, boolean>>({});
	const toggle = (key: string) => (openKeys[key] = !openKeys[key]);

	const formatSize = (bytes: number): string => {
		if (bytes < 1024) return `${bytes} B`;
		const kb = bytes / 1024;
		return `${kb < 10 ? kb.toFixed(1) : Math.round(kb)} KB`;
	};
</script>

<div class="context-list">
	{#if rows.length === 0}
		<p class="empty">No values are set in this scope or above it.</p>
	{/if}
	{#each rows as row (row.key)}
		{@const open = openKeys[row.key] ?? false}
		<div class="row">
			<div class="summary">
				<button
					type="button"
					class="toggle"
					aria-expanded={open}
					onclick={() => toggle(row.key)}
				>
					<ChevronRightIcon size={14} class={open ? 'chevron open' : 'chevron'} />
					<span class="key">{row.key}</span>
					<span class="size">{formatSize(valueSize(row.value))}</span>
				</button>
				{#if row.here}
					<span class="origin">set here</span>
				{:else}
					<span class="origin">from</span>
					<button type="button" class="scope-link" onclick={() => onscope?.(row.scope)}>
						{row.scope}
					</button>
				{/if}
				{#if row.shadows && row.shadows.length > 0}
					<span class="badge" title={`Hides the value set at ${row.shadows.join(', ')}`}>
						shadows {row.shadows.join(', ')}
					</span>
				{/if}
				{#if row.complete === false}
					<span class="badge">excerpt sent</span>
				{/if}
				{#if row.value.artifact}
					<span class="badge">artifact</span>
				{/if}
			</div>
			{#if open}
				<div class="body">
					<Payload label="value" text={valueText(row.value)} {maxHeight} />
				</div>
			{/if}
		</div>
	{/each}
</div>

<style>
	.context-list {
		display: flex;
		flex-direction: column;
		gap: 6px;
		min-width: 0;
	}
	.empty {
		margin: 0;
		color: var(--status-muted);
		font-size: 13px;
	}
	.row {
		display: flex;
		flex-direction: column;
		min-width: 0;
		background: var(--surface);
		border: 1px solid var(--map-line);
		border-radius: 6px;
		overflow: hidden;
	}
	.summary {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 6px 8px;
		min-width: 0;
		width: 100%;
		padding: 4px 8px;
	}
	.toggle {
		display: flex;
		align-items: center;
		gap: 8px;
		min-width: 0;
		padding: 2px 0;
		color: var(--foreground);
		font: inherit;
		text-align: left;
		background: transparent;
		border: none;
		cursor: pointer;
	}
	.toggle:hover .key {
		color: var(--status-live);
	}
	.toggle :global(.chevron) {
		flex-shrink: 0;
		color: var(--status-muted);
		transition: transform 0.1s ease;
	}
	.toggle :global(.chevron.open) {
		transform: rotate(90deg);
	}
	.key {
		flex-shrink: 0;
		color: var(--foreground);
		font-family: var(--font-mono);
		font-size: 13px;
		font-weight: 600;
	}
	.size {
		flex-shrink: 0;
		color: var(--status-muted);
		font-family: var(--font-mono);
		font-size: 13px;
	}
	.origin {
		flex-shrink: 0;
		color: var(--status-muted);
		font-size: 13px;
	}
	.scope-link {
		flex-shrink: 0;
		padding: 0;
		color: var(--status-live);
		font: inherit;
		font-size: 13px;
		font-family: var(--font-mono);
		background: transparent;
		border: none;
		text-decoration: underline;
		cursor: pointer;
	}
	.scope-link:hover {
		color: var(--foreground);
	}
	.badge {
		flex-shrink: 0;
		padding: 1px 6px;
		color: var(--status-muted);
		font-size: 13px;
		background: var(--background);
		border: 1px solid var(--map-line);
		border-radius: 999px;
	}
	.body {
		display: flex;
		flex-direction: column;
		gap: 6px;
		min-width: 0;
		padding: 0 8px 8px;
	}
</style>
