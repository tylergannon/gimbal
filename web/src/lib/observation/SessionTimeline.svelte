<script lang="ts">
	import type { JSONObject, ProjectionState } from '../sessionstate/index.js';
	import type { RunObservation } from './index.js';
	import MessageRow from './MessageRow.svelte';
	import { usageOf, usageText } from './index.js';

	let { state, revision, observation, turn }: { state: Readonly<ProjectionState>; revision: number; observation: RunObservation; turn: string } = $props();
	const text = (value: unknown) => typeof value === 'string' ? value : JSON.stringify(value, null, 2);
	const sessions = $derived.by(() => {
		revision;
		return [...new Set([...Object.keys(state.message), ...Object.keys(state.pending), ...Object.keys(state.active), ...Object.keys(state.permission)])].map((sessionID) => {
			const messages = state.message[sessionID] ?? [];
			const pending = new Map((state.pending[sessionID] ?? []).map((input) => [input.id, input]));
			return { sessionID, messages, permissions: state.permission[sessionID] ?? [], pending, unmatchedPending: [...pending.values()].filter((input) => !messages.some((message) => message.id === input.id)) };
		});
	});
	// The session's total is the roll-up Go pushed for the session this turn
	// belongs to. A session with nothing recorded yet zero-fills.
	const total = $derived.by(() => {
		revision;
		const session = observation.turns[turn]?.session ?? '';
		return observation.totals.sessions[session]?.all ?? usageOf(undefined);
	});
</script>

<div class="timeline">
	{#each sessions as session (session.sessionID)}
		<section class="native-session" data-session-id={session.sessionID}>
			<header class="session-header"><code>{session.sessionID}</code><span>{state.active[session.sessionID] ?? 'idle'}</span></header>
			<p class="session-total">{usageText(total)}</p>
			{#each session.messages as message (message.id)}
				<MessageRow {message} pending={session.pending.get(message.id)} revision={revision + observation.messageRevision(turn, message.id)} />
			{/each}
			{#each session.permissions as permission (permission.id)}
				<article class="permission" data-permission-id={permission.id}>
					<header><strong>Approval request</strong><span>{permission.reply ?? 'pending'}</span></header>
					<p class="prose">{permission.permission}</p>
					{#if permission.metadata !== undefined}<pre>{text(permission.metadata)}</pre>{/if}
				</article>
			{/each}
		{#each session.unmatchedPending as input (input.id)}
			<article data-message-id={input.id}>
				<header><strong>{input.type}</strong><span>{input.delivery ?? 'pending'}</span></header>
				{#if input.payload?.text !== undefined}<p class="prose">{input.payload.text}</p>{:else}<pre>{text(input.payload ?? input)}</pre>{/if}
			</article>
		{/each}
		{#if session.messages.length === 0 && session.permissions.length === 0 && session.unmatchedPending.length === 0}<p class="empty">Waiting for transcript events…</p>{/if}
		</section>
	{/each}
	{#if sessions.length === 0}<p class="empty">Waiting for transcript events…</p>{/if}
</div>

<style>
	.timeline { display: grid; gap: .8rem; }
	.native-session { display: grid; gap: .8rem; }
	.session-header code { text-transform: none; }
	article { border: 1px solid #dfe3ea; border-radius: .65rem; padding: .85rem 1rem; background: #fff; }
	.permission { border-left: 3px solid #b7791f; }
	header { display: flex; justify-content: space-between; gap: 1rem; color: #556070; font-size: .82rem; text-transform: capitalize; }
	.prose { white-space: pre-wrap; margin: .65rem 0; }
	pre { overflow-x: auto; white-space: pre-wrap; font: .82rem/1.45 ui-monospace, monospace; }
	.empty { color: #697386; }
	.session-total { color: #556070; font-size: .78rem; }
</style>
