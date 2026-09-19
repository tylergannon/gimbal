<script lang="ts">
	import Pip, { type RowStatus } from "./Pip.svelte";
	import {
		mapIcon,
		mapWording,
		rowStatus,
		stepName,
		type Step,
		type StepRow,
	} from "./step.js";

	type Props = {
		/** The operation the workflow's `Graph` wrote: one agent call, one
		 * command, one interview. It carries the node's icon and its name. */
		step: Step;
		/** The row the run wrote for this step, absent while the run has not
		 * reached it. It carries the node's status. */
		row?: StepRow;
		/** The state to draw when the caller states it rather than handing a
		 * row: one word of the run's own status vocabulary. A gallery uses it
		 * to draw a state no fixture row holds; the map hands rows. */
		status?: RowStatus;
		/** The one line of run under the name: "turn 3", "planner",
		 * "question 2 · 40 s", "not started". Prompts, values and timings are
		 * the detail pane's business, never the map's. */
		meta?: string;
		/** Selected: the one step the detail pane is pointed at. */
		selected?: boolean;
		/** The small variant, for the steps a scope runs in passing. */
		small?: boolean;
		/** Told when the node is clicked. The node holds no selection of its
		 * own; whoever draws the map decides what a selection means. */
		onselect?: () => void;
	};

	const {
		step,
		row,
		status,
		meta = "",
		selected = false,
		small = false,
		onselect,
	}: Props = $props();

	const Icon = $derived(mapIcon[step.kind]);
	const name = $derived(stepName(step));
	// A stated state wins, because only a caller with no row states one.
	const state = $derived(status === undefined ? rowStatus(row) : { status, error: "" });

	// A command is named the way it is typed, so it reads in the monospace
	// face wherever it appears; the small variant is monospace throughout,
	// as `Main.html` and `Collapse.html` draw it.
	const mono = $derived(small || step.kind === "command");
</script>

<button
	type="button"
	class="nd"
	class:sel={selected}
	class:sm={small}
	aria-pressed={selected}
	aria-label="{mapWording[step.kind]} {name}"
	onclick={() => onselect?.()}
>
	<span class="ico"><Icon size={14} aria-hidden="true" /></span>
	<span class="nm" class:mono>{name}</span>
	{#if meta !== ""}<span class="meta">{meta}</span>{/if}
	<span class="sp"></span>
	<Pip status={state.status} error={state.error} />
</button>

<style>
	.nd {
		box-sizing: border-box;
		width: 100%;
		height: 44px;
		display: flex;
		align-items: center;
		gap: 10px;
		padding: 0 12px 0 8px;
		margin: 0;
		background: var(--paper-2);
		color: var(--card-foreground);
		border: 1px solid var(--line);
		border-radius: 8px;
		box-shadow: var(--shadow-node);
		white-space: nowrap;
		font: inherit;
		text-align: left;
		cursor: pointer;
	}

	.ico {
		width: 26px;
		height: 26px;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		background: var(--muted);
		border-radius: calc(var(--radius) - 4px);
		color: var(--foreground);
		flex-shrink: 0;
	}

	.nm {
		font-size: 15px;
		line-height: 20px;
		font-weight: 600;
		letter-spacing: -0.005em;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.meta {
		font-size: 13px;
		line-height: 18px;
		color: var(--ink-2);
	}

	/* The gap that holds the pip at the right edge, however long the name. */
	.sp {
		flex-grow: 1;
	}

	.sel {
		border-color: var(--live-ring);
		box-shadow:
			0 0 0 3px var(--live-soft),
			var(--shadow-node);
	}

	.sm {
		height: 36px;
		padding: 0 10px 0 8px;
		gap: 8px;
	}

	.sm .ico {
		width: 20px;
		height: 20px;
		background: transparent;
		color: var(--ink-2);
	}

	/* The small variant is compact in its height, its padding and its icon.
	   The name stays at the 15px claim 9 sets, only lighter; the specimen
	   shrinks it to 13.5px, and the map's floor for text wins. */
	.sm .nm {
		font-weight: 500;
	}

	.mono {
		font-family: var(--font-mono, ui-monospace, SFMono-Regular, Menlo, Monaco, monospace);
	}

	.nd:focus-visible {
		outline: 2px solid var(--live);
		outline-offset: 2px;
	}
</style>
