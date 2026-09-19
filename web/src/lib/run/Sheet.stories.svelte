<script module lang="ts">
	import { defineMeta } from "@storybook/addon-svelte-csf";
	import type { ScopeRow } from "../observation/index.js";
	import type { Graph } from "../workflow/types.js";
	import Node from "./Node.svelte";
	import Sheet, { type SheetInstance, type SheetStep } from "./Sheet.svelte";
	import { at, implementInterviewRun } from "./fixtures/index.js";
	import { stepName, stepsOf, type ScopeStep, type Step, type StepRow } from "./step.js";

	const { Story } = defineMeta({
		title: "Gimble/Run/Sheet",
		component: Sheet,
		tags: ["autodocs"],
		argTypes: {
			path: { control: "boolean" },
			selected: { control: "boolean" },
			folded: { control: "boolean" },
			foldable: { control: "boolean" },
			root: { control: "boolean" },
			writes: { control: { type: "number", min: 0, max: 16 } },
			scope: { control: false },
			row: { control: false },
			instances: { control: false },
			steps: { control: false },
		},
	});

	const { graph, snapshot } = implementInterviewRun;

	/** The row a run wrote under this key. */
	const rowAt = <T,>(table: Record<string, T>, key: string): T => {
		const found = table[key];
		if (found === undefined) throw new Error(`the fixture has no row at ${key}`);
		return found;
	};

	const scopeAt = (key: string): ScopeRow => rowAt(snapshot.scopes, key);

	// The shape: the reconnaissance group and the implementation loop.
	const group = graph.body.find((operation) => operation.kind === "group");
	const children = group?.kind === "group" ? group.children : [];
	const loop = graph.body.find((operation) => operation.kind === "promise_loop");
	const work = loop?.kind === "promise_loop" ? stepsOf(loop.body) : [];

	/** The keys one pass of the loop's body writes: every `Set` the graph
	 * states, and the `task` the runtime writes when it opens the scope. This
	 * is the "of 8" in the context chip; how many are written is the row's. */
	const setKeys = (body: Graph["body"]): string[] =>
		body.flatMap((operation) => {
			if (operation.kind === "condition") {
				return operation.branches.flatMap((branch) => setKeys(branch.body));
			}
			return operation.kind === "set" ? [operation.key] : [];
		});

	const taskWrites = loop?.kind === "promise_loop" ? setKeys(loop.body).length + 1 : 0;

	/** The session a turn ran on, without its ordinal, which is the name the
	 * call that made it carries. */
	const sessionName = (id: string): string =>
		id.slice(id.lastIndexOf("/") + 1).replace(/\.\d+$/, "");

	/** The row this scope wrote for one step of its body. */
	const rowFor = (key: string, step: Step): StepRow | undefined => {
		if (step.kind === "command") return snapshot.commands[`${key}/${stepName(step)}.1`];
		return Object.values(snapshot.turns).find(
			(turn) => turn.scope === key && sessionName(turn.session) === stepName(step),
		);
	};

	/** One pass of the loop's body, as its fold and its open sheet draw it. */
	const stepsIn = (key: string): { step: Step; row?: StepRow }[] =>
		work.map((step) => ({ step, row: rowFor(key, step) }));

	/** The instant Main.html is drawn at, which the fixtures share. A running
	 * scope's elapsed time is measured to it, never to the real clock. */
	const now = at("2026-09-17T12:48:50");

	const elapsed = (row: ScopeRow): string => {
		const seconds = Math.round(((row.ended === 0 ? now : row.ended) - row.began) / 1000);
		const minutes = Math.floor(seconds / 60);
		return minutes === 0
			? `${seconds} s`
			: `${minutes}m ${String(seconds % 60).padStart(2, "0")}s`;
	};

	const scope = (kind: ScopeStep["kind"], name: string): ScopeStep => ({ kind, name });

	const reconnaissance = scope("group", "reconnaissance");
	const implementation = scope("loop", "implementation");
	const task = scope("scope", "task");

	/** The three passes the planner has dispatched, oldest first. */
	const tasks: SheetInstance[] = [1, 2, 3].map((ordinal) => {
		const row = scopeAt(`implementation.1/task.${ordinal}`);
		return { row, steps: stepsIn(row.key), elapsed: elapsed(row) };
	});

	/** The two legs of the reconnaissance group, each one agent call. */
	const legs = children.map((child) => ({
		scope: scope("scope", child.name),
		row: scopeAt(`reconnaissance.1/${child.name}.1`),
		steps: stepsOf(child.body).map((step) => ({
			step,
			row: rowFor(`reconnaissance.1/${child.name}.1`, step),
		})),
	}));

	/** A group folds to one glyph per child scope. */
	const legGlyphs: SheetStep[] = legs.map((leg) => ({ step: leg.scope, row: leg.row }));

	/** A loop folds to one glyph per pass. */
	const taskGlyphs: SheetStep[] = tasks.map((one) => ({ step: task, row: one.row }));

	/** The one line of run under a node's name. A check that passed says
	 * nothing, the way `Main.html` draws it; one that did not says so. */
	const metaOf = (one: SheetStep): string => {
		if (one.status === "failed") return "the check failed";
		const row = one.row;
		if (row === undefined) return "not started";
		if ("question_id" in row) return row.status === "pending" ? "question waiting" : "answered";
		if ("exit_code" in row) {
			if (row.ended === 0) return "running";
			return row.exit_code === 0 ? "" : `exit ${row.exit_code}`;
		}
		if ("key" in row) return "";
		return `turn ${row.id.slice(row.id.lastIndexOf(".") + 1)}`;
	};

	/** The steps of a body, which are the ones a `Node` draws. A glyph can
	 * also stand for a nested scope, and a scope is a sheet, never a node. */
	const nodes = (shown: SheetStep[]): (SheetStep & { step: Step })[] =>
		shown.filter(
			(one): one is SheetStep & { step: Step } =>
				one.step.kind === "agent_call" ||
				one.step.kind === "command" ||
				one.step.kind === "interview",
		);

	/** Collapse.html's own pass: the second task with its first check failed.
	 * The fixture is `Main.html`'s run, where every check passed, so the
	 * failure is stated rather than taken from a row the run never wrote. */
	const collapseTasks: SheetInstance[] = tasks.map((one, index) =>
		index !== 1
			? one
			: {
					...one,
					steps: one.steps?.map((step) =>
						stepName(step.step) === "task-check" ? { ...step, status: "failed" } : step,
					),
				},
	);
