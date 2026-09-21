<script module lang="ts">
	// Promoted from concepts/SessionTabs.svelte: the body of the agent-turn
	// detail pane (status line, tab bar, tab panels, steer footer). DetailPane
	// owns the one pane header used for every kind of selection; this owns
	// everything below it for a turn. See
	// ephemeral/design/issue-325-detail-view-proposal.md §2(a).
	import type { PipState } from '../Pip.svelte';

	/** One watcher of this call, as DetailPane derives it from the operation's
	 * declared supervisors and the watcher's latest turn in this scope. */
	export type WatcherRow = {
		session: string;
		role: string;
		pip: PipState;
		verdict: string;
		result: string;
	};
</script>

<script lang="ts">
	import type { AgentCall } from '../../workflow/types.js';
	import Pip from '../Pip.svelte';
	import StatusLine from './StatusLine.svelte';
	import Outcome from './Outcome.svelte';
	import Activity from './Activity.svelte';
	import SteerBox from './SteerBox.svelte';
	import PromptAndContext from './PromptAndContext.svelte';
	import UsageTable from './UsageTable.svelte';
	import Payload from './Payload.svelte';
	import TabBar from './TabBar.svelte';
	import { usageOf, type RunObservation, type RunSnapshot, type TurnRow } from '../../observation/index.svelte.js';
	import type { ActionFeedback } from '../DetailPane.svelte';

	let {
		snapshot,
		observation,
		turn,
		definition,
		watchers = [],
		live,
		onsteer,
		onstop,
		onscope
	}: {
		snapshot: RunSnapshot;
		observation?: RunObservation;
		turn: TurnRow;
		definition?: AgentCall;
		watchers?: WatcherRow[];
		live: boolean;
		onsteer: (message: string) => Promise<ActionFeedback>;
		onstop?: () => Promise<ActionFeedback>;
		onscope?: (scopeKey: string) => void;
	} = $props();

	const session = $derived(snapshot.sessions[turn.session]);
	const ended = $derived(!!turn.ended);

	const outcomeState = $derived.by(() => {
		if (!turn.ended) return 'running' as const;
		if (turn.error) return 'failed' as const;
		if (turn.interrupted) return 'interrupted' as const;
		return 'ended' as const;
	});

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
		outcomeState === 'running' ? formatDuration(now - turn.started) : formatDuration(turn.duration || turn.ended - turn.started)
	);
	const calls = $derived(snapshot.model_calls[turn.id]?.length ?? 0);
	const cost = $derived(
		Object.values(snapshot.turn_usage[turn.id] ?? {}).reduce((sum, usage) => sum + usage.stated_cost, 0)
	);

	const transcriptState = $derived(observation?.state(turn.id));
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
	// DetailPane keys this component by turn identity, so this seed is kept for
	// live updates of the same turn and naturally resets for another turn.
	let active = $state<TabID>(turn.ended ? 'result' : 'activity');
</script>

<StatusLine state={outcomeState} duration={durationText} model={session?.model ?? ''} {calls} {cost} />

<TabBar {tabs} active={active} onselect={(id) => (active = id as TabID)} />

<div class="panel-scroller" class:activity-mode={active === 'activity'}>
	{#if active === 'activity'}
		<div role="tabpanel" id="panel-activity" aria-labelledby="tab-activity" class="panel activity-panel">
			{#if transcriptState && observation}
				<Activity state={transcriptState} {observation} turn={turn.id} />
			{:else}
				<p class="empty">No transcript recorded for this turn.</p>
			{/if}
		</div>
	{:else if active === 'result'}
		<div role="tabpanel" id="panel-result" aria-labelledby="tab-result" class="panel">
			<Outcome {turn} lastText={lastAssistantText} />
			{#if watchers.length > 0}
				<div class="watchers">
					<div class="section-title">Watchers</div>
					{#each watchers as watcher (watcher.session)}
						<details class="watcher-row">
							<summary>
								<Pip state={watcher.pip} />
								<span class="name">{watcher.session}</span>
								<span class="verdict">{watcher.verdict}</span>
							</summary>
							<Payload label="result" text={watcher.result} maxHeight={260} />
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
			<PromptAndContext {turn} {snapshot} {onscope} />
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
				<dt>Role</dt><dd>{definition?.role ?? '—'}</dd>
				<dt>Session</dt><dd>{definition?.session ?? session?.name ?? '—'}</dd>
				<dt>Adapter</dt><dd>{session?.adapter ?? '—'}</dd>
				<dt>Model</dt><dd>{session?.model ?? '—'}</dd>
				<dt>Source</dt>
				<dd>{#if definition}<code>{definition.file}:{definition.line}</code>{:else}—{/if}</dd>
			</dl>
			{#if definition?.prompt}
				<Payload label="declared prompt" text={definition.prompt} maxHeight={320} />
			{:else}
				<p class="help">No declared definition is reachable from this view; showing what the snapshot has.</p>
			{/if}
		</div>
	{/if}
</div>

<SteerBox
	onsteer={(message) => onsteer(message)}
	onstop={() => onstop?.() ?? Promise.resolve({ ok: false, message: 'Stopping is unavailable.' })}
	ended={ended || !live}
/>

<style>
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
		padding: 8px 16px 0;
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
