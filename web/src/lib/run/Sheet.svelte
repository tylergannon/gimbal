<script module lang="ts">
	import type { ScopeStep, Step, StepRow } from "./step.js";
	import type { RowStatus } from "./Pip.svelte";
	import type { ScopeRow } from "../observation/index.js";

	/** One step of a folded scope: the operation the graph wrote and the row
	 * the run wrote for it, which is the status its glyph carries. A nested
	 * scope folds to one glyph too. */
	export type SheetStep = {
		step: Step | ScopeStep;
		row?: StepRow;
		/** The state to draw when the caller states it rather than handing a
		 * row, in the run's own status vocabulary. `Node` takes the same. */
		status?: RowStatus;
	};

	/** One pass of a repeated scope. Its row's `key` is the runtime scope key,
	 * which is what the sheet reports when a reader picks it. */
	export type SheetInstance = {
		row: ScopeRow;
		steps?: SheetStep[];
		elapsed?: string;
	};

	/** What a sheet hands its children: the tint they take, and the fold they
	 * share. Opening one child folds the rest, and the depth alternates the
	 * two paper tints so adjacent levels always differ. */
	type Nest = {
		depth: number;
		join(fold: () => void): () => void;
		opened(fold: () => void): void;
	};

	const nest = Symbol("sheet nesting");
</script>

<script lang="ts">
	import BracesIcon from "@lucide/svelte/icons/braces";
	import ChevronDownIcon from "@lucide/svelte/icons/chevron-down";
	import ChevronRightIcon from "@lucide/svelte/icons/chevron-right";
	import { getContext, onDestroy, setContext, type Snippet } from "svelte";
	import * as Select from "#lib/components/ui/select/index.js";
	import Pip, { shapeOf, wording } from "./Pip.svelte";
	import { mapIcon, mapWording, rowStatus, stepName } from "./step.js";

	type Props = {
		/** Which kind of scope this is and what the source named it, which is
		 * the icon and the text of the label. */
		scope: ScopeStep;
		/** The row the run wrote for this scope, absent before the run enters
		 * it. It carries the scope's status and the keys written so far. */
		row?: ScopeRow;
		/** The canvas: the run's root scope, which is not drawn as a sheet and
		 * never folds. It starts the alternation its children continue. */
		root?: boolean;
		/** On the path from the canvas to the selection, which is the only
		 * thing a shadow means. */
		path?: boolean;
		/** The selected scope: raised, with a dark border. */
		selected?: boolean;
		/** How many keys this scope's body writes, which the graph states.
		 * How many are written is the row's business. No chip when the scope
		 * writes nothing. */
		writes?: number;
		/** Every pass of a repeated scope, oldest first. Given these, the
		 * label carries a select and the sheet shows the latest pass. */
		instances?: SheetInstance[];
		/** The pass being shown, as its scope key. Bindable: it starts at the
		 * latest pass, the select writes it, and whoever draws the map can
		 * follow it or set it. */
		instance?: string;
		/** Told the scope key whenever a reader picks a different pass. */
		oninstance?: (key: string) => void;
		/** The steps a fold shows, in source order. A repeated scope's
		 * instance carries its own. */
		steps?: SheetStep[];
		/** How long the scope has been going, already worded. */
		elapsed?: string;
		/** Whether this sheet can fold at all. */
		foldable?: boolean;
		/** Folded now. Bindable, because opening a sheet folds its siblings. */
		folded?: boolean;
		/** The sheet's contents when it is open: its steps and its children.
		 * It is handed the steps of the pass the sheet is showing, so a body
		 * cannot drift from the label above it. */
		children?: Snippet<[SheetStep[]]>;
	};

	let {
		scope,
		row,
		root = false,
		path = false,
		selected = false,
		writes = 0,
		instances = [],
		instance = $bindable(),
		oninstance,
		steps = [],
		elapsed = "",
		foldable = true,
		folded = $bindable(false),
		children,
	}: Props = $props();

	// The canvas is not a level, so the sheets directly inside it take the
	// first tint and their children the second.
	const parent = getContext<Nest | undefined>(nest);
	const depth = $derived(root ? -1 : (parent?.depth ?? 0));

	const members = new Set<() => void>();
	setContext<Nest>(nest, {
		get depth() {
			return depth + 1;
		},
		join(fold) {
			members.add(fold);
			return () => members.delete(fold);
		},
		opened(fold) {
			for (const other of members) if (other !== fold) other();
		},
	});

	const canFold = $derived(!root && foldable);
	const fold = () => {
		if (canFold) folded = true;
	};

	// Every sheet joins its parent's fold, and the ones that cannot fold
	// simply ignore the call.
	if (parent) onDestroy(parent.join(fold));

	const toggle = () => {
		if (!canFold) return;
		folded = !folded;
		// Opening a sheet folds its siblings, so one pass is legible at a time.
		if (!folded) parent?.opened(fold);
	};

	// A repeated scope defaults to its latest pass, and shows whichever one
	// the reader picked after that.
	const current = $derived(
		instances.length === 0
			? undefined
			: (instances.find((one) => one.row.key === instance) ??
					instances[instances.length - 1]),
	);

	const ordinal = $derived(current ? instances.indexOf(current) + 1 : 0);
	const shown = $derived(rowStatus(current ? current.row : row));
	const shownSteps = $derived(current?.steps ?? steps);
	const shownElapsed = $derived(current?.elapsed ?? elapsed);
	const written = $derived(Object.keys((current ? current.row : row)?.values ?? {}).length);

	const shut = $derived(folded && canFold);
	const Icon = $derived(mapIcon[scope.kind]);
	const label = $derived(
		["slabel", depth % 2 === 1 ? "l2" : "", path ? "path" : "", selected ? "sel" : ""]
			.filter((part) => part !== "")
			.join(" "),
	);

	const pick = (key: string) => {
		instance = key;
		oninstance?.(key);
	};

	/** What a glyph or a node draws for one step: the row it has, or the
	 * state the caller stated for it. */
	const stepState = (one: SheetStep) =>
		one.status === undefined ? rowStatus(one.row) : { status: one.status, error: "" };
