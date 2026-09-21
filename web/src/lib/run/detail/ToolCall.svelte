<script lang="ts">
	import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
	import type { JSONObject } from '../../sessionstate/index.js';
	import Payload from './Payload.svelte';

	let { part, maxHeight = 260 }: { part: JSONObject; maxHeight?: number } = $props();

	const DIGEST_KEYS = [
		'file_path',
		'path',
		'command',
		'pattern',
		'url',
		'query',
		'description'
	] as const;

	const rowText = (value: unknown): string =>
		typeof value === 'string' ? value : JSON.stringify(value, null, 2);

	const truncateLeft = (value: string, max = 60): string =>
		value.length <= max ? value : `…${value.slice(value.length - (max - 1))}`;

	const digestOf = (input: unknown): string => {
		if (input === undefined || input === null) return '';
		if (typeof input === 'string') return input;
		if (typeof input !== 'object') return String(input);
		const obj = input as Record<string, unknown>;
		for (const key of DIGEST_KEYS) if (typeof obj[key] === 'string') return obj[key] as string;
		for (const key of Object.keys(obj)) if (typeof obj[key] === 'string') return obj[key] as string;
		return '';
	};

	const formatDuration = (ms: number): string => {
		if (ms < 1000) return `${Math.round(ms)}ms`;
		const seconds = ms / 1000;
		return seconds < 10 ? `${seconds.toFixed(1)}s` : `${Math.round(seconds)}s`;
	};

	// eventText / transcriptOf: a subagent's own transcript, nested one level
	// further inside a tool part. Ported from the pre-slice-1 MessageRow.
	const text = (value: unknown) => (typeof value === 'string' ? value : JSON.stringify(value, null, 2));
	const eventText = (event: JSONObject) => {
		const native = event.event ?? event;
		const delta = native.delta ?? native.event?.delta;
		return (
			delta?.text ??
			delta?.thinking ??
			native.message?.content?.map((part: JSONObject) => part.text).filter(Boolean).join('\n') ??
			text(native)
		);
	};
	const transcriptOf = (value: JSONObject | undefined) => {
		const content = value?.content?.find((entry: JSONObject) => entry.type === 'transcript');
		const events = value?.metadata?.transcript ?? content?.events;
		return Array.isArray(events) ? events : undefined;
	};

	const status = $derived(part.state?.status ?? 'pending');
	const statusLabel = $derived(status === 'completed' ? 'ok' : status);
	const transcript = $derived(transcriptOf(part.state));

	let manualOpen = $state<boolean | undefined>(undefined);
	const openByDefault = $derived(
		status === 'streaming' || status === 'running' || status === 'pending' || status === 'error'
	);
	const open = $derived(manualOpen ?? openByDefault);

	let now = $state(Date.now());
	$effect(() => {
		if (status !== 'running' && status !== 'streaming') return;
		const timer = setInterval(() => (now = Date.now()), 1000);
		return () => clearInterval(timer);
	});
	const duration = $derived.by(() => {
		const time = part.time;
		const start = time?.created ?? time?.ran;
		if (start === undefined) return undefined;
		const end = time?.completed ?? (status === 'running' || status === 'streaming' ? now : undefined);
		if (end === undefined) return undefined;
		return formatDuration(Math.max(0, end - start));
	});

	const digest = $derived(truncateLeft(digestOf(part.state?.input)));

	const inputEntries = $derived.by(() => {
		const input = part.state?.input;
		if (input === null || input === undefined || typeof input !== 'object' || Array.isArray(input))
			return [];
		return Object.entries(input as Record<string, unknown>);
	});
	const rawInputText = $derived(typeof part.state?.input === 'string' ? part.state.input : undefined);

	const outputParts = $derived(
		(part.state?.content ?? []).filter((entry: JSONObject) => entry.type !== 'transcript')
	);
	const progressMetadata = $derived(
		status === 'running' && part.state?.metadata && Object.keys(part.state.metadata).length > 0 && transcript === undefined
			? part.state.metadata
			: undefined
	);
	const errorText = $derived(part.state?.error !== undefined ? rowText(part.state.error) : undefined);
</script>

