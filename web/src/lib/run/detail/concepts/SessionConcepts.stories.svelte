<script module lang="ts">
	import { defineMeta } from "@storybook/addon-svelte-csf";
	import { RunObservation } from "../../../observation/index.js";
	import { issue325Fixture, issue325Finished } from "../../fixtures/index.js";
	import SessionTabs from "./SessionTabs.svelte";
	import SessionScroll from "./SessionScroll.svelte";

	const runningTurnID = "coding.1/turn.2";
	const finishedTurnID = "coding.1/turn.2";

	const runningObservation = new RunObservation(issue325Fixture.snapshot);
	const finishedObservation = new RunObservation(issue325Finished);

	const fakeSteer = async () => {
		await new Promise((resolve) => setTimeout(resolve, 400));
		return { ok: true, message: "Sent into the running turn." };
	};
	const fakeStop = async () => {
		await new Promise((resolve) => setTimeout(resolve, 400));
		return { ok: true, message: "Stop requested." };
	};

	const { Story } = defineMeta({
		title: "Gimbal/Run/Concepts/Session detail",
		parameters: { layout: "centered" },
	});
</script>

<Story name="A tabs · running" asChild>
	<div style="width: 480px; height: 800px;">
		<SessionTabs
			snapshot={issue325Fixture.snapshot}
			observation={runningObservation}
			revision={runningObservation.revision}
			turnID={runningTurnID}
			onsteer={fakeSteer}
			onstop={fakeStop}
			onmaximize={() => {}}
			onopen={() => {}}
		/>
	</div>
</Story>

<Story name="A tabs · finished" asChild>
	<div style="width: 480px; height: 800px;">
		<SessionTabs
			snapshot={issue325Finished}
			observation={finishedObservation}
			revision={finishedObservation.revision}
			turnID={finishedTurnID}
			onsteer={fakeSteer}
			onstop={fakeStop}
			onmaximize={() => {}}
			onopen={() => {}}
		/>
	</div>
</Story>

<Story name="B scroll · running" asChild>
	<div style="width: 480px; height: 800px;">
		<SessionScroll
			snapshot={issue325Fixture.snapshot}
			observation={runningObservation}
			revision={runningObservation.revision}
			turnID={runningTurnID}
			onsteer={fakeSteer}
			onstop={fakeStop}
			onmaximize={() => {}}
			onopen={() => {}}
		/>
	</div>
</Story>

<Story name="B scroll · finished" asChild>
	<div style="width: 480px; height: 800px;">
		<SessionScroll
			snapshot={issue325Finished}
			observation={finishedObservation}
			revision={finishedObservation.revision}
			turnID={finishedTurnID}
			onsteer={fakeSteer}
			onstop={fakeStop}
			onmaximize={() => {}}
			onopen={() => {}}
		/>
	</div>
</Story>

<Story name="A tabs · wide" asChild>
	<div style="width: 1100px; height: 800px;">
		<SessionTabs
			snapshot={issue325Fixture.snapshot}
			observation={runningObservation}
			revision={runningObservation.revision}
			turnID={runningTurnID}
			onsteer={fakeSteer}
			onstop={fakeStop}
			onmaximize={() => {}}
			onopen={() => {}}
		/>
	</div>
</Story>

<Story name="B scroll · wide" asChild>
	<div style="width: 1100px; height: 800px;">
		<SessionScroll
			snapshot={issue325Fixture.snapshot}
			observation={runningObservation}
			revision={runningObservation.revision}
			turnID={runningTurnID}
			onsteer={fakeSteer}
			onstop={fakeStop}
			onmaximize={() => {}}
			onopen={() => {}}
		/>
	</div>
</Story>
