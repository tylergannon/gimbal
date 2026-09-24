<script module lang="ts">
	// One agent session on its own: a compact Focus card, the same card's
	// prompt/context reverse, and the existing full turn detail below it.
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
	import PromptAndContext from './PromptAndContext.svelte';
	import SteerBox from './SteerBox.svelte';
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
	const liveTurn = $derived(runningTurn ?? turns.at(-1));
	// The reader can inspect any recorded turn and assistant message. This
	// selection never changes which turn receives steering or a stop request.
	let pickedTurnID = $state<string>();
	let pickedMomentID = $state<string>();
	let showingPrompt = $state(false);
	let historySessionID = $state("");
	$effect(() => {
		if (historySessionID === sessionID) return;
		historySessionID = sessionID;
		pickedTurnID = undefined;
		pickedMomentID = undefined;
		showingPrompt = false;
	});
	const chosenTurn = $derived(
		(pickedTurnID ? turns.find((row) => row.id === pickedTurnID) : undefined) ?? liveTurn
	);
	const turnMoments = $derived.by(() => {
		revision;
		if (!chosenTurn || !observation) return [];
		const state = observation.state(chosenTurn.id);
		return Object.values(state?.message ?? {}).flat().filter((message) => message.type === 'assistant');
	});
	function hasRecordedProse(message: (typeof turnMoments)[number]): boolean {
		return (message.content ?? []).some(
			(part: { type?: string; text?: unknown }) =>
				(part.type === 'text' || part.type === 'reasoning') &&
				typeof part.text === 'string' &&
				part.text.trim()
		);
	}
	function momentSummary(message: (typeof turnMoments)[number]): string {
		const content = message.content ?? [];
		const text = content
			.filter((part: { type?: string; text?: unknown }) =>
				(part.type === 'text' || part.type === 'reasoning') && typeof part.text === 'string'
			)
			.map((part: { text: string }) => part.text.trim().replace(/\s+/g, ' '))
			.find(Boolean);
		if (text) return `↳ ${text.length > 64 ? `${text.slice(0, 61)}…` : text}`;
		const tools = content.filter((part: { type?: string; function?: { name?: string }; name?: string }) => part.type === 'tool');
		const toolName = tools.map((tool: { function?: { name?: string }; name?: string }) => tool.function?.name ?? tool.name).find(Boolean);
		return toolName ? `${toolName} · recorded tool` : tools.length ? `${tools.length} recorded tool calls` : 'No recorded text';
	}
	const latestProseMoment = $derived([...turnMoments].reverse().find(hasRecordedProse));
	const latestMoment = $derived(turnMoments.at(-1));
	const chosenMoment = $derived(
		turnMoments.find((message) => message.id === pickedMomentID) ?? latestProseMoment ?? latestMoment
	);
	const orderedMoments = $derived(
		chosenMoment ? [chosenMoment, ...turnMoments.filter((message) => message.id !== chosenMoment.id).reverse()] : []
	);
	const recordedText = $derived.by(() => {
		const content = chosenMoment?.content ?? [];
		return content
			.filter((part: { type?: string; text?: unknown }) => part.type === 'text' && typeof part.text === 'string' && part.text.trim())
			.map((part: { text: string }) => part.text);
	});
	const recordedReasoning = $derived.by(() => {
		const content = chosenMoment?.content ?? [];
		return content
			.filter((part: { type?: string; text?: unknown }) => part.type === 'reasoning' && typeof part.text === 'string' && part.text.trim())
			.map((part: { text: string }) => part.text);
	});
	const activity = $derived.by(() => {
		const messages = chosenTurn && observation ? observation.state(chosenTurn.id)?.message ?? {} : {};
		const rows = Object.values(messages).flat();
		const tools = rows.flatMap((message) => message.content ?? []).filter((part: { type?: string }) => part.type === 'tool');
		return { messages: rows.length, tools: tools.length };
	});
	const latestToolActivity = $derived.by(() => {
		revision;
		const messages = liveTurn && observation ? observation.state(liveTurn.id)?.message ?? {} : {};
		const rows = Object.values(messages).flat();
		const newestAssistant = [...rows].reverse().find((message) => message.type === 'assistant');
		return (newestAssistant?.content ?? []).filter((part: { type?: string }) => part.type === 'tool').length;
	});
	const toolActivityLabel = $derived(liveTurn?.ended === 0 && live ? 'Current tool activity' : 'Latest tool activity');
	const readingHistory = $derived.by(() => {
		if (!chosenTurn || !liveTurn || chosenTurn.id !== liveTurn.id) return Boolean(chosenTurn && liveTurn);
		return Boolean(pickedMomentID && pickedMomentID !== latestMoment?.id);
	});
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
	function returnToLive() {
		pickedTurnID = undefined;
		pickedMomentID = undefined;
		showingPrompt = false;
	}
	function selectTurn(turn: TurnRow) {
		pickedTurnID = turn.id;
		pickedMomentID = undefined;
		showingPrompt = false;
	}
