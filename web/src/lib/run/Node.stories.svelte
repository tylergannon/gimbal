<script module lang="ts">
	import { defineMeta } from "@storybook/addon-svelte-csf";
	import type { CommandRow, TurnRow } from "../observation/index.js";
	import Node from "./Node.svelte";
	import type { RowStatus } from "./Pip.svelte";
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
	const testRun = rowAt(snapshot.commands, "implementation.1/task.2/test.1");
	const taskCheckRun = rowAt(snapshot.commands, "implementation.1/task.2/task-check.1");
	const waiting = rowAt(planTripRun.snapshot.interviews, "01M2RWY8K3VQ2D7N5R1T9B4XZM");
	const answered = rowAt(planTripRun.snapshot.interviews, "01M2RWY1P8CNZ4G6L0M2V7XDKS");

	// Two readings the fixtures have no row for, made from their own rows by
	// changing only the fields that carry the reading: a turn that ended
	// carrying an error failed, and a command the runtime has started and not
	// yet reaped is running with no exit code.
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

	/** How long a command took, worded the way the map words elapsed time. */
	const took = (ms: number): string => {
		const seconds = Math.round(ms / 1000);
		const minutes = Math.floor(seconds / 60);
		return minutes === 0 ? `${seconds} s` : `${minutes}m ${String(seconds % 60).padStart(2, "0")}s`;
	};

	const readings = ["ended", "running", "failed", "waiting for you", "not yet"];

	/** One cell of the matrix: what the node is holding, and the line of run
	 * it says under the name. A cell with neither a row nor a state is a step
	 * the run has not reached. `status` states a state no row of the run
	 * produces, which is the only way a gallery can draw all five. */
	type Cell = { row?: StepRow; status?: RowStatus; meta: string };

	/** Every reading, per kind, in the order `readings` names them. `now` is
	 * what the row is holding at the fixtures' clock, which the selected and
	 * small variants are drawn from. */
	const matrix: { step: Step; now: StepRow; meta: string; cells: Cell[] }[] = [
		{
			step: coding,
			now: codingTurn,
			meta: "turn 3",
			cells: [
				{ row: codingTurn2, meta: "turn 2" },
				{ row: codingTurn, meta: "turn 3" },
				{ row: failedTurn, meta: "turn 2" },
				{ status: "pending", meta: "waiting for you" },
				{ meta: "not started" },
			],
		},
		{
			step: test,
			now: testRun,
			meta: `exit ${testRun.exit_code} · ${took(testRun.duration)}`,
			cells: [
				{ row: testRun, meta: `exit ${testRun.exit_code} · ${took(testRun.duration)}` },
				{ row: runningCommand, meta: "running" },
				{ row: recordedFailure, meta: `exit ${recordedFailure.exit_code}` },
				{ status: "pending", meta: "waiting for you" },
				{ meta: "not started" },
			],
		},
		{
			step: preferences,
			now: waiting,
			meta: "question 2 · 40 s",
			cells: [
				{ row: answered, meta: "question 1" },
				{ status: "running", meta: "running" },
				{ status: "failed", meta: "no answer" },
				{ row: waiting, meta: "question 2 · 40 s" },
				{ meta: "not started" },
			],
		},
	];

	/** The variants every kind is drawn in, beside the plain node. */
	const variants: { name: string; selected?: boolean; small?: boolean }[] = [
		{ name: "plain" },
		{ name: "selected", selected: true },
		{ name: "small", small: true },
		{ name: "small, selected", small: true, selected: true },
	];
</script>

<Story name="Agent call" args={{ step: coding, row: codingTurn, meta: "turn 3" }} />

<Story
	name="Command"
	args={{ step: test, row: testRun, meta: `exit ${testRun.exit_code} · ${took(testRun.duration)}` }}
/>

<Story name="Interview" args={{ step: preferences, row: waiting, meta: "question 2 · 40 s" }} />

<Story name="Selected" args={{ step: coding, row: codingTurn, meta: "turn 3", selected: true }} />

<Story
	name="Small"
	args={{
		step: taskCheck,
		row: taskCheckRun,
		meta: `exit ${taskCheckRun.exit_code} · ${took(taskCheckRun.duration)}`,
		small: true,
	}}
/>

<Story name="Not yet" args={{ step: validation, meta: "not started" }} />

<!-- Main.html's selected coding turn and the four checks under it, at the
     two sizes the map draws them. -->
<Story name="A scope's steps" asChild>
	<div class="column">
		<Node step={coding} row={codingTurn} meta="turn 3" selected />
		<Node
			step={taskCheck}
			row={taskCheckRun}
			meta={`exit ${taskCheckRun.exit_code} · ${took(taskCheckRun.duration)}`}
			small
		/>
		<Node
			step={test}
			row={testRun}
			meta={`exit ${testRun.exit_code} · ${took(testRun.duration)}`}
			small
		/>
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
				<Node step={line.step} row={cell.row} status={cell.status} meta={cell.meta} />
			{/each}
		{/each}
	</div>
	<p class="note">
		Eleven of these nodes hold a row of the fixtures. The other four are states the run's rows do not
		produce — an agent call and a command waiting on a person, an interview running and failed — so
		they are stated with <span class="mono">status</span>, a word of the run's own status
		vocabulary. The map hands rows; only a gallery states a state.
	</p>
</Story>

<Story name="Selected and small, every kind" asChild>
	<div class="grid" style="--columns: {variants.length + 1}">
		<span class="head"></span>
		{#each variants as variant (variant.name)}
			<span class="head">{variant.name}</span>
		{/each}
		{#each matrix as line (stepName(line.step))}
			<span class="head">{line.step.kind.replace("_", " ")}</span>
			{#each variants as variant (variant.name)}
				<Node
					step={line.step}
					row={line.now}
					meta={line.meta}
					selected={variant.selected}
					small={variant.small}
				/>
			{/each}
		{/each}
	</div>
	<p class="note">
		Every variant carries its meta line: the agent call is on turn 3, the command says its exit and
		how long it took, and the interview says which question is out and for how long.
	</p>
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

	.mono {
		font-family: var(--font-mono, ui-monospace, SFMono-Regular, Menlo, Monaco, monospace);
	}

	.note {
		max-width: 760px;
		margin: 16px 0 0;
		font-size: 13px;
		line-height: 18px;
		color: var(--ink-2);
	}
</style>
