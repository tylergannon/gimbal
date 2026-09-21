<script module lang="ts">
	export type StatusWord = 'running' | 'ended' | 'failed' | 'interrupted';
</script>

<script lang="ts">
	// One line: state, elapsed/duration, model, call count, cost. All plain
	// values — the caller derives them from the snapshot, this component never
	// reads TurnRow/SessionRow itself.
	let {
		state,
		duration,
		model,
		calls,
		cost
	}: {
		state: StatusWord;
		duration: string;
		model: string;
		calls: number;
		cost: number;
	} = $props();
</script>

<div class="status-line">
	<span class="word" data-state={state}>{state}</span>
	<span class="dot">·</span>
	<span>{duration}</span>
	<span class="dot">·</span>
	<span class="mono">{model}</span>
	<span class="dot">·</span>
	<span>{calls} model {calls === 1 ? 'call' : 'calls'}</span>
	<span class="dot">·</span>
	<span class="mono">${cost.toFixed(2)}</span>
</div>

<style>
	.status-line {
		display: flex;
		flex-wrap: wrap;
		align-items: baseline;
		gap: 6px;
		min-width: 0;
		padding: 8px 16px;
		color: var(--foreground);
		font-size: 13px;
		line-height: 18px;
		background: color-mix(in oklch, var(--muted) 60%, var(--card));
		border-bottom: 1px solid var(--border);
	}
	.dot {
		color: var(--status-muted);
	}
	.mono {
		font-family: var(--font-mono);
	}
	.word {
		font-weight: 600;
		text-transform: capitalize;
	}
	.word[data-state='running'] {
		color: var(--status-running);
	}
	.word[data-state='ended'] {
		color: var(--foreground);
	}
	.word[data-state='failed'],
	.word[data-state='interrupted'] {
		color: var(--destructive);
	}
</style>
