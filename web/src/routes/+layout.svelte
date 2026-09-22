<script lang="ts">
	import '../app.css';
	import { navigating, page } from '$app/state';
	import favicon from '#lib/assets/favicon.svg';
	import RunsLoading from '#lib/run/RunsLoading.svelte';

	let { children } = $props();
	const project = $derived(page.params.project);
	const root = $derived(project ? `/projects/${project}` : '/');
	const loadingRuns = $derived(navigating.to?.url.pathname === root && navigating.from?.url.pathname !== root);
</script>

<svelte:head><link rel="icon" href={favicon} /></svelte:head>
<nav class="site-nav"><a href="/">Projects</a>{#if project}<a href={`${root}/conversations`}>Conversations</a><a href={root}>Runs</a><a href={`${root}/about`}>About</a>{/if}</nav>
<main>
	{#if loadingRuns}
		<div class="runs-page"><RunsLoading /></div>
	{:else}
		{@render children()}
	{/if}
</main>

<style>
	:global(body) {
		margin: 0;
		font: 16px/1.5 var(--font-sans);
		color: var(--foreground);
		background: var(--background);
	}

	nav {
		display: flex;
		gap: 1.25rem;
		padding: 0.9rem 2rem;
		border-bottom: 1px solid var(--map-line);
		background: var(--card);
	}

	nav a {
		color: var(--status-live);
		text-decoration: none;
	}

	nav a:hover {
		color: var(--foreground);
	}

	main {
		min-width: 0;
	}

	.runs-page {
		display: flex;
		min-width: 0;
		justify-content: center;
		padding: 0 32px;
	}
</style>