<article class="tool-call">
	<button type="button" class="summary" aria-expanded={open} onclick={() => (manualOpen = !open)}>
		<ChevronRightIcon size={14} class={open ? 'chevron open' : 'chevron'} />
		<span class="name">{part.name}</span>
		{#if digest}<span class="digest" title={digestOf(part.state?.input)}>{digest}</span>{/if}
		<span class="spacer"></span>
		<span class="status" data-status={status}>{statusLabel}</span>
		{#if duration}<span class="duration">{duration}</span>{/if}
	</button>

	{#if open}
		<div class="body">
			{#if inputEntries.length > 0}
				<div class="input-block">
					<div class="input-strip"><span class="label">input</span></div>
					{#each inputEntries as [key, value] (key)}
						<div class="kv-row">
							<div class="kv-key">{key}</div>
							<pre class="kv-value" style={`max-height: ${maxHeight}px`}>{rowText(value)}</pre>
						</div>
					{/each}
				</div>
			{:else if rawInputText}
				<Payload label="input" text={rawInputText} {maxHeight} />
			{/if}

			{#each outputParts as content, index (index)}
				{#if content.type === 'text'}
					<Payload label="output" text={content.text ?? ''} {maxHeight} />
				{:else}
					<Payload label={content.type ?? 'output'} text={rowText(content)} {maxHeight} />
				{/if}
			{/each}

			{#if errorText}
				<Payload label="error" text={errorText} tone="error" {maxHeight} />
			{/if}

			{#if progressMetadata}
				<Payload label="progress" text={rowText(progressMetadata)} {maxHeight} />
			{/if}

			{#if transcript}
				<details class="nested">
					<summary>Subagent transcript · {transcript.length} events</summary>
					<div class="transcript-events">
						{#each transcript as event, index (index)}
							<pre class="event">{eventText(event)}</pre>
						{/each}
					</div>
				</details>
			{/if}
		</div>
	{/if}
</article>

<style>
	.tool-call {
		display: flex;
		min-width: 0;
		flex-direction: column;
		margin-left: 8px;
		background: var(--background);
		border: 1px solid var(--map-line);
		border-radius: 6px;
	}
	.summary {
		display: flex;
		align-items: center;
		gap: 8px;
		min-width: 0;
		width: 100%;
		padding: 6px 8px;
		color: var(--foreground);
		font: inherit;
		text-align: left;
		background: transparent;
		border: none;
		border-radius: 6px;
		cursor: pointer;
	}
	.summary:hover {
		background: color-mix(in oklch, var(--surface) 60%, transparent);
	}
	.summary :global(.chevron) {
		flex-shrink: 0;
		color: var(--status-muted);
		transition: transform 0.1s ease;
	}
	.summary :global(.chevron.open) {
		transform: rotate(90deg);
	}
	.name {
		flex-shrink: 0;
		color: var(--foreground);
		font-family: var(--font-mono);
		font-size: 13px;
		font-weight: 600;
	}
	.digest {
		overflow: hidden;
		min-width: 0;
		flex: 1;
		color: var(--status-muted);
		font-family: var(--font-mono);
		font-size: 13px;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.spacer {
		flex: 0 1 8px;
	}
	.status {
		flex-shrink: 0;
		color: var(--status-muted);
		font-size: 13px;
		text-transform: capitalize;
	}
	.status[data-status='running'],
	.status[data-status='streaming'] {
		color: var(--status-running);
	}
	.status[data-status='error'] {
		color: var(--destructive);
	}
	.duration {
		flex-shrink: 0;
		color: var(--status-muted);
		font-family: var(--font-mono);
		font-size: 13px;
	}
	.body {
		display: flex;
		flex-direction: column;
		gap: 8px;
		min-width: 0;
		padding: 0 8px 8px;
	}
	.input-block {
		display: flex;
		flex-direction: column;
		min-width: 0;
		background: var(--code);
		border: 1px solid var(--map-line);
		border-radius: 6px;
		overflow: hidden;
	}
	.input-strip {
		padding: 4px 8px;
		color: var(--foreground);
		font-size: 13px;
		font-weight: 600;
		background: var(--surface);
		border-bottom: 1px solid var(--map-line);
	}
	.kv-row {
		min-width: 0;
		padding: 6px 8px;
	}
	.kv-row + .kv-row {
		border-top: 1px solid var(--border);
	}
	.kv-key {
		margin-bottom: 4px;
		color: var(--status-muted);
		font-family: var(--font-mono);
		font-size: 13px;
	}
	.kv-value {
		overflow: auto;
		min-width: 0;
		margin: 0;
		color: var(--code-foreground);
		font-family: var(--font-mono);
		font-size: 13px;
		line-height: 1.5;
		white-space: pre-wrap;
		overflow-wrap: anywhere;
	}
	.nested {
		min-width: 0;
		background: var(--surface);
		border: 1px solid var(--map-line);
		border-radius: 6px;
	}
	.nested summary {
		padding: 6px 8px;
		color: var(--status-muted);
		font-size: 13px;
		cursor: pointer;
	}
	.transcript-events {
		display: flex;
		flex-direction: column;
		gap: 6px;
		padding: 0 8px 8px;
	}
	.event {
		overflow: auto;
		min-width: 0;
		max-height: 200px;
		margin: 0;
		padding: 6px 8px;
		color: var(--code-foreground);
		font-family: var(--font-mono);
		font-size: 13px;
		line-height: 1.5;
		white-space: pre-wrap;
		overflow-wrap: anywhere;
		background: var(--code);
		border: 1px solid var(--map-line);
		border-radius: 6px;
	}
</style>
