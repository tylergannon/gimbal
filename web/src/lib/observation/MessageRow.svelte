<script lang="ts">
	import type { JSONObject } from '../sessionstate/index.js';
	import { usageOf, usageText } from './index.js';

	let { message, pending, revision }: { message: JSONObject; pending?: JSONObject; revision: number } = $props();
	const text = (value: unknown) => typeof value === 'string' ? value : JSON.stringify(value, null, 2);
	const eventText = (event: JSONObject) => {
		const native = event.event ?? event;
		const delta = native.delta ?? native.event?.delta;
		return delta?.text ?? delta?.thinking ?? native.message?.content?.map((part: JSONObject) => part.text).filter(Boolean).join('\n') ?? text(native);
	};
	const row: JSONObject = $derived.by(() => {
		revision;
		return {
			...message,
			...(message.content ? { content: message.content.map((part: JSONObject) => ({ ...part, ...(part.state ? { state: { ...part.state } } : {}) })) } : {})
		};
	});
	const status = $derived(pending?.delivery ?? (row.error ? 'failed' : row.time?.completed ? 'completed' : 'running'));
</script>

<article class:assistant={row.type === 'assistant'} data-message-id={row.id}>
	<header>
		<strong>{row.type === 'assistant' ? 'Assistant' : row.type}</strong>
		<span>{status}</span>
	</header>
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
				<section class="tool">
					<header><strong>{part.name}</strong><span>{part.state?.status ?? 'pending'}</span></header>
					{#if part.state?.input !== undefined}<pre>{text(part.state.input)}</pre>{/if}
					{#if part.state?.status === 'running' && part.state?.metadata !== undefined}<pre class="progress">{text(part.state.metadata)}</pre>{/if}
					{#if part.state?.content !== undefined}
						{#each part.state.content as content}
							{#if content.type === 'transcript'}
								<details class="nested" open>
									<summary>Subagent transcript · {content.events?.length ?? 0} events</summary>
									{#each content.events ?? [] as event}
										<pre>{eventText(event)}</pre>
									{/each}
								</details>
							{:else}<pre>{text(content)}</pre>{/if}
						{/each}
					{/if}
					{#if part.state?.error !== undefined}<p class="error">{text(part.state.error)}</p>{/if}
				</section>
			{/if}
		{/each}
		{#if row.retry}<p class="warning">Retry scheduled{row.retry.attempt ? ` · attempt ${row.retry.attempt}` : ''}: {text(row.retry.error)}</p>{/if}
		{#if row.error}<p class="error">{text(row.error)}</p>{/if}
		<footer>{usageText(usageOf(row))}</footer>
	{:else if row.text !== undefined}
		<p class="prose">{row.text}</p>
	{:else if row.type === 'shell'}
		<pre>$ {row.command}\n{row.output ?? ''}</pre>
	{:else if row.type === 'compaction'}
		<p class="prose">{row.summary}</p>
	{:else}
		<pre>{text(row)}</pre>
	{/if}
</article>

<style>
	article { border: 1px solid #dfe3ea; border-radius: .65rem; padding: .85rem 1rem; background: #fff; }
	article.assistant { border-left: 3px solid #4468d7; }
	header { display: flex; justify-content: space-between; gap: 1rem; color: #556070; font-size: .82rem; text-transform: capitalize; }
	.prose { white-space: pre-wrap; margin: .65rem 0; }
	.reasoning { color: #515a68; }
	details { margin-top: .65rem; }
	.tool { margin-top: .65rem; border-radius: .45rem; background: #f3f5f8; padding: .7rem; }
	.progress { color: #174d9b; }
	.nested { margin-top: .6rem; padding-left: .6rem; border-left: 2px solid #9aa8c5; }
	pre { overflow-x: auto; white-space: pre-wrap; font: .82rem/1.45 ui-monospace, monospace; }
	footer { margin-top: .65rem; color: #697386; font-size: .75rem; }
	.error { color: #a22525; }
	.warning { color: #8a5a00; }
</style>