</script>

{#snippet sessionBlock()}
	{#if session}
		<dl class="session-facts">
			<dt>Session</dt><dd>{session.name}</dd>
			<dt>Model</dt><dd>{session.model}</dd>
			<dt>Adapter</dt><dd>{session.adapter}</dd>
			<dt>Session id</dt><dd><code class="session-id" title={session.id}>{session.id}</code></dd>
			<dt>Created in</dt>
			<dd><button type="button" class="scope-link" onclick={() => onscope?.(session.scope)}>{session.scope || 'root'}</button></dd>
		</dl>
		<UsageTable usage={totalUsage} title="Session total" />
	{/if}
{/snippet}

<div class="session-page">
	{#if !session}
		<div class="missing"><p>Session <code>{sessionID}</code> was not found in this run.</p></div>
	{:else}
		<div class="focus-column">
			<section class="focus-card" aria-label="Focus card">
				<header class="card-head">
					<div class="identity">
						<span class="eyebrow">Focus</span>
						<strong>{session.name}</strong>
						<span class="model">{session.model}</span>
					</div>
					<button type="button" class="flip-button" disabled={!chosenTurn} onclick={() => (showingPrompt = !showingPrompt)}>
						{showingPrompt ? '← Back to live' : 'Prompt & context ↻'}
					</button>
				</header>

				{#if showingPrompt && chosenTurn}
					<div class="card-back" role="region" aria-label="Prompt and context">
						<div class="back-title"><span>Turn {ordinalOf(chosenTurn)} input</span><span>Exact recorded input</span></div>
					<div class="prompt-scroll"><PromptAndContext turn={chosenTurn} {snapshot} {onscope} /></div>
				</div>
					{:else}
					<div class="card-front">
						<div class="prose-area" aria-live="polite">
						{#if recordedText.length || recordedReasoning.length}
							{#each recordedText as text, i (`text-${i}`)}<p class="assistant-text">{text}</p>{/each}
							{#each recordedReasoning as text, i (`reasoning-${i}`)}<details class="reasoning" open><summary>Recorded reasoning text</summary><p>{text}</p></details>{/each}
						{:else}
							<p class="no-prose">No assistant prose recorded for this turn yet.</p>
							{/if}
						</div>
						<div class="source-line">
							{#if latestToolActivity}
								<span>{toolActivityLabel}</span><span>{latestToolActivity} tool {latestToolActivity === 1 ? 'call' : 'calls'}</span>
							{:else}
								<span>Recorded activity</span><span>{activity.messages} messages · {activity.tools} tool calls</span>
							{/if}
						</div>
						<div class="turn-picker" aria-label="Recorded turns">
							{#each turns as turn (turn.id)}
							{const chosen = turn.id === chosenTurn?.id}
							<button type="button" class:chosen class="turn-chip" aria-current={chosen ? 'true' : undefined} onclick={() => selectTurn(turn)}>
								<Pip state={turnPip(turn)} /> Turn {ordinalOf(turn)} · {turnOutcome(turn)}
							</button>
							{/each}
						</div>
						{#if readingHistory}
							<button type="button" class="return-live" onclick={returnToLive}>Return to live</button>
						{/if}
					<div class="moment-history" aria-label="Recorded assistant history">
						<span class="history-label">History</span>
						<div class="moment-strip">
							{#each orderedMoments as message (message.id)}
								<button type="button" class:chosen={message.id === (chosenMoment?.id ?? '')} aria-label={momentSummary(message)} aria-pressed={message.id === (chosenMoment?.id ?? '')} title={momentSummary(message)} onclick={() => (pickedMomentID = message.id)}>
									<span>{momentSummary(message)}</span>
								</button>
							{/each}
						</div>
					</div>
					<div class="turn-meta">
						{#if chosenTurn}<span>Turn {ordinalOf(chosenTurn)} · {chosenTurn.scope || 'root'}</span><span>{turnDuration(chosenTurn)}</span>{/if}
					</div>
				</div>
			{/if}
			</section>

			<SteerBox
				onsteer={(message) => onsteer?.({ run: snapshot.run.id, session: sessionID, message }) ?? Promise.resolve({ ok: false, message: 'Steering is unavailable.' })}
				onstop={() => runningTurn && onstop ? onstop(runningTurn) : Promise.resolve({ ok: false, message: 'There is no running turn to stop.' })}
				ended={!runningTurn || !live}
			/>

			<details class="full-details">
				<summary>Full activity and turn details</summary>
				{#if chosenTurn}
					<SessionDetail
						{snapshot}
						{observation}
						{revision}
						turn={chosenTurn}
						{definition}
						{watchers}
						{live}
						showSteering={false}
						onsteer={() => Promise.resolve({ ok: false, message: 'Use the current-turn steer box above.' })}
						onscope={onscope}
					/>
				{:else}
					<p class="empty">This session has no recorded turns.</p>
				{/if}
			</details>

			<details class="session-details">
				<summary>Session details and usage</summary>
				{@render sessionBlock()}
			</details>
		</div>
	{/if}
</div>

<style>
	.session-page { display: flex; min-height: 0; flex: 1; min-width: 0; justify-content: center; overflow: auto; padding: clamp(12px, 3vw, 32px); background: var(--background); }
	.focus-column { display: flex; width: min(100%, 820px); min-width: 0; flex: 0 0 auto; flex-direction: column; align-self: flex-start; gap: 12px; margin: 0 auto; }
	.focus-card { flex: 0 0 auto; background: var(--card); border: 1px solid var(--map-line); border-radius: 12px; box-shadow: var(--shadow-xs); }
	.card-head { display: flex; min-width: 0; align-items: center; justify-content: space-between; gap: 12px; padding: 14px 18px; border-bottom: 1px solid var(--border); }
	.identity { display: flex; min-width: 0; align-items: baseline; flex-wrap: wrap; gap: 8px 12px; }
	.identity strong { font-size: 16px; }
	.eyebrow { color: var(--status-live); font-size: 11px; font-weight: 700; letter-spacing: .08em; text-transform: uppercase; }
	.model { color: var(--status-muted); font: 12px var(--font-mono); }
	.flip-button, .return-live { padding: 7px 10px; color: var(--foreground); font: inherit; font-size: 12px; background: var(--surface); border: 1px solid var(--map-line); border-radius: 7px; cursor: pointer; }
	.flip-button:hover, .return-live:hover { border-color: var(--status-live); }
	.card-front, .card-back { display: flex; flex-direction: column; gap: 12px; padding: 14px 18px 18px; }
	.turn-picker { display: flex; flex-wrap: wrap; gap: 6px; }
	.turn-chip { display: inline-flex; align-items: center; gap: 6px; padding: 5px 9px; color: var(--foreground); font: inherit; font-size: 12px; background: var(--background); border: 1px solid var(--map-line); border-radius: 999px; cursor: pointer; }
	.turn-chip.chosen { background: var(--map-live-soft); border-color: var(--status-live); }
	.return-live { align-self: flex-start; color: var(--status-live); }
	.moment-history { display: flex; min-width: 0; align-items: center; gap: 8px; padding-top: 2px; }
	.history-label { flex: 0 0 auto; color: var(--status-muted); font-size: 11px; }
	.moment-strip { display: flex; min-width: 0; flex: 1; gap: 5px; overflow-x: auto; padding-bottom: 2px; }
	.moment-strip button { display: flex; max-width: 220px; flex: 0 0 auto; flex-direction: column; gap: 1px; padding: 5px 8px; color: var(--status-muted); font: inherit; font-size: 11px; text-align: left; background: transparent; border: 1px solid var(--map-line); border-radius: 6px; cursor: pointer; }
	.moment-strip button span:first-child { color: inherit; font-weight: 600; }
	.moment-strip button span { max-width: 204px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
	.moment-strip button.chosen { color: var(--foreground); background: var(--map-live-soft); border-color: var(--status-live); }
	.prose-area { display: flex; flex-direction: column; gap: 10px; padding: 4px 0; }
	.assistant-text, .reasoning p { margin: 0; overflow-wrap: anywhere; font-size: 18px; line-height: 1.5; white-space: pre-wrap; }
	.reasoning { color: var(--status-muted); }
	.reasoning summary { margin-bottom: 6px; font-size: 12px; cursor: pointer; }
	.no-prose { margin: 0; color: var(--status-muted); font-size: 14px; }
	.source-line { display: flex; flex-wrap: wrap; gap: 4px 8px; color: var(--status-muted); font-size: 11px; }
	.turn-meta { display: flex; flex-wrap: wrap; align-items: baseline; gap: 6px 10px; color: var(--status-muted); font-size: 12px; }
	.turn-meta { justify-content: space-between; border-top: 1px solid var(--border); padding-top: 8px; font-family: var(--font-mono); }
	.back-title { display: flex; justify-content: space-between; gap: 8px; color: var(--foreground); font-size: 13px; font-weight: 600; }
	.back-title span + span { color: var(--status-muted); font-size: 11px; font-weight: 400; }
	.prompt-scroll { min-height: 0; max-height: min(65vh, 620px); overflow: auto; padding-right: 4px; }
	.full-details, .session-details { overflow: hidden; background: var(--card); border: 1px solid var(--map-line); border-radius: 9px; }
	.full-details > summary, .session-details > summary { padding: 11px 14px; color: var(--foreground); font-size: 13px; font-weight: 600; cursor: pointer; }
	.full-details[open] > summary, .session-details[open] > summary { border-bottom: 1px solid var(--border); }
	.full-details :global(.session-detail), .full-details :global(.status-line), .full-details :global(.tab-bar) { min-width: 0; }
	.full-details :global(.panel-scroller) { max-height: 55vh; }
	.session-facts { display: grid; grid-template-columns: 90px minmax(0, 1fr); gap: 6px 10px; margin: 12px 14px; font-size: 12px; }
	.session-facts dt { color: var(--status-muted); }
	.session-facts dd { min-width: 0; margin: 0; overflow-wrap: anywhere; }
	.session-id { display: block; overflow: hidden; font-family: var(--font-mono); text-overflow: ellipsis; white-space: nowrap; }
	.scope-link { padding: 0; color: var(--status-live); font: inherit; text-align: left; background: transparent; border: 0; cursor: pointer; }
	.missing { display: flex; flex: 1; align-items: center; justify-content: center; padding: 24px; }
	.missing p, .empty { margin: 0; color: var(--status-muted); font-size: 13px; }
	@media (max-width: 600px) {
		.session-page { align-items: stretch; padding: 8px; }
		.focus-column { width: 100%; gap: 8px; }
		.card-head { align-items: flex-start; padding: 12px; }
		.card-front, .card-back { padding: 12px; }
		.identity { gap: 4px 8px; }
		.identity strong { font-size: 14px; }
		.assistant-text, .reasoning p { font-size: 17px; }
		.flip-button { flex: 0 0 auto; }
		.full-details :global(.panel-scroller) { max-height: none; }
	}
</style>
