<script lang="ts">
	// The transcript for one turn: quieter than SessionTimeline (no raw
	// session-id heading, tool-only messages skip their card chrome via
	// MessageRow's `quiet` prop), windowed to the last 50 messages with a
	// "Load earlier" control, and pinned to the tail while the reader is near
	// the bottom.
	import type { ProjectionState } from '../../sessionstate/index.js';
	import type { RunObservation } from '../../observation/index.js';
	import MessageRow from '../MessageRow.svelte';

	// Destructured under a local alias: a binding literally named `state`
	// would collide with the `$state` rune used below.
	let {
		state: transcriptState,
		revision,
		observation,
		turn
	}: { state: Readonly<ProjectionState>; revision: number; observation: RunObservation; turn: string } =
		$props();

	const WINDOW = 50;
	const NEAR_BOTTOM_PX = 48;

	const sessions = $derived.by(() => {
		revision;
		return Object.keys(transcriptState.message).map((sessionID) => ({
			sessionID,
			messages: transcriptState.message[sessionID] ?? [],
			pending: new Map((transcriptState.pending[sessionID] ?? []).map((input) => [input.id, input]))
		}));
	});
	const allMessages = $derived.by(() =>
		sessions.flatMap((session) => session.messages.map((message) => ({ session, message })))
	);

	let visibleCount = $state(WINDOW);
	$effect(() => {
		turn;
		visibleCount = WINDOW;
	});
	const hiddenCount = $derived(Math.max(0, allMessages.length - visibleCount));
	const visibleMessages = $derived(allMessages.slice(Math.max(0, allMessages.length - visibleCount)));

	function loadEarlier() {
		visibleCount = Math.min(allMessages.length, visibleCount + WINDOW);
	}

	let scroller: HTMLDivElement | undefined = $state();
	let pinned = $state(true);
	let newSinceUnpin = $state(0);
	let previousTotal = 0;

	function isNearBottom(el: HTMLDivElement) {
		return el.scrollHeight - el.scrollTop - el.clientHeight <= NEAR_BOTTOM_PX;
	}
	function onScroll() {
		if (!scroller) return;
		if (isNearBottom(scroller)) {
			pinned = true;
			newSinceUnpin = 0;
		} else {
			pinned = false;
		}
	}
	function jumpToLatest() {
		if (!scroller) return;
		scroller.scrollTop = scroller.scrollHeight;
		pinned = true;
		newSinceUnpin = 0;
	}

	$effect(() => {
		const total = allMessages.length;
		if (pinned) {
			previousTotal = total;
			const target = scroller;
			if (target) requestAnimationFrame(() => (target.scrollTop = target.scrollHeight));
		} else if (total > previousTotal) {
			newSinceUnpin += total - previousTotal;
			previousTotal = total;
		} else {
			previousTotal = total;
		}
	});
</script>

<div class="activity">
	{#if hiddenCount > 0}
		<button type="button" class="load-earlier" onclick={loadEarlier}>
			Load earlier ({hiddenCount})
		</button>
	{/if}
	<div class="scroller" bind:this={scroller} onscroll={onScroll}>
		{#each visibleMessages as { session, message } (message.id)}
			<MessageRow
				{message}
				pending={session.pending.get(message.id)}
				revision={revision + observation.messageRevision(turn, message.id)}
				quiet
			/>
		{/each}
		{#if visibleMessages.length === 0}
			<p class="empty">Waiting for transcript events…</p>
		{/if}
	</div>
	{#if !pinned}
		<button type="button" class="jump-pill" onclick={jumpToLatest}>
			Jump to latest{newSinceUnpin > 0 ? ` (${newSinceUnpin} new)` : ''}
		</button>
	{/if}
</div>

<style>
	.activity {
		position: relative;
		display: flex;
		min-width: 0;
		min-height: 0;
		flex: 1;
		flex-direction: column;
	}
	.load-earlier {
		align-self: center;
		margin: 8px 0;
		padding: 5px 10px;
		color: var(--status-live);
		font: inherit;
		font-size: 13px;
		background: var(--surface);
		border: 1px solid var(--map-line);
		border-radius: 999px;
		cursor: pointer;
	}
	.load-earlier:hover {
		color: var(--foreground);
		border-color: var(--map-line-strong);
	}
	.scroller {
		display: flex;
		min-width: 0;
		min-height: 0;
		flex: 1;
		flex-direction: column;
		gap: 8px;
		overflow-x: hidden;
		overflow-y: auto;
		padding: 4px 2px 8px;
	}
	.empty {
		margin: 0;
		color: var(--status-muted);
		font-size: 13px;
	}
	.jump-pill {
		position: absolute;
		bottom: 12px;
		left: 50%;
		z-index: 1;
		padding: 6px 12px;
		color: var(--primary-foreground);
		font: inherit;
		font-size: 13px;
		font-weight: 500;
		white-space: nowrap;
		background: var(--primary);
		border: none;
		border-radius: 999px;
		box-shadow: var(--map-shadow-path);
		cursor: pointer;
		transform: translateX(-50%);
	}
</style>
