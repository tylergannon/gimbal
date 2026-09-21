<script module lang="ts">
	// One agent session on its own: every turn it has run, and the full turn
	// detail (SessionDetail) for the chosen one. The run page shows it in place
	// of the map and pane when a session is opened.
	import type { ActionFeedback } from '../DetailPane.svelte';
	import type { Steer } from '../../../routes/steer.remote.js';
	import type { PipState } from '../Pip.svelte';
</script>

<script lang="ts">
	import { clock } from '../../clock.svelte.js';
	import type { AgentCall } from '../../workflow/types.js';
	import { usageOf, type RunObservation, type RunSnapshot, type TurnRow } from '../../observation/index.js';
	import { turnOutcome } from '../watchers.js';
	import Pip from '../Pip.svelte';
	import SessionDetail, { type WatcherRow } from './SessionDetail.svelte';
	import UsageTable from './UsageTable.svelte';

	let {
		snapshot,
		observation,
		revision = 0,
		sessionID,
		definition,
		watchers = [],
		onsteer,
		onstop,
		onscope
	}: {
		snapshot: RunSnapshot;
		observation?: RunObservation;
		revision?: number;
		sessionID: string;
		definition?: AgentCall;
		watchers?: WatcherRow[];
		onsteer?: (request: Steer) => Promise<ActionFeedback>;
		onstop?: (turn: TurnRow) => Promise<ActionFeedback>;
		onscope?: (scopeKey: string) => void;
	} = $props();

	const session = $derived(snapshot.sessions[sessionID]);
	const live = $derived(snapshot.run.status === 'running');
	const turns = $derived(
		Object.values(snapshot.turns)
			.filter((row) => row.session === sessionID)
			.sort((left, right) => left.started - right.started)
	);
	const runningTurn = $derived(turns.find((row) => row.ended === 0));
	// The turn the reader picked; until then, the running turn or the last one.
	let pickedTurnID = $state<string>();
	const chosenTurn = $derived<TurnRow | undefined>(
		(pickedTurnID ? turns.find((row) => row.id === pickedTurnID) : undefined) ?? runningTurn ?? turns.at(-1)
	);
	const totalUsage = $derived(snapshot.totals.sessions[sessionID]?.all ?? usageOf(undefined));

	function ordinalOf(turn: TurnRow): number {
		return turns.findIndex((row) => row.id === turn.id) + 1;
	}
	function turnPip(turn: TurnRow): PipState {
		if (!turn.ended) return 'running';
		if (turn.interrupted) return 'ended';
		return turn.error ? 'failed' : 'ended';
	}
	function turnDuration(turn: TurnRow): string {
		const ms = turn.ended ? turn.duration || turn.ended - turn.started : Math.max(0, clock.now - turn.started);
		const seconds = Math.floor(ms / 1000);
		return `${Math.floor(seconds / 60)}m ${(seconds % 60).toString().padStart(2, '0')}s`;
	}
	function turnUsage(turn: TurnRow): { output: number; cost: number } {
		const rows = Object.values(snapshot.turn_usage[turn.id] ?? {});
		return {
			output: rows.reduce((sum, row) => sum + row.output, 0),
			cost: rows.reduce((sum, row) => sum + row.stated_cost, 0)
		};
	}
</script>

