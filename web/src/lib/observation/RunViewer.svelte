<script lang="ts">
	import { RunObservation, usageOf, usageText, type Decision, type ObservationDelta, type RunSnapshot } from './index.js';
	import SessionTimeline from './SessionTimeline.svelte';
	import SteerBox from './SteerBox.svelte';

	let { snapshot }: { snapshot: RunSnapshot } = $props();
	const observation = $derived(new RunObservation(snapshot));
	let revision = $state(0);
	let connectionState = $state<'connecting' | 'live' | 'disconnected'>('connecting');
	const currentRun = $derived.by(() => { revision; return { ...observation.run }; });
	// The run's total is the root scope's, which covers every scope under it.
	// Go recomputed it and pushed it; nothing here adds anything up.
	const runTotal = $derived.by(() => { revision; return usageText(observation.totals.scopes['']?.all ?? usageOf(undefined)); });
	const scopes = $derived.by(() => { revision; return Object.entries(observation.scopes).sort(([a], [b]) => a.localeCompare(b)); });
	const taskName = (task: unknown) =>
		typeof task === 'object' && task !== null && 'name' in task ? String((task as { name: unknown }).name) : JSON.stringify(task);
	// A decision's body is the planner's own event, so the task it dispatched
	// is read out of it; a decision with none ended the dispatch.
	const decisionName = (decision: Decision) => {
		const body = decision.body;
		const task = typeof body === 'object' && body !== null ? (body as { task?: unknown }).task : undefined;
		return task === undefined ? 'ended dispatch' : taskName(task);
	};
	// One section per turn, oldest first. A turn is where the work happened,
	// and its session says which agent ran it.
	const views = $derived.by(() => {
		revision;
		return Object.values(observation.turns)
			.sort((a, b) => a.started - b.started || a.id.localeCompare(b.id))
			.map((turn) => ({ turn, session: observation.sessions[turn.session], state: observation.state(turn.id) }));
	});

	$effect(() => {
		const generation = observation.beginConnection();
		let stream: EventSource | undefined;
		let retry: ReturnType<typeof setTimeout> | undefined;
		const listener = (message: MessageEvent<string>) => {
			if (!observation.isCurrentConnection(generation)) return;
			const delta = JSON.parse(message.data) as ObservationDelta;
			const accepted = observation.applyDelta(delta, generation);
			if (accepted) revision = observation.revision;
		};
		const snapshotListener = (message: MessageEvent<string>) => {
			if (observation.replace(JSON.parse(message.data) as RunSnapshot, generation)) revision = observation.revision;
		};
		const connect = () => {
			if (!observation.isCurrentConnection(generation)) return;
			const query = new URLSearchParams({ stream: observation.stream, position: String(observation.position) });
			stream = new EventSource(`/api/runs/${encodeURIComponent(observation.run.id)}/events?${query}`);
			stream.addEventListener('delta', listener as EventListener);
			stream.addEventListener('snapshot', snapshotListener as EventListener);
			stream.onopen = () => { if (observation.isCurrentConnection(generation)) connectionState = 'live'; };
			stream.onerror = () => {
				if (!observation.isCurrentConnection(generation)) return;
				connectionState = 'disconnected';
				stream?.close();
				if (observation.run.status !== 'running') return;
				retry = setTimeout(connect, 250);
			};
		};
		connect();
		return () => {
			observation.endConnection(generation);
			if (retry !== undefined) clearTimeout(retry);
			stream?.close();
		};
	});
</script>

<header class="run-header">
	<div><p class="eyebrow">Run</p><h1>{currentRun.name}</h1></div>
	<div class="states"><span class={`status ${currentRun.status}`}>{currentRun.status}</span><span>{connectionState}</span></div>
</header>
<p class="run-usage">Run usage · {runTotal}</p>
{#if currentRun.error}<p class="run-error">{currentRun.error}</p>{/if}

{#each scopes as [key, scope] (key)}
	<section class="scope">
		<header>
			<div><h2>{scope.name}</h2><p>{key}</p></div>
			<span class={`status ${scope.status}`}>{scope.status}</span>
		</header>
		{#if scope.error}<p class="run-error">{scope.error}</p>{/if}
		{#if scope.task !== undefined}<p>task · {taskName(scope.task)}</p>{/if}
		{#each scope.decisions ?? [] as decision (decision.seq)}<p>decision · {decisionName(decision)}</p>{/each}
		{#each Object.entries(scope.values ?? {}) as [name, value] (name)}<p>{name} · {JSON.stringify(value)}</p>{/each}
	</section>
{/each}

{#each views as view (view.turn.id)}
	{@const turn = view.turn}
	{@const session = view.session}
	{@const state = view.state}
	<section class="invocation">
		<header>
			<div><h2>{session?.name ?? turn.session}</h2><p>{session?.adapter ?? 'agent'} · {session?.model ?? 'model unavailable'}</p></div>
			<span>{turn.scope}</span>
		</header>
		{#if state}<SessionTimeline {state} {revision} {observation} turn={turn.id} />{/if}
		{#if currentRun.status === 'running' && turn.ended === 0}
			<SteerBox run={currentRun.id} session={turn.session} name={session?.name ?? turn.session} />
		{/if}
	</section>
{/each}
{#if views.length === 0}<p>No agent turns have started.</p>{/if}

<style>
	.run-header, .invocation > header, .scope > header, .states { display: flex; align-items: center; justify-content: space-between; gap: 1rem; }
	h1, h2, p { margin: 0; }
	.eyebrow { color: #697386; font-size: .75rem; text-transform: uppercase; letter-spacing: .08em; }
	.states { color: #697386; font-size: .8rem; }
	.status { border-radius: 999px; padding: .2rem .55rem; background: #e9edf4; }
	.status.running { background: #e3efff; color: #174d9b; }
	.status.completed { background: #e0f4e7; color: #176137; }
	.status.failed, .status.cancelled, .run-error { color: #a22525; }
	.run-error { margin-top: 1rem; }
	.run-usage { margin: .5rem 0 1rem; color: #556070; font-size: .8rem; }
	.invocation { margin-top: 2rem; display: grid; gap: 1rem; }
	.invocation > header p, .invocation > header span { color: #697386; font-size: .85rem; }
	.scope { margin-top: 1rem; display: grid; gap: .25rem; }
	.scope p { color: #697386; font-size: .85rem; overflow-wrap: anywhere; }
	.status.ended { background: #e9edf4; color: #3c4a5e; }
</style>
