<script lang="ts">
	import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
	import type { JSONObject } from '../sessionstate/index.js';
	import { usageOf, usageText } from '../observation/index.js';
	import Payload from './detail/Payload.svelte';
	import ToolCall from './detail/ToolCall.svelte';

	let { message, pending, revision, quiet = false }: { message: JSONObject; pending?: JSONObject; revision: number; quiet?: boolean } = $props();
	const text = (value: unknown) => typeof value === 'string' ? value : JSON.stringify(value, null, 2);
	const row: JSONObject = $derived.by(() => {
		revision;
		return {
			...message,
			...(message.content ? { content: message.content.map((part: JSONObject) => ({ ...part, ...(part.state ? { state: { ...part.state } } : {}) })) } : {})
		};
	});
	const status = $derived(pending?.delivery ?? (row.error ? 'failed' : row.time?.completed ? 'completed' : 'running'));
	// Quiet mode drops an assistant message straight to its tool rows when it
	// carries no text or reasoning of its own — no "Assistant / completed"
	// card is worth a whole article for a message that is only tool calls.
	const toolOnly = $derived(
		quiet &&
			row.type === 'assistant' &&
			(row.content ?? []).length > 0 &&
			(row.content ?? []).every((part: JSONObject) => part.type === 'tool')
	);
	const shortUsage = $derived.by(() => {
		const usage = usageOf(row);
		return `${usage.output} out · $${usage.stated_cost.toFixed(2)}`;
	});

	let collapsed = $state(false);
</script>

{#if toolOnly}
	{#each row.content as part, index (`${row.id}:${part.id ?? part.type}:${index}`)}
		<ToolCall part={part} />
	{/each}
{:else}
<article class:assistant={row.type === 'assistant'} data-message-id={row.id}>
	<button type="button" class="head" aria-expanded={!collapsed} onclick={() => (collapsed = !collapsed)}>
		<ChevronRightIcon size={14} class={collapsed ? 'chevron' : 'chevron open'} />
		<strong>{row.type === 'assistant' ? 'Assistant' : row.type}</strong>
		<span class="spacer"></span>
		{#if quiet && row.type === 'assistant'}<span class="meta">{shortUsage}</span>{/if}
		<span class="status">{status}</span>
	</button>
	{#if !collapsed}
		<div class="content">
			{#if row.type === 'assistant'}
				{#each row.content ?? [] as part, index (`${row.id}:${part.id ?? part.type}:${index}`)}
					{#if part.type === 'text'}
						<p class="prose">{part.text}</p>
					{:else if part.type === 'reasoning'}
						<details open={!part.time?.completed}>
							<summary>Reasoning · {part.time?.completed ? 'completed' : 'running'}</summary>
							<p class="prose reasoning">{part.text}</p>
						</details>
					{:else if part.type === 'tool'}
						<ToolCall part={part} />
					{/if}
				{/each}
				{#if row.retry}<p class="warning">Retry scheduled{row.retry.attempt ? ` · attempt ${row.retry.attempt}` : ''}: {text(row.retry.error)}</p>{/if}
				{#if row.error}<p class="error">{text(row.error)}</p>{/if}
				{#if !quiet}<footer>{usageText(usageOf(row))}</footer>{/if}
			{:else if row.text !== undefined}
				<p class="prose">{row.text}</p>
			{:else if row.type === 'shell'}
				<Payload label="shell" text={`$ ${row.command}\n${row.output ?? ''}`} />
			{:else if row.type === 'compaction'}
				<p class="prose">{row.summary}</p>
			{:else}
				<Payload label={String(row.type ?? 'event')} text={text(row)} />
			{/if}
		</div>
	{/if}
</article>
{/if}

<style>
	article { min-width: 0; background: var(--surface); border: 1px solid var(--map-line); border-radius: .65rem; overflow: hidden; }
	.head { display: flex; align-items: center; gap: 8px; width: 100%; min-width: 0; padding: .85rem 1rem; color: var(--status-muted); font: inherit; font-size: 13px; text-align: left; text-transform: capitalize; background: transparent; border: none; cursor: pointer; }
	.head:hover { background: color-mix(in oklch, var(--background) 50%, transparent); }
	.head :global(.chevron) { flex-shrink: 0; color: var(--status-muted); transition: transform .1s ease; }
	.head :global(.chevron.open) { transform: rotate(90deg); }
	.head strong { color: var(--foreground); font-weight: 600; }
	.spacer { flex: 1; }
	.meta { flex-shrink: 0; text-transform: none; color: var(--status-muted); font-family: var(--font-mono); font-size: 13px; }
	.status { flex-shrink: 0; font-size: 13px; }
	.content { display: flex; min-width: 0; flex-direction: column; gap: .65rem; padding: 0 1rem .85rem; }
	.prose { min-width: 0; margin: 0; overflow-wrap: anywhere; font-size: 13px; line-height: 1.5; white-space: pre-wrap; }
	.reasoning { color: var(--status-muted); }
	details { min-width: 0; }
	details summary { font-size: 13px; cursor: pointer; }
	footer { color: var(--status-muted); font-size: 13px; }
	.error { color: var(--destructive); font-size: 13px; overflow-wrap: anywhere; white-space: pre-wrap; }
	.warning { color: var(--foreground); font-size: 13px; overflow-wrap: anywhere; white-space: pre-wrap; }
</style>