{#snippet sessionBlock()}
	{#if session}
		<div class="section-title">Session</div>
		<dl>
			<dt>Name</dt>
			<dd>{session.name}</dd>
			<dt>Model</dt>
			<dd>{session.model}</dd>
			<dt>Adapter</dt>
			<dd>{session.adapter}</dd>
			<dt>Session id</dt>
			<dd><code class="session-id" title={session.id}>{session.id}</code></dd>
			<dt>Created in</dt>
			<dd>
				<button type="button" class="scope-link" onclick={() => onscope?.(session.scope)}>
					{session.scope || 'root'}
				</button>
			</dd>
		</dl>
		<UsageTable usage={totalUsage} title="Session total" />
	{/if}
{/snippet}

<div class="session-page">
	{#if !session}
		<div class="missing">
			<p>Session <code>{sessionID}</code> was not found in this run.</p>
		</div>
	{:else}
		<div class="rail">
			<section class="turns">
				<div class="section-title">Turns</div>
				<div class="turn-list full">
					{#each turns as turn (turn.id)}
						{const chosen = $derived(turn.id === chosenTurn?.id)}
						{const usage = $derived(turnUsage(turn))}
						<button
							type="button"
							class={["turn-row", { chosen }]}
							aria-current={chosen ? 'true' : undefined}
							onclick={() => (pickedTurnID = turn.id)}
						>
							<span class="ordinal">Turn {ordinalOf(turn)}</span>
							<Pip state={turnPip(turn)} />
							<span class="state">{turnOutcome(turn)}</span>
							<span class="scope">{turn.scope || 'root'}</span>
							<span class="duration">{turnDuration(turn)}</span>
							<span class="usage">{usage.output.toLocaleString()} out · ${usage.cost.toFixed(2)}</span>
						</button>
					{/each}
				</div>
				<div class="turn-list compact">
					{#each turns as turn (turn.id)}
						{const chosen = $derived(turn.id === chosenTurn?.id)}
						<button
							type="button"
							class={["turn-chip", { chosen }]}
							aria-current={chosen ? 'true' : undefined}
							onclick={() => (pickedTurnID = turn.id)}
						>
							<Pip state={turnPip(turn)} />
							Turn {ordinalOf(turn)}
						</button>
					{/each}
				</div>
			</section>
			<details class="session-details" open>
				<summary>Session details</summary>
				{@render sessionBlock()}
			</details>
		</div>
		<div class="detail-area">
			{#if chosenTurn}
				<SessionDetail
					{snapshot}
					{observation}
					{revision}
					turn={chosenTurn}
					{definition}
					{watchers}
					{live}
					onsteer={(message) =>
						onsteer?.({ run: snapshot.run.id, session: sessionID, message }) ??
						Promise.resolve({ ok: false, message: 'Steering is unavailable.' })}
					onstop={onstop ? () => onstop(chosenTurn) : undefined}
					{onscope}
				/>
			{:else}
				<p class="empty">This session has no recorded turns.</p>
			{/if}
		</div>
	{/if}
</div>

<style>
	.session-page {
		display: flex;
		min-height: 0;
		flex: 1;
		min-width: 0;
		background: var(--background);
	}
	.missing {
		display: flex;
		flex: 1;
		flex-direction: column;
		gap: 8px;
		align-items: flex-start;
		justify-content: center;
		padding: 24px;
	}
	.missing p {
		margin: 0;
		font-size: 14px;
	}
	.rail {
		display: flex;
		width: 260px;
		flex-shrink: 0;
		flex-direction: column;
		gap: 12px;
		padding: 12px;
		overflow-y: auto;
		background: color-mix(in oklch, var(--muted) 40%, var(--background));
		border-right: 1px solid var(--map-line);
	}
	.detail-area {
		display: flex;
		min-width: 0;
		flex: 1;
		flex-direction: column;
		overflow: hidden;
		background: var(--card);
	}
	.empty {
		margin: 16px;
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
	.turns {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}
	.turn-list.full {
		display: flex;
		flex-direction: column;
		gap: 6px;
	}
	.turn-list.compact {
		display: none;
	}
	.turn-row {
		display: flex;
		flex-direction: column;
		gap: 3px;
		padding: 8px 10px;
		color: var(--foreground);
		font: inherit;
		text-align: left;
		background: var(--card);
		border: 1px solid var(--map-line);
		border-radius: 7px;
		cursor: pointer;
	}
	.turn-row:nth-child(even) {
		background: color-mix(in oklch, var(--card) 88%, var(--foreground));
	}
	.turn-row:hover {
		border-color: var(--map-line-strong);
	}
	.turn-row.chosen {
		background: var(--map-live-soft);
		border-color: var(--status-live);
		box-shadow: var(--shadow-xs);
	}
	.turn-row .ordinal {
		display: flex;
		align-items: center;
		gap: 6px;
		font-size: 13px;
		font-weight: 600;
	}
	.turn-row .state {
		color: var(--status-muted);
		font-size: 13px;
		text-transform: capitalize;
	}
	.turn-row .scope,
	.turn-row .duration,
	.turn-row .usage {
		overflow: hidden;
		color: var(--status-muted);
		font-family: var(--font-mono);
		font-size: 13px;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.turn-chip {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		padding: 6px 10px;
		color: var(--foreground);
		font: inherit;
		font-size: 13px;
		background: var(--card);
		border: 1px solid var(--map-line);
		border-radius: 999px;
		cursor: pointer;
	}
	.turn-chip.chosen {
		background: var(--map-live-soft);
		border-color: var(--status-live);
	}
	.session-details {
		padding: 10px;
		background: var(--card);
		border: 1px solid var(--map-line);
		border-radius: 7px;
	}
	.session-details summary {
		color: var(--foreground);
		font-size: 13px;
		font-weight: 600;
		cursor: pointer;
	}
	.session-details dl {
		display: grid;
		grid-template-columns: 88px minmax(0, 1fr);
		gap: 6px 10px;
		margin: 10px 0 12px;
		font-size: 13px;
	}
	.session-details dt {
		color: var(--status-muted);
	}
	.session-details dd {
		min-width: 0;
		margin: 0;
		overflow-wrap: anywhere;
	}
	.session-id {
		display: block;
		overflow: hidden;
		font-family: var(--font-mono);
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.scope-link {
		padding: 0;
		color: var(--status-live);
		font: inherit;
		font-size: 13px;
		text-align: left;
		background: transparent;
		border: 0;
		cursor: pointer;
	}
	.scope-link:hover {
		text-decoration: underline;
	}

	@media (min-width: 900px) {
		.session-details > summary {
			display: none;
		}
		.session-details:not([open]) > :not(summary) {
			display: block;
		}
	}

	@media (max-width: 900px) {
		.session-page {
			flex-direction: column;
			overflow-y: auto;
		}
		.rail {
			width: auto;
			flex-shrink: 0;
			border-right: none;
			border-bottom: 1px solid var(--map-line);
		}
		.turn-list.full {
			display: none;
		}
		.turn-list.compact {
			display: flex;
			flex-wrap: wrap;
			gap: 6px;
		}
		.detail-area {
			min-height: 480px;
		}
	}
</style>
