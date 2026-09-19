<script lang="ts">
	import '../app.css';
	import favicon from '#lib/assets/favicon.svg';

	let { children } = $props();
</script>

<svelte:head><link rel="icon" href={favicon} /></svelte:head>
<nav class="site-nav"><a href="/">Runs</a><a href="/about">About</a></nav>
<main>{@render children()}</main>

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
</style>
