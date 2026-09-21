<script lang="ts">
	// Concept B: one ordered scroll, no tabs. Outcome sticks to the top once
	// the turn has ended (bounded by Payload's own max-height, so it never
	// eats the whole pane); Activity owns the remaining space with its own
	// windowed, pinning transcript; Prompt/Usage/Watchers/Source sit as
	// collapsed one-line disclosures at the end, each reachable in one click
	// from the jump-link row under the status line.
	import { tick } from 'svelte';
	import BotIcon from '@lucide/svelte/icons/bot';
	import MaximizeIcon from '@lucide/svelte/icons/maximize-2';
	import ExternalLinkIcon from '@lucide/svelte/icons/external-link';
	import { Badge } from '#lib/components/ui/badge/index.js';
	import Pip, { type PipState } from '../../Pip.svelte';
	import StatusLine from '../StatusLine.svelte';
	import Outcome from '../Outcome.svelte';
	import Activity from '../Activity.svelte';
	import SteerBox from '../SteerBox.svelte';
	import PromptAndContext from '../PromptAndContext.svelte';
	import UsageTable from '../UsageTable.svelte';
	import Payload from '../Payload.svelte';
	import { usageOf, type RunObservation, type RunSnapshot, type TurnRow } from '../../../observation/index.js';
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
	const ended = $derived(!!turn?.ended);

	const outcomeState = $derived.by(() => {
		if (!turn || !turn.ended) return 'running' as const;
		if (turn.error) return 'failed' as const;
		if (turn.interrupted) return 'interrupted' as const;
		return 'ended' as const;
	});
	const pipState = $derived<PipState>(
		!turn ? 'not-yet' : turn.error ? 'failed' : turn.ended ? 'ended' : 'running'
	);

	let now = $state(Date.now());
	$effect(() => {
		if (outcomeState !== 'running') return;
		const timer = setInterval(() => (now = Date.now()), 1000);
		return () => clearInterval(timer);
	});
	const formatDuration = (ms: number) => {
		const seconds = Math.floor(Math.max(0, ms) / 1000);
		return `${Math.floor(seconds / 60)}m ${(seconds % 60).toString().padStart(2, '0')}s`;
	};
	const durationText = $derived(
		!turn
			? '0m 00s'
			: outcomeState === 'running'
				? formatDuration(now - turn.started)
				: formatDuration(turn.duration || turn.ended - turn.started)
	);
	const calls = $derived(snapshot.model_calls[turnID]?.length ?? 0);
	const cost = $derived(
		Object.values(snapshot.turn_usage[turnID] ?? {}).reduce((sum, usage) => sum + usage.stated_cost, 0)
	);

	const transcriptState = $derived.by(() => {
		revision;
		return turn ? observation.state(turn.id) : undefined;
	});
	const lastAssistantText = $derived.by(() => {
		const state = transcriptState;
		if (!state) return undefined;
		const messages = Object.values(state.message).flat();
		for (let i = messages.length - 1; i >= 0; i--) {
			const message = messages[i];
			if (message.type !== 'assistant') continue;
			const textPart = [...(message.content ?? [])].reverse().find((part) => part.type === 'text');
			if (textPart?.text) return textPart.text as string;
		}
		return undefined;
	});

	const watcherRows = $derived.by(() => {
		if (!turn) return [];
		const latest = new Map<string, TurnRow>();
		for (const row of Object.values(snapshot.turns)) {
			if (row.scope !== turn.scope || row.session === turn.session) continue;
			const current = latest.get(row.session);
			if (!current || row.started > current.started) latest.set(row.session, row);
		}
		return [...latest.entries()]
			.map(([sessionID, watcherTurn]) => ({ session: snapshot.sessions[sessionID], turn: watcherTurn }))
			.filter((row): row is { session: NonNullable<typeof row.session>; turn: TurnRow } => !!row.session);
	});
	const watcherVerdict = (row: TurnRow) => (!row.ended ? 'running' : row.error ? 'failed' : 'ended');
	const watcherPip = (row: TurnRow): PipState => (!row.ended ? 'running' : row.error ? 'failed' : 'ended');

	const totalUsage = $derived(snapshot.totals.sessions[session?.id ?? '']?.all ?? usageOf(undefined));
	const byModel = $derived(snapshot.totals.sessions[session?.id ?? '']?.by_model ?? {});

	let promptOpen = $state(false);
	let usageOpen = $state(false);
	let sourceOpen = $state(false);

	function jumpTo(id: string, open?: () => void) {
		open?.();
		tick().then(() => {
			document.getElementById(id)?.scrollIntoView({ block: 'start', behavior: 'smooth' });
		});
	}
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

	<StatusLine state={outcomeState} duration={durationText} model={session?.model ?? ''} {calls} {cost} />

	<div class="jump-links">
		<button type="button" onclick={() => jumpTo('result')}>Result</button>
		<span class="dot">·</span>
		<button type="button" onclick={() => jumpTo('prompt', () => (promptOpen = true))}>Prompt</button>
		<span class="dot">·</span>
		<button type="button" onclick={() => jumpTo('usage', () => (usageOpen = true))}>Usage</button>
		<span class="dot">·</span>
		<button type="button" onclick={() => jumpTo('source', () => (sourceOpen = true))}>Source</button>
	</div>

	<div class="scroller">
		<div id="result" class="outcome-block">
			{#if ended && turn}
				<div class="sticky-outcome">
					<Outcome {turn} />
				</div>
			{:else if turn}
				<p class="running-line">Still running · {lastAssistantText ?? 'no assistant text yet'}</p>
			{/if}
		</div>

		<div class="activity-wrap">
			{#if transcriptState && turn}
				<Activity state={transcriptState} {revision} {observation} turn={turn.id} />
			{:else}
				<p class="empty">No transcript recorded for this turn.</p>
			{/if}
		</div>

		<details id="prompt" class="disclosure" bind:open={promptOpen}>
			<summary>Prompt and context</summary>
			{#if turn}<PromptAndContext {turn} {snapshot} />{/if}
		</details>

		<details id="usage" class="disclosure" bind:open={usageOpen}>
			<summary>Usage</summary>
			<div class="usage-body">
				<UsageTable usage={totalUsage} title="Total" />
				{#each Object.entries(byModel) as [model, usage] (model)}
					<UsageTable {usage} title={model} />
				{/each}
			</div>
		</details>

		<details id="watchers" class="disclosure">
			<summary>Watchers <span class="count">{watcherRows.length}</span></summary>
			<div class="watchers">
				{#each watcherRows as watcher (watcher.session.id)}
					<details class="watcher-row">
						<summary>
							<Pip state={watcherPip(watcher.turn)} />
							<span class="name">{watcher.session.name}</span>
							<span class="verdict">{watcherVerdict(watcher.turn)}</span>
						</summary>
						<Payload label="result" text={watcher.turn.result || watcher.turn.error || 'No result recorded.'} maxHeight={260} />
					</details>
				{:else}
					<p class="help">No watcher turns recorded in this scope.</p>
				{/each}
			</div>
		</details>

		<details id="source" class="disclosure" bind:open={sourceOpen}>
			<summary>Source</summary>
			<dl>
				<dt>Role</dt><dd>{session?.name ?? '—'}</dd>
				<dt>Session</dt><dd><code>{session?.id ?? '—'}</code></dd>
				<dt>Adapter</dt><dd>{session?.adapter ?? '—'}</dd>
				<dt>Model</dt><dd>{session?.model ?? '—'}</dd>
			</dl>
			<p class="help">No declared definition is reachable from this view; showing what the snapshot has.</p>
		</details>
	</div>

	{#if turn}<SteerBox {onsteer} {onstop} {ended} />{/if}
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
	.jump-links {
		display: flex;
		flex-shrink: 0;
		align-items: center;
		gap: 6px;
		padding: 6px 16px;
		background: var(--card);
		border-bottom: 1px solid var(--border);
	}
	.jump-links button {
		padding: 2px 0;
		color: var(--status-live);
		font: inherit;
		font-size: 13px;
		background: transparent;
		border: 0;
		cursor: pointer;
	}
	.jump-links button:hover {
		text-decoration: underline;
	}
	.jump-links .dot {
		color: var(--status-muted);
	}
	.scroller {
		display: flex;
		min-height: 0;
		flex: 1;
		flex-direction: column;
		overflow-x: hidden;
		overflow-y: auto;
	}
	.outcome-block {
		flex-shrink: 0;
	}
	.sticky-outcome {
		position: sticky;
		top: 0;
		z-index: 2;
		padding: 12px 16px;
		background: var(--card);
		border-bottom: 1px solid var(--border);
	}
	.running-line {
		overflow: hidden;
		margin: 0;
		padding: 10px 16px;
		color: var(--foreground);
		font-size: 14px;
		text-overflow: ellipsis;
		white-space: nowrap;
		border-bottom: 1px solid var(--border);
	}
	.activity-wrap {
		display: flex;
		min-height: 320px;
		flex: 1;
		flex-direction: column;
		padding: 10px 8px;
	}
	.empty {
		margin: 0;
		padding: 0 8px;
		color: var(--status-muted);
		font-size: 13px;
	}
	.disclosure {
		flex-shrink: 0;
		padding: 0 16px;
		border-top: 1px solid var(--border);
	}
	.disclosure:last-child {
		border-bottom: 1px solid var(--border);
	}
	.disclosure summary {
		display: flex;
		align-items: center;
		gap: 8px;
		height: 40px;
		color: var(--foreground);
		font-size: 13px;
		font-weight: 500;
		cursor: pointer;
		list-style: none;
	}
	.disclosure .count {
		color: var(--status-muted);
		font-weight: 400;
	}
	.disclosure[open] summary {
		border-bottom: 1px solid var(--border);
	}
	.disclosure > :global(*:not(summary)) {
		margin: 10px 0;
	}
	.usage-body {
		display: flex;
		flex-direction: column;
		gap: 12px;
	}
	.watchers {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}
	.watcher-row {
		padding: 2px 8px;
		background: var(--surface);
		border: 1px solid var(--map-line);
		border-radius: 7px;
	}
	.watcher-row summary {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 8px 0;
		height: auto;
		font-size: 13px;
		cursor: pointer;
		list-style: none;
	}
	.watcher-row .name {
		font-weight: 500;
	}
	.watcher-row .verdict {
		margin-left: auto;
		color: var(--status-muted);
		text-transform: capitalize;
	}
	dl {
		display: grid;
		grid-template-columns: 92px minmax(0, 1fr);
		gap: 6px 12px;
		margin: 0;
		font-size: 13px;
	}
	dt {
		color: var(--status-muted);
	}
	dd {
		min-width: 0;
		margin: 0;
		overflow-wrap: anywhere;
	}
	code {
		font-family: var(--font-mono);
	}
	.help {
		margin: 0;
		color: var(--status-muted);
		font-size: 13px;
	}
</style>
