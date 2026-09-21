<script lang="ts">
	// The tab strip shared by a session's detail (Activity · Result · Prompt ·
	// Usage · Source) and a scope's detail (Overview · Context · Decisions):
	// one look, one keyboard behavior, defined once.
	import { tick } from 'svelte';

	let {
		tabs,
		active,
		onselect
	}: {
		tabs: { id: string; label: string }[];
		active: string;
		onselect: (id: string) => void;
	} = $props();

	async function focusTab(id: string) {
		await tick();
		document.getElementById(`tab-${id}`)?.focus();
	}

	function onkeydown(event: KeyboardEvent) {
		const index = tabs.findIndex((tab) => tab.id === active);
		let next: string | undefined;
		if (event.key === 'ArrowRight') {
			next = tabs[(index + 1) % tabs.length].id;
		} else if (event.key === 'ArrowLeft') {
			next = tabs[(index - 1 + tabs.length) % tabs.length].id;
		} else if (event.key === 'Home') {
			next = tabs[0].id;
		} else if (event.key === 'End') {
			next = tabs[tabs.length - 1].id;
		} else {
			return;
		}
		event.preventDefault();
		onselect(next);
		focusTab(next);
	}
</script>

<div class="tabs" role="tablist" aria-label="Detail" tabindex="-1" {onkeydown}>
	{#each tabs as tab (tab.id)}
		<button
			type="button"
			role="tab"
			id={`tab-${tab.id}`}
			aria-controls={`panel-${tab.id}`}
			aria-selected={active === tab.id}
			tabindex={active === tab.id ? 0 : -1}
			class:active={active === tab.id}
			onclick={() => onselect(tab.id)}
		>
			{tab.label}
		</button>
	{/each}
</div>

<style>
	.tabs {
		display: flex;
		flex-shrink: 0;
		gap: 2px;
		padding: 0 8px;
		background: var(--card);
		border-bottom: 1px solid var(--border);
	}
	.tabs button {
		padding: 9px 10px;
		color: var(--status-muted);
		font: inherit;
		font-size: 13px;
		font-weight: 500;
		background: transparent;
		border: 0;
		border-bottom: 2px solid transparent;
		cursor: pointer;
	}
	.tabs button:hover {
		color: var(--foreground);
	}
	.tabs button.active {
		color: var(--foreground);
		border-bottom-color: var(--status-live);
	}
	.tabs button:focus-visible {
		outline: 2px solid var(--status-live);
		outline-offset: -2px;
	}
</style>