</script>

<script lang="ts">
	// The pass each board is showing, which is also what it reports outwards:
	// one piece of state, so the label, the body, the fold and the line under
	// the board cannot disagree.
	let shown = $state(tasks[tasks.length - 1].row.key);
	let reported = $state(tasks[tasks.length - 1].row.key);
	let kept = $state(collapseTasks[1].row.key);
	let beside = $state(tasks[tasks.length - 1].row.key);
</script>

<Story
	name="Scope"
	args={{
		scope: legs[0].scope,
		row: legs[0].row,
		steps: legs[0].steps,
		elapsed: elapsed(legs[0].row),
	}}
>
	{#snippet template(args)}
		<div class="board" style="width: 320px">
			<Sheet {...args}>
				{#snippet children(steps)}
					{#each nodes(steps) as one (stepName(one.step))}
						<Node step={one.step} row={one.row} status={one.status} meta={metaOf(one)} />
					{/each}
				{/snippet}
			</Sheet>
		</div>
	{/snippet}
</Story>

<!-- Main.html's two boards without the spine: the reconnaissance group flat
     and off the path, the implementation loop on the path, and the pass the
     run is inside raised and selected. Tints alternate with depth. -->
<Story name="Nesting and the path to the selection" asChild>
	<div class="board">
		<Sheet scope={scope("scope", ".")} row={scopeAt("")} root>
			<Sheet
				scope={reconnaissance}
				row={scopeAt("reconnaissance.1")}
				steps={legGlyphs}
				elapsed={elapsed(scopeAt("reconnaissance.1"))}
			>
				<div class="side">
					{#each legs as leg (leg.scope.name)}
						<Sheet
							scope={leg.scope}
							row={leg.row}
							steps={leg.steps}
							elapsed={elapsed(leg.row)}
						>
							{#snippet children(steps)}
								{#each nodes(steps) as one (stepName(one.step))}
									<Node step={one.step} row={one.row} status={one.status} meta={metaOf(one)} />
								{/each}
							{/snippet}
						</Sheet>
					{/each}
				</div>
			</Sheet>

			<Sheet
				scope={implementation}
				row={scopeAt("implementation.1")}
				steps={taskGlyphs}
				elapsed={elapsed(scopeAt("implementation.1"))}
				path
			>
				<Sheet
					scope={task}
					instances={tasks}
					writes={taskWrites}
					bind:instance={shown}
					selected
				>
					{#snippet children(steps)}
						{#each nodes(steps) as one (stepName(one.step))}
							<Node
								step={one.step}
								row={one.row}
								status={one.status}
								meta={metaOf(one)}
								small={one.step.kind === "command"}
								selected={stepName(one.step) === "coding"}
							/>
						{/each}
					{/snippet}
				</Sheet>
			</Sheet>
		</Sheet>
	</div>
</Story>

<!-- Claim 11: the label's select, latest first, recolouring the sheet and
     reporting the pass it now shows. -->
<Story name="A repeated scope" asChild>
	<div class="board" style="width: 420px">
		<Sheet
			scope={task}
			instances={tasks}
			writes={taskWrites}
			bind:instance={shown}
			oninstance={(key) => (reported = key)}
			path
			selected
		>
			{#snippet children(steps)}
				{#each nodes(steps) as one (stepName(one.step))}
					<Node
						step={one.step}
						row={one.row}
						status={one.status}
						meta={metaOf(one)}
						small={one.step.kind === "command"}
					/>
				{/each}
			{/snippet}
		</Sheet>
		<p class="note">
			Reported to the outside: <span class="mono">{reported}</span>. The chevron beside the select
			folds the pass and opens it again.
		</p>
	</div>
</Story>

<!-- Collapse.html's second board, with the notes it carries. -->
<Story name="What a fold keeps" asChild>
	<div class="board" style="width: 460px">
		<Sheet
			scope={task}
			instances={collapseTasks}
			writes={taskWrites}
			bind:instance={kept}
			oninstance={(key) => (reported = key)}
			folded
		>
			{#snippet children(steps)}
				{#each nodes(steps) as one (stepName(one.step))}
					<Node
						step={one.step}
						row={one.row}
						status={one.status}
						meta={metaOf(one)}
						small={one.step.kind === "command"}
					/>
				{/each}
			{/snippet}
		</Sheet>
		<ol class="note">
			<li>The label stays, and so does the select on a repeated scope.</li>
			<li>One glyph per step, in source order, carrying that step's status in this pass.</li>
			<li>The scope's own status and elapsed time on the right.</li>
			<li>Nothing else. Names, prompts and the context chip wait until it is opened.</li>
			<li>The chevron opens it, and so do the glyphs. Opening a scope folds its siblings.</li>
		</ol>
		<p class="note">
			Showing <span class="mono">{kept}</span>. Picking another pass changes the glyphs, the status
			and the elapsed time together, and the body it opens to is that same pass.
		</p>
	</div>
</Story>

<!-- Claim 12: opening one sheet folds the sheets beside it, and the root is
     the canvas, so it has no fold of its own. -->
<Story name="Folding siblings" asChild>
	<div class="board">
		<Sheet scope={scope("scope", ".")} row={scopeAt("")} root>
			<Sheet
				scope={reconnaissance}
				row={scopeAt("reconnaissance.1")}
				steps={legGlyphs}
				elapsed={elapsed(scopeAt("reconnaissance.1"))}
				folded
			>
				<div class="side">
					{#each legs as leg (leg.scope.name)}
						<Sheet
							scope={leg.scope}
							row={leg.row}
							steps={leg.steps}
							elapsed={elapsed(leg.row)}
							folded={leg.scope.name !== "backend"}
						>
							{#snippet children(steps)}
								{#each nodes(steps) as one (stepName(one.step))}
									<Node step={one.step} row={one.row} status={one.status} meta={metaOf(one)} />
								{/each}
							{/snippet}
						</Sheet>
					{/each}
				</div>
			</Sheet>

			<Sheet
				scope={implementation}
				row={scopeAt("implementation.1")}
				steps={taskGlyphs}
				elapsed={elapsed(scopeAt("implementation.1"))}
			>
				<Sheet
					scope={task}
					instances={tasks}
					writes={taskWrites}
					bind:instance={beside}
					folded
				>
					{#snippet children(steps)}
						{#each nodes(steps) as one (stepName(one.step))}
							<Node
								step={one.step}
								row={one.row}
								status={one.status}
								meta={metaOf(one)}
								small={one.step.kind === "command"}
							/>
						{/each}
					{/snippet}
				</Sheet>
			</Sheet>
		</Sheet>
		<p class="note">
			Open the reconnaissance group and the implementation loop folds, and the other way about. Every
			sheet here folds and opens again: the plain ones by their label, the repeated pass by the
			chevron beside its select. The canvas has no label and never folds.
		</p>
	</div>
</Story>

<style>
	.board {
		width: 620px;
		display: flex;
		flex-direction: column;
		gap: 24px;
		padding: 20px 12px;
		background: var(--ground);
		border-radius: 12px;
	}

	.side {
		display: flex;
		gap: 12px;
	}

	.side > :global(*) {
		flex: 1 1 0;
		min-width: 0;
	}

	.note {
		margin: 0;
		padding-left: 18px;
		list-style: decimal;
		font-size: 13px;
		line-height: 20px;
		color: var(--ink-2);
	}

	p.note {
		padding-left: 0;
	}

	.mono {
		font-family: var(--font-mono, ui-monospace, SFMono-Regular, Menlo, Monaco, monospace);
		color: var(--foreground);
	}
</style>
