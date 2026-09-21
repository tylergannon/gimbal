<script lang="ts">
	// Fixed footer: steer or stop the turn this pane is showing. Closes itself
	// once the turn has ended — steering a finished turn is not a thing.
	import { Button } from '#lib/components/ui/button/index.js';
	import { Textarea } from '#lib/components/ui/textarea/index.js';
	import type { ActionFeedback } from '../DetailPane.svelte';

	let {
		onsteer,
		onstop,
		disabled = false,
		ended = false
	}: {
		onsteer: (message: string) => Promise<ActionFeedback>;
		onstop: () => Promise<ActionFeedback>;
		disabled?: boolean;
		ended?: boolean;
	} = $props();

	let message = $state('');
	let pending = $state(false);
	let feedback = $state<ActionFeedback>();

	async function run(action: () => Promise<ActionFeedback>, clear?: () => void) {
		pending = true;
		feedback = undefined;
		try {
			feedback = await action();
			if (feedback.ok) clear?.();
		} catch (error) {
			feedback = { ok: false, message: error instanceof Error ? error.message : String(error) };
		} finally {
			pending = false;
		}
	}
</script>

{#if ended}
	<footer class="steer-box ended">
		<p>This turn has ended. Steering is closed.</p>
	</footer>
{:else}
	<footer class="steer-box">
		<Textarea rows={2} bind:value={message} placeholder="Steer this turn…" disabled={disabled || pending} />
		<div class="row">
			<span class="help">Lands before the next model call. Dropped if the turn ends first.</span>
			<Button
				variant="outline"
				size="sm"
				disabled={disabled || pending}
				onclick={() => run(() => onstop())}
			>
				Stop turn
			</Button>
			<Button
				size="sm"
				disabled={disabled || pending || !message.trim()}
				onclick={() => run(() => onsteer(message), () => (message = ''))}
			>
				Steer
			</Button>
		</div>
		{#if feedback}
			<p class="feedback" class:error={!feedback.ok} role="status">{feedback.message}</p>
		{/if}
	</footer>
{/if}

<style>
	.steer-box {
		display: flex;
		flex-direction: column;
		gap: 8px;
		padding: 12px 16px;
		background: color-mix(in oklch, var(--muted) 60%, var(--card));
		border-top: 1px solid var(--border);
	}
	.steer-box.ended p {
		margin: 0;
		color: var(--status-muted);
		font-size: 13px;
		line-height: 18px;
	}
	.row {
		display: flex;
		align-items: center;
		gap: 8px;
	}
	.help {
		flex: 1;
		color: var(--status-muted);
		font-size: 13px;
		line-height: 18px;
	}
	.feedback {
		margin: 0;
		padding: 7px 9px;
		color: var(--foreground);
		font-size: 13px;
		background: var(--map-live-soft);
		border-radius: 6px;
	}
	.feedback.error {
		color: var(--destructive);
		background: color-mix(in oklch, var(--destructive) 10%, transparent);
	}
</style>
