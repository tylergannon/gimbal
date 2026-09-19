<script module lang="ts">
	import { defineMeta } from "@storybook/addon-svelte-csf";
	import type { CommandRow, TurnRow } from "../observation/index.js";
	import Node from "./Node.svelte";
	import { implementInterviewRun, planTripRun, recordedFailure } from "./fixtures/index.js";
	import { stepName, stepsOf, type Step, type StepRow } from "./step.js";

	const { Story } = defineMeta({
		title: "Gimble/Run/Node",
		component: Node,
		tags: ["autodocs"],
		argTypes: {
			meta: { control: "text" },
			selected: { control: "boolean" },
			small: { control: "boolean" },
			step: { control: false },
			row: { control: false },
		},
	});

	/** The one step of a body with this name, so a story says which step it
	 * draws instead of restating it. */
	const named = (steps: Step[], name: string): Step => {
		const found = steps.find((one) => stepName(one) === name);
		if (found === undefined) throw new Error(`the fixture body has no step named ${name}`);
		return found;
	};

	/** The row a run wrote under this key. */
	const rowAt = <T,>(table: Record<string, T>, key: string): T => {
		const found = table[key];
		if (found === undefined) throw new Error(`the fixture has no row at ${key}`);
		return found;
	};

	const { graph, snapshot } = implementInterviewRun;

	// The steps of the loop implement-interview works in: the coding call,
	// the four checks, and the validator's call.
	const loop = graph.body.find((operation) => operation.kind === "promise_loop");
	const work = loop?.kind === "promise_loop" ? stepsOf(loop.body) : [];
	const coding = named(work, "coding");
	const taskCheck = named(work, "task-check");
	const test = named(work, "test");
	const validation = named(work, "qa-orchestration");

	// plan-trip's two legs each ask the same named interview.
	const research = planTripRun.graph.body.find((operation) => operation.kind === "group");
	const lodging = research?.kind === "group" ? stepsOf(research.children[0].body) : [];
	const preferences = named(lodging, "preferences");

	const codingTurn = rowAt(snapshot.turns, "coding.1/turn.3");
	const codingTurn2 = rowAt(snapshot.turns, "coding.1/turn.2");
	const validationTurn = rowAt(snapshot.turns, "implementation.1/task.2/qa-orchestration.1/turn.1");
	const testRun = rowAt(snapshot.commands, "implementation.1/task.2/test.1");
	const taskCheckRun = rowAt(snapshot.commands, "implementation.1/task.2/task-check.1");
	const waiting = rowAt(planTripRun.snapshot.interviews, "01M2RWY8K3VQ2D7N5R1T9B4XZM");
	const answered = rowAt(planTripRun.snapshot.interviews, "01M2RWY1P8CNZ4G6L0M2V7XDKS");

	// Two readings the four fixtures have no row for. A node has to draw
	// them, so they are the fixtures' own rows with only the fields that
	// make the reading changed: a turn that ended carrying an error failed,
	// and a command the runtime has started but not reaped is running with
	// no exit code yet.
	const failedTurn: TurnRow = {
		...codingTurn2,
		result: "",
		error: "the model ended the turn without an answer",
	};
	const runningCommand: CommandRow = {
		...testRun,
		exit_code: -1,
		stdout: "",
		stderr: "",
		ended: 0,
		duration: 0,
	};

	const readings = ["ended", "running", "failed", "waiting for you", "not yet"];

	/** One cell of the matrix: a row of the fixtures and the line of run a
	 * node says under the name when it is holding that row. A cell with no
	 * row at all is a step the run has not reached; a missing cell is a
	 * reading that kind of row cannot hold. */
	type Cell = { row?: StepRow; meta: string } | undefined;

	/** Every reading a node can carry, per kind, in the order `readings`
	 * names them. `now` is the row the run is holding at the fixtures' clock,
	 * which the selected and small variants are drawn from. */
	const matrix: { step: Step; now: StepRow; meta: string; cells: Cell[] }[] = [
		{
			step: coding,
			now: codingTurn,
			meta: "turn 3",
			cells: [
				{ row: codingTurn2, meta: "turn 2" },
				{ row: codingTurn, meta: "turn 3" },
				{ row: failedTurn, meta: "turn 2" },
				undefined,
				{ meta: "not started" },
			],
		},
		{
			step: test,
			now: testRun,
			meta: "",
			cells: [
				{ row: testRun, meta: "" },
				{ row: runningCommand, meta: "" },
				{ row: recordedFailure, meta: "" },
				undefined,
				{ meta: "not started" },
			],
		},
		{
			step: preferences,
			now: waiting,
			meta: "question 2 · 40 s",
			cells: [
				{ row: answered, meta: "question 1" },
				undefined,
				undefined,
				{ row: waiting, meta: "question 2 · 40 s" },
				{ meta: "not started" },
			],
		},
	];
