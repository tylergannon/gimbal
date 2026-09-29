<script lang="ts">
	import type { RunSnapshot, TurnRow } from '../../observation/index.js';
	import ContextList from './ContextList.svelte';
	import { barePrompt, turnContext } from './contextOf.js';
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

</script>

<div class="prompt-and-context">
	<section>
		<div class="heading">Prompt</div>
		<Payload label="prompt" text={prompt} maxHeight={420} prose />
	</section>
	<section>
		<div class="heading">Context sent</div>
		<p class="summary">
			{context.rows.length} value{context.rows.length === 1 ? '' : 's'}
			{#if context.rows.some((row) => row.complete === false)}
				· some not included in full
			{/if}
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
