<script lang="ts">
	import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
	import type { JSONObject } from '../sessionstate/index.js';
	import { usageOf, usageText } from '../observation/index.js';
	import Payload from './detail/Payload.svelte';
	import ToolCall from './detail/ToolCall.svelte';

	let { message, pending, revision }: { message: JSONObject; pending?: JSONObject; revision: number } = $props();
	const text = (value: unknown) => typeof value === 'string' ? value : JSON.stringify(value, null, 2);
	const row: JSONObject = $derived.by(() => {
		revision;
		return {
			...message,
			...(message.content ? { content: message.content.map((part: JSONObject) => ({ ...part, ...(part.state ? { state: { ...part.state } } : {}) })) } : {})
		};
	});
	const status = $derived(pending?.delivery ?? (row.error ? 'failed' : row.time?.completed ? 'completed' : 'running'));

	let collapsed = $state(false);
</script>

<article class:assistant={row.type === 'assistant'} data-message-id={row.id}>
	<button type="button" class="head" aria-expanded={!collapsed} onclick={() => (collapsed = !collapsed)}>
		<ChevronRightIcon size={14} class={collapsed ? 'chevron' : 'chevron open'} />
		<strong>{row.type === 'assistant' ? 'Assistant' : row.type}</strong>
		<span class="spacer"></span>
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
				<footer>{usageText(usageOf(row))}</footer>
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

<style>
	article { min-width: 0; background: var(--surface); border: 1px solid var(--map-line); border-radius: .65rem; overflow: hidden; }
	.head { display: flex; align-items: center; gap: 8px; width: 100%; min-width: 0; padding: .85rem 1rem; color: var(--status-muted); font: inherit; font-size: 13px; text-align: left; text-transform: capitalize; background: transparent; border: none; cursor: pointer; }
	.head:hover { background: color-mix(in oklch, var(--background) 50%, transparent); }
	.head :global(.chevron) { flex-shrink: 0; color: var(--status-muted); transition: transform .1s ease; }
	.head :global(.chevron.open) { transform: rotate(90deg); }
	.head strong { color: var(--foreground); font-weight: 600; }
	.spacer { flex: 1; }
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