</script>

<Story name="Agent call" args={{ step: coding, row: codingTurn, meta: "turn 3" }} />

<Story name="Command" args={{ step: test, row: testRun }} />

<Story name="Interview" args={{ step: preferences, row: waiting, meta: "question 2 · 40 s" }} />

<Story name="Selected" args={{ step: coding, row: codingTurn, meta: "turn 3", selected: true }} />

<Story name="Small" args={{ step: taskCheck, row: taskCheckRun, small: true }} />

<Story name="Not yet" args={{ step: validation, meta: "not started" }} />

<!-- Main.html's selected coding turn and the four checks under it, at the
     two sizes the map draws them. -->
<Story name="A scope's steps" asChild>
	<div class="column">
		<Node step={coding} row={codingTurn} meta="turn 3" selected />
		<Node step={taskCheck} row={taskCheckRun} small />
		<Node step={test} row={testRun} small />
		<Node step={validation} meta="not started" />
	</div>
</Story>

<Story name="Every reading" asChild>
	<div class="grid" style="--columns: {readings.length + 1}">
		<span class="head"></span>
		{#each readings as reading (reading)}
			<span class="head">{reading}</span>
		{/each}
		{#each matrix as line (stepName(line.step))}
			<span class="head">{line.step.kind.replace("_", " ")}</span>
			{#each line.cells as cell, index (index)}
				{#if cell === undefined}
					<span class="gap">—</span>
				{:else}
					<Node step={line.step} row={cell.row} meta={cell.meta} />
				{/if}
			{/each}
		{/each}
	</div>
	<p class="note">
		A blank is a reading that kind of row cannot hold: only an interview waits for a person, and an
		interview row is only ever pending or answered. Every node is drawn from one of the four
		fixtures' own rows.
	</p>
</Story>

<Story name="Selected and small, every kind" asChild>
	<div class="grid" style="--columns: 3">
		<span class="head"></span>
		<span class="head">selected</span>
		<span class="head">small</span>
		{#each matrix as line (stepName(line.step))}
			<span class="head">{line.step.kind.replace("_", " ")}</span>
			<Node step={line.step} row={line.now} meta={line.meta} selected />
			<Node step={line.step} row={line.now} meta={line.meta} small />
		{/each}
	</div>
</Story>

<style>
	.column {
		display: flex;
		flex-direction: column;
		gap: 12px;
		width: 360px;
	}

	.grid {
		display: grid;
		grid-template-columns: max-content repeat(calc(var(--columns) - 1), 300px);
		gap: 12px;
		align-items: center;
	}

	.head {
		font-size: 13px;
		line-height: 18px;
		font-weight: 600;
		color: var(--ink-2);
		white-space: nowrap;
	}

	.gap {
		font-size: 13px;
		line-height: 18px;
		color: var(--ink-2);
		text-align: center;
	}

	.note {
		max-width: 760px;
		margin: 16px 0 0;
		font-size: 13px;
		line-height: 18px;
		color: var(--ink-2);
	}
</style>
