<svelte:head>
	<title>About Gimble</title>
</svelte:head>

<article>
	<h1 data-testid="title">About Gimble</h1>
	<p class="lead">
		Gimble is a small Go runtime for programs that ask agents to do work. It makes the
		workflow visible in the code and keeps a record of each run.
	</p>

	<section aria-labelledby="why">
		<h2 id="why">Why it exists</h2>
		<p>
			Agent workflows still need ordinary program structure: a lifetime for each piece of work,
			clear ownership of sessions and goroutines, cancellation that reaches the agent, and a way
			to see what happened afterward. Gimble supplies those basics without taking over the rest
			of the program.
		</p>
	</section>

	<section aria-labelledby="terms">
		<h2 id="terms">Four terms</h2>
		<dl>
			<dt>Run</dt><dd>One execution of a workflow and its record on disk.</dd>
			<dt>Scope</dt><dd>A named piece of a run. It ends when its function returns.</dd>
			<dt>Session</dt><dd>One agent conversation in one working directory.</dd>
			<dt>Group</dt><dd>A named set of concurrent child scopes. <code>Wait</code> joins them.</dd>
		</dl>
	</section>

	<section aria-labelledby="attractor">
		<h2 id="attractor">Not an Attractor implementation</h2>
		<p>
			Gimble is a reaction to the work around Attractor, not an implementation of it. It does not
			adopt Attractor as its architecture, product plan, or vocabulary. The useful lesson kept
			here is simple: keep the workflow in code a person can read, and make the agent work
			observable.
		</p>
	</section>

	<section aria-labelledby="status">
		<h2 id="status">Current boundary</h2>
		<p>
			The current product is the Go library and its run logs. This website is its short guide.
			A browser view for inspecting live and past runs is planned work, not something this page
			claims to provide today.
		</p>
		<p>
			For the complete programming contract, use <code>go doc -all github.com/tylergannon/gimble</code>.
		</p>
	</section>
</article>

<style>
	article { max-width: 48rem; }
	h1 { margin-top: 0; }
	.lead { font-size: 1.2rem; }
	section { margin-top: 2.5rem; }
	dt { margin-top: 1rem; font-weight: 700; }
	dd { margin: 0.2rem 0 0; }
</style>
