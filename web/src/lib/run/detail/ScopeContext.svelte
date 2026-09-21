<script lang="ts">
	import type { RunSnapshot } from '../../observation/index.svelte.js';
	import ContextList from './ContextList.svelte';
	import { visibleContext } from './contextOf.js';

	let {
		snapshot,
		scopeKey,
		onscope
	}: {
		snapshot: RunSnapshot;
		scopeKey: string;
		onscope?: (scopeKey: string) => void;
	} = $props();

	const rows = $derived(visibleContext(snapshot, scopeKey));
	const here = $derived(rows.filter((row) => row.here));
	const outer = $derived(rows.filter((row) => !row.here));
</script>

<div class="scope-context">
	<section>
		<div class="heading">Set here</div>
		<ContextList rows={here} {onscope} />
	</section>
	{#if outer.length > 0}
		<section>
			<div class="heading">From outer scopes</div>
			<ContextList rows={outer} {onscope} />
		</section>
	{/if}
</div>

<style>
	.scope-context {
		display: flex;
		flex-direction: column;
		gap: 16px;
		min-width: 0;
	}
	.heading {
		margin-bottom: 6px;
		color: var(--foreground);
		font-size: 13px;
		font-weight: 600;
	}
</style>
