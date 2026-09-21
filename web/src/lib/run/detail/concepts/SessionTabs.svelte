<script lang="ts">
	// Concept A: stable tabs (Activity · Result · Prompt · Usage · Source).
	// One home per fact, borrowed from Temporal's execution detail / Chrome
	// DevTools. See ephemeral/design/issue-325-detail-view-proposal.md §2(a).
	//
	// The concept's body was promoted to ../SessionDetail.svelte, which the
	// real DetailPane now renders. This file stays only as a thin wrapper
	// (its own header + SessionDetail) so the Storybook concept keeps
	// rendering exactly the pane a person would see.
	import BotIcon from '@lucide/svelte/icons/bot';
	import MaximizeIcon from '@lucide/svelte/icons/maximize-2';
	import ExternalLinkIcon from '@lucide/svelte/icons/external-link';
	import { Badge } from '#lib/components/ui/badge/index.js';
	import Pip, { type PipState } from '../../Pip.svelte';
	import SessionDetail from '../SessionDetail.svelte';
	import type { RunObservation, RunSnapshot } from '../../../observation/index.js';
	import type { ActionFeedback } from '../../DetailPane.svelte';

	let {
		snapshot,
		observation,
		revision,
		turnID,
		onsteer,
		onstop,
		onmaximize,
		onopen
	}: {
		snapshot: RunSnapshot;
		observation: RunObservation;
		revision: number;
		turnID: string;
		onsteer: (message: string) => Promise<ActionFeedback>;
		onstop: () => Promise<ActionFeedback>;
		onmaximize?: () => void;
		onopen?: () => void;
	} = $props();

	const turn = $derived(snapshot.turns[turnID]);
	const session = $derived(snapshot.sessions[turn?.session ?? '']);
	const scope = $derived(snapshot.scopes[turn?.scope ?? '']);

	const pipState = $derived<PipState>(
		!turn ? 'not-yet' : turn.error ? 'failed' : turn.ended ? 'ended' : 'running'
	);
	const live = $derived(snapshot.run.status === 'running');
</script>

<div class="pane">
	<header class="head">
		<div class="title-row">
			<BotIcon size={16} />
			<h2>{session?.name ?? turn?.session ?? 'session'}</h2>
			<Badge variant="outline">agent call</Badge>
			<span class="spacer"></span>
			<Pip state={pipState} />
			{#if onmaximize}
				<button type="button" class="icon-btn" aria-label="Maximize the detail pane" onclick={onmaximize}>
					<MaximizeIcon size={16} />
				</button>
			{/if}
			{#if onopen}
				<button type="button" class="icon-btn" aria-label="Open the session page" onclick={onopen}>
					<ExternalLinkIcon size={16} />
				</button>
			{/if}
		</div>
		<div class="placement" title={`${scope?.key || 'root'} · ${session?.id ?? ''}`}>
			{scope?.key || 'root'} · {session?.id ?? ''}
		</div>
	</header>

	{#if turn}
		<SessionDetail {snapshot} {observation} {revision} {turn} {live} {onsteer} {onstop} />
	{/if}
</div>

<style>
	.pane {
		display: flex;
		min-width: 0;
		height: 100%;
		flex-direction: column;
		overflow: hidden;
		color: var(--card-foreground);
		background: var(--card);
		border: 1px solid var(--map-line);
		border-radius: 8px;
	}
	.head {
		display: flex;
		flex-direction: column;
		flex-shrink: 0;
		gap: 8px;
		padding: 12px 14px 10px;
		background: color-mix(in oklch, var(--muted) 60%, var(--card));
		border-bottom: 1px solid var(--border);
	}
	.title-row {
		display: flex;
		align-items: center;
		gap: 8px;
	}
	h2 {
		margin: 0;
		overflow: hidden;
		font-size: 15px;
		line-height: 22px;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.spacer {
		flex: 1;
	}
	.placement {
		overflow: hidden;
		color: var(--status-muted);
		font-family: var(--font-mono);
		font-size: 13px;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.icon-btn {
		display: inline-flex;
		width: 26px;
		height: 26px;
		flex-shrink: 0;
		align-items: center;
		justify-content: center;
		color: var(--foreground);
		background: transparent;
		border: 0;
		border-radius: 6px;
		cursor: pointer;
	}
	.icon-btn:hover {
		background: var(--muted);
	}
</style>