</script>

<div
	class="sheet"
	class:canvas={root}
	class:l2={depth % 2 === 1}
	class:path
	class:sel={selected}
	class:folded={shut}
>
	{#if !root}
		<div class="lbl">
			{#if instances.length > 0}
				<!-- The label of a repeated scope is its select, so its fold is a
				     control of its own beside it. -->
				{#if canFold}
					<button
						type="button"
						class="fb"
						aria-expanded={!shut}
						aria-label="{shut ? 'Open' : 'Fold'} {scope.name} {ordinal} of {instances.length}"
						onclick={toggle}
					>
						{#if shut}
							<ChevronRightIcon size={12} aria-hidden="true" />
						{:else}
							<ChevronDownIcon size={12} aria-hidden="true" />
						{/if}
					</button>
				{/if}
				<Select.Root type="single" value={current?.row.key ?? ""} onValueChange={pick}>
					<Select.Trigger
						class="{label} select"
						aria-label="{scope.name}, pass {ordinal} of {instances.length}"
					>
						<Pip status={shown.status} error={shown.error} />
						<span>{scope.name} {ordinal} of {instances.length}</span>
					</Select.Trigger>
					<Select.Content align="start">
						{#each instances as one, index (one.row.key)}
							{@const state = rowStatus(one.row)}
							<Select.Item value={one.row.key} label="{scope.name} {index + 1}">
								<Pip status={state.status} error={state.error} size={8} />
								<span>{scope.name} {index + 1}</span>
							</Select.Item>
						{/each}
					</Select.Content>
				</Select.Root>
			{:else}
				<button
					type="button"
					class={label}
					aria-expanded={canFold ? !shut : undefined}
					disabled={!canFold}
					onclick={toggle}
				>
					<!-- Folded, the label carries the chevron that opens it, as the
					     specimens draw it; open, it is the label itself that folds. -->
					{#if shut}
						<span class="fi"><ChevronRightIcon size={12} aria-hidden="true" /></span>
					{/if}
					<Icon size={13} aria-hidden="true" />
					<span>{scope.name}</span>
				</button>
			{/if}
		</div>

		<!-- Folded, a sheet keeps its label and its glyphs and nothing else:
		     context waits until it is opened. -->
		{#if writes > 0 && !shut}
			<div class="chip">
				<BracesIcon size={12} aria-hidden="true" />
				context<span class="muted">{written} of {writes}</span>
			</div>
		{/if}
	{/if}

	{#if shut}
		<button type="button" class="foldrow" aria-expanded="false" onclick={toggle}>
			<span class="fold">
				{#each shownSteps as one}
					{@const StepIcon = mapIcon[one.step.kind]}
					{@const state = stepState(one)}
					<span class="g" title="{mapWording[one.step.kind]} {stepName(one.step)}">
						<StepIcon size={13} aria-hidden="true" />
						<Pip status={state.status} error={state.error} size={8} />
					</span>
				{/each}
			</span>
			<span class="sp"></span>
			<span class="cap"
				>{wording[shapeOf(shown.status, shown.error)]}{shownElapsed === ""
					? ""
					: ` · ${shownElapsed}`}</span
			>
		</button>
	{:else}
		<div class="body">{@render children?.(shownSteps)}</div>
	{/if}
</div>

<style>
	/* Every rule on the map is `--line`, not the specimens' `--line-soft`:
	   soft is 1.8:1 on light paper and 2.1:1 on dark, and claim 18 puts the
	   floor for a border at 3:1. The hierarchy the specimens draw survives it,
	   because the selected sheet is the only one ruled in `--line-strong`. */
	.sheet {
		position: relative;
		box-sizing: border-box;
		border: 1px solid var(--line);
		border-radius: 10px;
		background: var(--paper-1);
		/* Off the path, flat. A shadow on the map means one thing, so a sheet
		   that is not on the way to the selection casts none. */
		box-shadow: none;
		/* Nested sheets inset by 8px, and the label straddling the top border
		   needs the room above the first step. */
		padding: 32px 8px 8px;
	}

	.l2 {
		background: var(--paper-2);
	}

	/* Shadow means one thing: the path to the selection. */
	.path {
		border-color: var(--line);
		box-shadow: var(--shadow-sheet-path);
	}

	.sel {
		border: 1.5px solid var(--line-strong);
		box-shadow: var(--shadow-sheet-selected);
	}

	/* The root is the canvas: no sheet of its own, and no fold. */
	.canvas {
		border: none;
		background: none;
		box-shadow: none;
		padding: 0;
	}

	.body {
		display: flex;
		flex-direction: column;
		gap: 12px;
	}

	/* A fold is the label, one row of glyphs, and nothing else. */
	.folded {
		padding-bottom: 8px;
	}

	/* The label straddles the top border, with the fold of a repeated scope
	   beside it. */
	.lbl {
		position: absolute;
		top: -12px;
		left: 12px;
		display: flex;
		align-items: center;
		gap: 6px;
	}

	/* The label is a child element, so the select's trigger wears the same
	   class the plain label does. */
	.sheet :global(.slabel) {
		position: static;
		box-sizing: border-box;
		display: inline-flex;
		align-items: center;
		gap: 7px;
		height: 24px;
		width: auto;
		min-width: 0;
		padding: 0 10px;
		margin: 0;
		font: inherit;
		font-size: 13px;
		line-height: 18px;
		font-weight: 600;
		letter-spacing: 0.01em;
		color: var(--foreground);
		background: var(--paper-1);
		border: 1px solid var(--line);
		border-radius: 7px;
		white-space: nowrap;
		cursor: pointer;
	}

	.sheet :global(.slabel:disabled) {
		cursor: default;
		opacity: 1;
	}

	.sheet :global(.slabel.l2) {
		background: var(--paper-2);
	}

	.sheet :global(.slabel.path) {
		border-color: var(--line);
	}

	.sheet :global(.slabel.sel) {
		border: 1.5px solid var(--line-strong);
	}

	.sheet :global(.slabel.select) {
		padding-right: 7px;
	}

	.sheet :global(.slabel.select > svg:last-child) {
		width: 14px;
		height: 14px;
		color: var(--ink-2);
	}

	.sheet :global(.slabel:focus-visible) {
		outline: 2px solid var(--live);
		outline-offset: 2px;
	}

	.fi {
		display: inline-flex;
		color: var(--ink-2);
	}

	/* A repeated scope's own fold: the label chip, square. */
	.fb {
		box-sizing: border-box;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 24px;
		height: 24px;
		padding: 0;
		margin: 0;
		color: var(--ink-2);
		background: var(--paper-1);
		border: 1px solid var(--line);
		border-radius: 7px;
		cursor: pointer;
	}

	.l2 > .lbl > .fb {
		background: var(--paper-2);
	}

	.fb:focus-visible {
		outline: 2px solid var(--live);
		outline-offset: 2px;
	}

	/* The context chip straddles the top border at the right. */
	.chip {
		position: absolute;
		top: -12px;
		right: 12px;
		box-sizing: border-box;
		display: inline-flex;
		align-items: center;
		gap: 7px;
		height: 24px;
		padding: 0 10px;
		font-size: 13px;
		line-height: 18px;
		font-weight: 500;
		color: var(--foreground);
		background: var(--paper-1);
		border: 1px solid var(--line);
		border-radius: 7px;
		white-space: nowrap;
	}

	.l2 > .chip {
		background: var(--paper-2);
	}

	.chip > :global(svg) {
		color: var(--ink-2);
	}

	.muted {
		color: var(--ink-2);
	}

	.foldrow {
		display: flex;
		align-items: center;
		width: 100%;
		height: 24px;
		padding: 0 4px;
		margin: 0;
		border: none;
		background: none;
		font: inherit;
		color: inherit;
		text-align: left;
		cursor: pointer;
	}

	.foldrow:focus-visible {
		outline: 2px solid var(--live);
		outline-offset: 2px;
		border-radius: 6px;
	}

	.fold {
		display: flex;
		align-items: center;
		gap: 4px;
	}

	/* One glyph per step, carrying that step's status. */
	.g {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		height: 26px;
		padding: 0 5px;
		border: 1px solid var(--line);
		border-radius: 6px;
		background: var(--paper-2);
		color: var(--ink-2);
	}

	/* Only this sheet's own fold, never a folded sheet nested in it. */
	.l2 > .foldrow .g {
		background: var(--paper-1);
	}

	.sp {
		flex-grow: 1;
	}

	.cap {
		font-size: 13px;
		line-height: 18px;
		font-weight: 600;
		color: var(--ink-2);
		white-space: nowrap;
	}
</style>
