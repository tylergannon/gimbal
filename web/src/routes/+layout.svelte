<script lang="ts">
	import '../app.css';

	let { children } = $props();
</script>

<nav>
	<a href="/">Guide</a>
	<a href="/about">About</a>
</nav>

<main>
	{@render children()}
</main>

<style>
	:global(body) {
		margin: 0;
		font: 16px/1.5 ui-sans-serif, system-ui, sans-serif;
		color: #14181f;
		background: #fbfbfd;
	}

	nav {
		display: flex;
		gap: 1.25rem;
		padding: 0.9rem 2rem;
		border-bottom: 1px solid #e3e6ec;
		background: #fff;
	}

	nav a {
		color: #2b5fd9;
		text-decoration: none;
	}

	main {
		max-width: 44rem;
		margin: 0 auto;
		padding: 2.5rem 2rem;
	}
</style>
