<script lang="ts">
	// A turn's answer: banner + result + error while ended, "still running" +
	// last text while live. This is the block issue 325 says is missing.
	import type { TurnRow } from '../../observation/index.js';
	import Payload from './Payload.svelte';

	let { turn, lastText }: { turn: TurnRow; lastText?: string } = $props();

	const outcome = $derived(
		!turn.ended ? 'running' : turn.interrupted ? 'interrupted' : turn.error ? 'failed' : 'ended'
	);

	const formatDuration = (ms: number) => {
		const seconds = Math.floor(Math.max(0, ms) / 1000);
		return `${Math.floor(seconds / 60)}m ${(seconds % 60).toString().padStart(2, '0')}s`;
	};
	const formatTime = (ms: number) =>
		new Date(ms).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });

	const resultText = $derived.by(() => {
		if (!turn.result) return '';
		try {
			return JSON.stringify(JSON.parse(turn.result), null, 2);
		} catch {
			return turn.result;
		}
	});
</script>

<div class="outcome">
	{#if outcome === 'running'}
		<div class="running">
			<span class="still-running">Still running</span>
			<span class="dot">·</span>
			<span>started {formatTime(turn.started)}</span>
		</div>
		{#if lastText}
			<p class="last-text">{lastText}</p>
		{/if}
	{:else}
		<div class="banner" class:destructive={outcome === 'failed' || outcome === 'interrupted'}>
			<strong>{outcome}</strong>
			<span class="dot">·</span>
			<span>{formatDuration(turn.duration || turn.ended - turn.started)}</span>
			<span class="dot">·</span>
			<span>finished {formatTime(turn.ended)}</span>
		</div>
		{#if turn.result}
			<Payload label="result" text={resultText} maxHeight={420} />
		{/if}
		{#if turn.error}
			<Payload label="error" text={turn.error} tone="error" maxHeight={420} />
		{/if}
		{#if !turn.result && !turn.error}
			<p class="help">No result was recorded for this turn.</p>
		{/if}
	{/if}
</div>

<style>
	.outcome {
		display: flex;
		min-width: 0;
		flex-direction: column;
		gap: 10px;
	}
	.running {
		display: flex;
		align-items: baseline;
		gap: 6px;
		font-size: 14px;
	}
	.still-running {
		font-weight: 600;
	}
	.banner {
		display: flex;
		flex-wrap: wrap;
		align-items: baseline;
		gap: 6px;
		padding: 8px 10px;
		font-size: 14px;
		border: 1px solid var(--map-line);
		border-radius: 7px;
	}
	.banner strong {
		text-transform: capitalize;
	}
	.banner.destructive {
		color: var(--destructive);
		border-color: color-mix(in oklch, var(--destructive) 45%, transparent);
	}
	.dot {
		color: var(--status-muted);
	}
	.last-text {
		display: -webkit-box;
		margin: 0;
		overflow: hidden;
		color: var(--status-muted);
		font-size: 14px;
		line-height: 20px;
		white-space: pre-wrap;
		-webkit-box-orient: vertical;
		-webkit-line-clamp: 3;
		line-clamp: 3;
	}
	.help {
		margin: 0;
		color: var(--status-muted);
		font-size: 13px;
	}
</style>
