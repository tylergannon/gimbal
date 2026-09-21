<script lang="ts">
	import type { RunSnapshot, TurnRow } from '../../observation/index.svelte.js';
	import ContextList from './ContextList.svelte';
	import { barePrompt, turnContext, valueSize } from './contextOf.js';
	import Payload from './Payload.svelte';

	let {
		turn,
		snapshot,
		onscope
	}: {
		turn: TurnRow;
		snapshot: RunSnapshot;
		onscope?: (scopeKey: string) => void;
	} = $props();

	const context = $derived(turnContext(snapshot, turn));
	const prompt = $derived(barePrompt(turn, snapshot));

	const tokens = $derived(
		Math.round(context.rows.reduce((total, row) => total + valueSize(row.value), 0) / 4),
	);
	const formatTokens = (count: number): string => `about ${count.toLocaleString()} tokens`;
</script>

<div class="prompt-and-context">
	<section>
		<div class="heading">Prompt</div>
		<Payload label="prompt" text={prompt} maxHeight={420} prose />
	</section>
	<section>
		<div class="heading">Context sent</div>
		<p class="summary">
			{context.rows.length} value{context.rows.length === 1 ? '' : 's'} · {formatTokens(tokens)}
		</p>
		{#if !context.recorded}
			<p class="note">
				Inferred from the scopes: this run was saved before turns recorded their context.
			</p>
		{/if}
		<ContextList rows={context.rows} {onscope} />
	</section>
</div>

<style>
	.prompt-and-context {
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
	.summary {
		margin: 0 0 6px;
		color: var(--status-muted);
		font-size: 13px;
	}
	.note {
		margin: 0 0 8px;
		color: var(--status-muted);
		font-size: 13px;
		font-style: italic;
	}
</style>
