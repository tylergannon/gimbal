<script lang="ts">
	// Concept A: stable tabs (Activity · Result · Prompt · Usage · Source).
	// One home per fact, borrowed from Temporal's execution detail / Chrome
	// DevTools. See ephemeral/design/issue-325-detail-view-proposal.md §2(a).
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
	const activityCounts = $derived.by(() => {
		const state = transcriptState;
		if (!state) return { messages: 0, tools: 0 };
		const messages = Object.values(state.message).flat();
		const tools = messages.reduce(
			(sum, message) => sum + (message.content ?? []).filter((part: { type: string }) => part.type === 'tool').length,
			0
		);
		return { messages: messages.length, tools };
	});

	// Watchers: other sessions with a turn in this same scope. The concept's
	// props carry no graph, so this is inferred from the snapshot rather than
	// read off the agent_call operation's declared supervisors.
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

	type TabID = 'activity' | 'result' | 'prompt' | 'usage' | 'source';
	const tabs: { id: TabID; label: string }[] = [
		{ id: 'activity', label: 'Activity' },
		{ id: 'result', label: 'Result' },
		{ id: 'prompt', label: 'Prompt' },
		{ id: 'usage', label: 'Usage' },
		{ id: 'source', label: 'Source' }
	];
	let active = $state<TabID>(snapshot.turns[turnID]?.ended ? 'result' : 'activity');

	async function focusTab(id: TabID) {
		await tick();
		document.getElementById(`tab-${id}`)?.focus();
	}
	function onTabKeydown(event: KeyboardEvent) {
		const index = tabs.findIndex((tab) => tab.id === active);
		if (event.key === 'ArrowRight') {
			active = tabs[(index + 1) % tabs.length].id;
		} else if (event.key === 'ArrowLeft') {
			active = tabs[(index - 1 + tabs.length) % tabs.length].id;
		} else if (event.key === 'Home') {
			active = tabs[0].id;
		} else if (event.key === 'End') {
			active = tabs[tabs.length - 1].id;
		} else {
			return;
		}
		event.preventDefault();
		focusTab(active);
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

	<div class="tabs" role="tablist" aria-label="Session detail" tabindex="-1" onkeydown={onTabKeydown}>
		{#each tabs as tab (tab.id)}
			<button
				type="button"
				role="tab"
				id={`tab-${tab.id}`}
				aria-controls={`panel-${tab.id}`}
				aria-selected={active === tab.id}
				tabindex={active === tab.id ? 0 : -1}
				class:active={active === tab.id}
				onclick={() => (active = tab.id)}
			>
				{tab.label}
			</button>
		{/each}
	</div>

	<div class="panel-scroller" class:activity-mode={active === 'activity'}>
		{#if active === 'activity'}
			<div role="tabpanel" id="panel-activity" aria-labelledby="tab-activity" class="panel activity-panel">
				{#if transcriptState && turn}
					<Activity state={transcriptState} {revision} {observation} turn={turn.id} />
				{:else}
					<p class="empty">No transcript recorded for this turn.</p>
				{/if}
			</div>
		{:else if active === 'result'}
			<div role="tabpanel" id="panel-result" aria-labelledby="tab-result" class="panel">
				{#if turn}<Outcome {turn} lastText={lastAssistantText} />{/if}
				{#if watcherRows.length > 0}
					<div class="watchers">
						<div class="section-title">Watchers</div>
						{#each watcherRows as watcher (watcher.session.id)}
							<details class="watcher-row">
								<summary>
									<Pip state={watcherPip(watcher.turn)} />
									<span class="name">{watcher.session.name}</span>
									<span class="verdict">{watcherVerdict(watcher.turn)}</span>
								</summary>
								<Payload label="result" text={watcher.turn.result || watcher.turn.error || 'No result recorded.'} maxHeight={260} />
							</details>
						{/each}
					</div>
				{/if}
				<button type="button" class="link-button" onclick={() => (active = 'activity')}>
					Activity · {activityCounts.messages} messages, {activityCounts.tools} tool calls →
				</button>
			</div>
		{:else if active === 'prompt'}
			<div role="tabpanel" id="panel-prompt" aria-labelledby="tab-prompt" class="panel">
				{#if turn}<PromptAndContext {turn} {snapshot} />{/if}
			</div>
		{:else if active === 'usage'}
			<div role="tabpanel" id="panel-usage" aria-labelledby="tab-usage" class="panel">
				<UsageTable usage={totalUsage} title="Total" />
				{#each Object.entries(byModel) as [model, usage] (model)}
					<UsageTable {usage} title={model} />
				{/each}
			</div>
		{:else if active === 'source'}
			<div role="tabpanel" id="panel-source" aria-labelledby="tab-source" class="panel">
				<dl>
					<dt>Role</dt><dd>{session?.name ?? '—'}</dd>
					<dt>Session</dt><dd><code>{session?.id ?? '—'}</code></dd>
					<dt>Adapter</dt><dd>{session?.adapter ?? '—'}</dd>
					<dt>Model</dt><dd>{session?.model ?? '—'}</dd>
				</dl>
				<p class="help">No declared definition is reachable from this view; showing what the snapshot has.</p>
			</div>
		{/if}
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
	.tabs {
		display: flex;
		flex-shrink: 0;
		gap: 2px;
		padding: 0 8px;
		background: var(--card);
		border-bottom: 1px solid var(--border);
	}
	.tabs button {
		padding: 9px 10px;
		color: var(--status-muted);
		font: inherit;
		font-size: 13px;
		font-weight: 500;
		background: transparent;
		border: 0;
		border-bottom: 2px solid transparent;
		cursor: pointer;
	}
	.tabs button:hover {
		color: var(--foreground);
	}
	.tabs button.active {
		color: var(--foreground);
		border-bottom-color: var(--status-live);
	}
	.tabs button:focus-visible {
		outline: 2px solid var(--status-live);
		outline-offset: -2px;
	}
	.panel-scroller {
		display: flex;
		min-height: 0;
		flex: 1;
		flex-direction: column;
		gap: 16px;
		padding: 14px 16px;
		overflow-x: hidden;
		overflow-y: auto;
	}
	.panel-scroller.activity-mode {
		gap: 0;
		padding: 8px 8px 0;
		overflow: hidden;
	}
	.panel {
		display: flex;
		min-width: 0;
		flex-direction: column;
		gap: 12px;
	}
	.activity-panel {
		min-height: 0;
		flex: 1;
	}
	.empty {
		margin: 0;
		color: var(--status-muted);
		font-size: 13px;
	}
	.section-title {
		padding-bottom: 6px;
		color: var(--foreground);
		font-size: 13px;
		font-weight: 600;
		border-bottom: 1px solid var(--border);
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
	.watcher-row :global(.payload) {
		margin-bottom: 8px;
	}
	.link-button {
		align-self: flex-start;
		padding: 0;
		color: var(--status-live);
		font: inherit;
		font-size: 13px;
		background: transparent;
		border: 0;
		cursor: pointer;
	}
	.link-button:hover {
		text-decoration: underline;
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
