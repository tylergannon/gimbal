<script module lang="ts">
  import type { InterviewAnswer } from "../../routes/interview.remote.js";
  import type { LoopMessage, Steer } from "../../routes/steer.remote.js";

  export type ActionFeedback = {
    ok: boolean;
    message: string;
  };
</script>

<script lang="ts">
  import BotIcon from "@lucide/svelte/icons/bot";
  import BracesIcon from "@lucide/svelte/icons/braces";
  import ChevronRightIcon from "@lucide/svelte/icons/chevron-right";
  import EyeIcon from "@lucide/svelte/icons/eye";
  import InterviewIcon from "@lucide/svelte/icons/message-circle-question-mark";
  import RepeatIcon from "@lucide/svelte/icons/repeat-2";
  import TerminalIcon from "@lucide/svelte/icons/terminal";
  import { Badge } from "#lib/components/ui/badge/index.js";
  import { Button } from "#lib/components/ui/button/index.js";
  import { Textarea } from "#lib/components/ui/textarea/index.js";
  import SessionTimeline from "./SessionTimeline.svelte";
  import {
    usageOf,
    usageText,
    type CommandRow,
    type InterviewRow,
    type RunObservation,
    type RunSnapshot,
    type ScopeRow,
    type TurnRow,
  } from "../observation/index.js";
  import type { NodeOperation } from "./Node.svelte";
  import Pip, { type PipState } from "./Pip.svelte";
  import type { RunSelection } from "./selection.js";

  let {
    snapshot,
    selection,
    observation,
    revision = 0,
    onsteer,
    onloop,
    onanswer,
  }: {
    snapshot: RunSnapshot;
    selection?: RunSelection;
    observation?: RunObservation;
    revision?: number;
    onsteer?: (request: Steer) => Promise<ActionFeedback>;
    onloop?: (request: LoopMessage) => Promise<ActionFeedback>;
    onanswer?: (request: InterviewAnswer) => Promise<ActionFeedback>;
  } = $props();

  let steerMessage = $state("");
  let loopMessage = $state("");
  let answer = $state("");
  let pending = $state(false);
  let feedback = $state<ActionFeedback>();

  // Selection retains identity while every displayed row comes from the most
  // recent observation snapshot.
  const scope = $derived(
    selection ? snapshot.scopes[selection.scope.key] : undefined,
  );
  const nodeOperation = $derived<NodeOperation | undefined>(
    selection?.kind === "node" ? selection.operation : undefined,
  );
  const agentDefinition = $derived(
    nodeOperation?.kind === "agent_call" ? nodeOperation : undefined,
  );

  const selectedTurnID = $derived.by(() => {
    if (!selection) return "";
    if (selection.kind === "history-turn") return selection.turn.id;
    if (selection.kind === "node" && selection.runtime && "prompt" in selection.runtime)
      return selection.runtime.id;
    if (selection.kind === "watcher") return selection.turn?.id ?? "";
    return "";
  });
  const turn = $derived<TurnRow | undefined>(
    selectedTurnID ? snapshot.turns[selectedTurnID] : latestTurn(scope, nodeOperation),
  );

  const selectedCommandID = $derived.by(() => {
    if (!selection) return "";
    if (selection.kind === "history-command") return selection.command.id;
    if (selection.kind === "node" && selection.runtime && "exit_code" in selection.runtime)
      return selection.runtime.id;
    return "";
  });
  const command = $derived<CommandRow | undefined>(
    selectedCommandID ? snapshot.commands[selectedCommandID] : latestCommand(scope, nodeOperation),
  );

  const interviewSession = $derived(
    selection?.kind === "node" &&
      selection.runtime &&
      "question_id" in selection.runtime
      ? selection.runtime.session
      : undefined,
  );
  const interview = $derived<InterviewRow | undefined>(
    latestInterview(scope, nodeOperation, interviewSession),
  );
  const interviewQuestions = $derived.by(() => {
    if (!interview) return [];
    return Object.values(snapshot.interviews)
      .filter(
        (row) =>
          row.scope === interview.scope &&
          row.session === interview.session &&
          row.name === interview.name,
      )
      .sort((left, right) => left.asked - right.asked);
  });
  const session = $derived(turn ? snapshot.sessions[turn.session] : interview ? snapshot.sessions[interview.session] : undefined);
  const transcriptState = $derived(turn && observation ? observation.state(turn.id) : undefined);
  const notObserved = $derived(
    selection?.kind === "node" &&
      (selection.operation.kind === "agent_call" ||
        selection.operation.kind === "command" ||
        selection.operation.kind === "interview") &&
      !turn &&
      !command &&
      !interview,
  );
  const liveTurn = $derived(snapshot.run.status === "running" && !!turn && turn.ended === 0);
  const liveLoop = $derived(snapshot.run.status === "running" && !!scope?.loop && scope.status === "running");
  const pendingInterview = $derived(
    snapshot.run.status === "running" && interview?.status === "pending",
  );

  const title = $derived.by(() => {
    if (!selection) return snapshot.run.name;
    if (selection.kind === "watcher") return selection.supervisor.session;
    if (selection.kind === "history-turn") return session?.name ?? selection.turn.id;
    if (selection.kind === "history-command") return selection.command.name;
    if (selection.kind === "history-scope" || selection.kind === "sheet" || selection.kind === "instance")
      return selection.scope.name || snapshot.run.name;
    return selection.operation.kind === "agent_call"
      ? selection.operation.session
      : selection.operation.name;
  });
  const kind = $derived.by(() => {
    if (!selection) return "run";
    if (selection.kind === "watcher") return "watcher";
    if (notObserved && selection.kind === "node") {
      return selection.operation.kind === "agent_call"
        ? "agent call"
        : selection.operation.kind;
    }
    if (command) return "command";
    if (interview) return "interview";
    if (turn) return "agent call";
    if (scope?.loop) return "loop";
    return "scope";
  });
  const pipState = $derived<PipState>(
    notObserved
      ? "not-yet"
      : command
      ? command.interrupted
        ? "ended"
        : command.error || (command.ended > 0 && command.exit_code !== 0)
        ? "failed"
        : command.ended
          ? "ended"
        : "running"
      : interview
        ? interview.status === "pending"
          ? "waiting"
          : "ended"
        : turn
          ? turn.interrupted
            ? "ended"
            : turn.error
            ? "failed"
            : turn.ended
              ? "ended"
              : "running"
          : scope?.error
            ? "failed"
            : scope?.status === "running"
              ? "running"
              : "ended",
  );

  function latestTurn(selectedScope: ScopeRow | undefined, operation: NodeOperation | undefined) {
    if (!selectedScope || operation?.kind !== "agent_call") return undefined;
    const sessionIDs = new Set(
      Object.values(snapshot.sessions)
        .filter((row) => row.name === operation.session)
        .map((row) => row.id),
    );
    return Object.values(snapshot.turns)
      .filter((row) => row.scope === selectedScope.key && sessionIDs.has(row.session))
      .sort((left, right) => left.started - right.started)
      .at(-1);
  }

  function latestWatcherTurn(selectedScope: ScopeRow | undefined, sessionName: string) {
    if (!selectedScope) return undefined;
    const sessionIDs = new Set(
      Object.values(snapshot.sessions)
        .filter((row) => row.name === sessionName)
        .map((row) => row.id),
    );
    return Object.values(snapshot.turns)
      .filter((row) => row.scope === selectedScope.key && sessionIDs.has(row.session))
      .sort((left, right) => left.started - right.started)
      .at(-1);
  }

  function turnOutcome(row: TurnRow) {
    if (!row.ended) return "running";
    if (row.interrupted) return "interrupted";
    return row.error ? "failed" : "ended";
  }

  function latestCommand(selectedScope: ScopeRow | undefined, operation: NodeOperation | undefined) {
    if (!selectedScope || operation?.kind !== "command") return undefined;
    return Object.values(snapshot.commands)
      .filter((row) => row.scope === selectedScope.key && row.name === operation.name)
      .sort((left, right) => left.started - right.started)
      .at(-1);
  }

  function latestInterview(
    selectedScope: ScopeRow | undefined,
    operation: NodeOperation | undefined,
    selectedSession?: string,
  ) {
    if (!selectedScope || operation?.kind !== "interview") return undefined;
    return Object.values(snapshot.interviews)
      .filter(
        (row) =>
          row.scope === selectedScope.key &&
          row.name === operation.name &&
          (!selectedSession || row.session === selectedSession),
      )
      .sort((left, right) => left.asked - right.asked)
      .at(-1);
  }

  const taskDescription = (value: unknown) => {
    if (typeof value !== "object" || value === null) return String(value ?? "");
    const task = value as { description?: unknown; name?: unknown };
    return String(task.description ?? task.name ?? "");
  };
  const text = (value: unknown) =>
    typeof value === "string" ? value : JSON.stringify(value, null, 2);
  const duration = (started: number, ended: number, stated = 0) => {
    const milliseconds = stated || Math.max(0, (ended || Date.now()) - started);
    const seconds = Math.floor(milliseconds / 1000);
    return `${Math.floor(seconds / 60)}m ${(seconds % 60).toString().padStart(2, "0")}s`;
  };

  async function act(run: () => Promise<ActionFeedback>, clear?: () => void) {
    pending = true;
    feedback = undefined;
    try {
      feedback = await run();
      if (feedback.ok) clear?.();
    } catch (error) {
      feedback = { ok: false, message: error instanceof Error ? error.message : String(error) };
    } finally {
      pending = false;
    }
  }
</script>

<aside class="detail">
  <header class="detail-head">
    <div class="title-row">
      {#if kind === "agent call"}<BotIcon size={16} />
      {:else if kind === "command"}<TerminalIcon size={16} />
      {:else if kind === "interview"}<InterviewIcon size={16} />
      {:else if kind === "watcher"}<EyeIcon size={16} />
      {:else if kind === "loop"}<RepeatIcon size={16} />
      {:else}<BracesIcon size={16} />{/if}
      <h2>{title}</h2>
      <Badge variant="outline">{kind}</Badge>
      <span class="spacer"></span>
      <Pip state={pipState} />
    </div>
    {#if scope}
      <div class="placement">scope <code>{scope.key || "root"}</code>{#if session} · session <code>{session.id}</code>{/if}</div>
    {/if}
  </header>

  <div class="detail-body">
    {#if !selection}
      <section>
        <div class="section-title">Run overview</div>
        {#if snapshot.run.error}
          <p>{snapshot.run.error}</p>
        {:else}
          <p class="empty-selection">Select a sheet, call, command, interview, or watcher to inspect it.</p>
        {/if}
        <dl>
          <dt>Scopes</dt><dd>{Object.keys(snapshot.scopes).length}</dd>
          <dt>Turns</dt><dd>{Object.keys(snapshot.turns).length}</dd>
          <dt>Commands</dt><dd>{Object.keys(snapshot.commands).length}</dd>
        </dl>
      </section>
    {:else if selection.kind === "watcher"}
      <section>
        <div class="section-title">Instruction</div>
        <p>{selection.supervisor.instruction}</p>
      </section>
      <section>
        <div class="section-title">Session</div>
        <dl><dt>Role</dt><dd>{selection.supervisor.role}</dd><dt>Source</dt><dd><code>{selection.supervisor.file}:{selection.supervisor.line}</code></dd></dl>
      </section>
    {:else if notObserved}
      <section class="not-observed">
        <div class="section-title">Not observed</div>
        <p>
          This step did not run in <code>{scope?.key || "root"}</code>. It may be behind a guard or
          belong to a later path; the containing scope’s outcome is not the outcome of this step.
        </p>
      </section>
    {:else if interview}
      <section class="exchanges">
        <div class="section-title">Exchanges</div>
        {#each interviewQuestions as question, index (question.question_id)}
          <article class:waiting={question.status === "pending"}>
            <strong>Question {index + 1}</strong>
            <p>{question.question}</p>
          </article>
          {#if question.status === "answered"}
            <article class="answer"><strong>You</strong><p>{question.answer || "Interview ended."}</p></article>
          {/if}
        {/each}
      </section>
      {#if pendingInterview}
        <section>
          <label for="answer">Your answer</label>
          <Textarea id="answer" rows={4} bind:value={answer} placeholder="Answer here. Send with ⌘⏎" />
          <div class="action-row">
            <Button
              size="sm"
              disabled={pending}
              onclick={() =>
                act(
                  () => onanswer?.({ run: snapshot.run.id, question_id: interview.question_id, answer }) ?? Promise.resolve({ ok: false, message: "Answer delivery is unavailable." }),
                  () => (answer = ""),
                )}
            >Answer</Button>
            <Button
              variant="outline"
              size="sm"
              disabled={pending}
              onclick={() =>
                act(() => onanswer?.({ run: snapshot.run.id, question_id: interview.question_id, answer: "" }) ?? Promise.resolve({ ok: false, message: "Answer delivery is unavailable." }))}
            >End interview</Button>
          </div>
          <p class="help">End interview sends no answer. The workflow continues with the exchanges so far.</p>
        </section>
      {/if}
    {:else if command}
      {#if !command.interrupted && (command.error || command.exit_code !== 0)}
        <div class="failure"><strong>{command.ended ? `Command exited ${command.exit_code}` : "Command failed"}</strong><p>{command.error || "The recorded command returned a nonzero exit code."}</p></div>
      {/if}
      <section>
        <div class="section-title">This instance</div>
        <dl>
          <dt>Command</dt><dd><code>{command.command} {command.args.join(" ")}</code></dd>
          <dt>Exit code</dt><dd>{command.ended ? command.exit_code : "running"}</dd>
          <dt>Duration</dt><dd>{duration(command.started, command.ended, command.duration)}</dd>
          <dt>Workdir</dt><dd><code>{command.workdir}</code></dd>
        </dl>
      </section>
      <section class="command-output">
        <div class="section-title">Stdout</div>
        <pre>{command.stdout || "No stdout was recorded."}</pre>
        {#if command.stdout_file}
          <p class="output-reference">Complete output <code>{command.stdout_file}</code></p>
        {:else}
          <p class="help">No complete-output reference was recorded.</p>
        {/if}
      </section>
      <section class="command-output">
        <div class="section-title">Stderr</div>
        <pre>{command.stderr || "No stderr was recorded."}</pre>
        {#if command.stderr_file}
          <p class="output-reference">Complete output <code>{command.stderr_file}</code></p>
        {:else}
          <p class="help">No complete-output reference was recorded.</p>
        {/if}
      </section>
    {:else if turn}
      {#if scope?.task !== undefined}
        <section><div class="section-title">Assignment</div><p>{taskDescription(scope.task)}</p></section>
      {/if}
      <section>
        <div class="section-title">Activity <span>· {snapshot.model_calls[turn.id]?.length ?? 0} model calls</span></div>
        {#each (snapshot.model_calls[turn.id] ?? []).slice(-4) as call (call.message)}
          <div class="activity"><code>{new Date(call.started).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" })}</code><span>{call.model}</span><span>{call.output} out</span></div>
        {/each}
      </section>
      <section>
        <div class="section-title">Transcript and tool activity</div>
        {#if transcriptState && observation}
          <SessionTimeline state={transcriptState} {revision} {observation} turn={turn.id} />
        {:else}
          <p class="help">No session transcript events have been recorded for this turn.</p>
        {/if}
      </section>
      <div class="disclosures">
        <details><summary><ChevronRightIcon size={14} />Prompt sent <span>{turn.prompt.length} chars</span></summary><pre>{turn.prompt}</pre></details>
        <details>
          <summary><ChevronRightIcon size={14} />Watchers <span>{agentDefinition?.supervisors.length ?? 0}</span></summary>
          {#if agentDefinition}
            {#each agentDefinition.supervisors as watcher (watcher.session)}
              {@const watcherTurn = latestWatcherTurn(scope, watcher.session)}
              <article class="watcher-detail">
                <div><strong>{watcher.session}</strong><span>{watcher.role}</span></div>
                <p>{watcher.instruction || "No watcher instruction was recorded."}</p>
                {#if watcherTurn}
                  <dl>
                    <dt>Latest turn in scope</dt><dd><code>{watcherTurn.id}</code></dd>
                    <dt>Outcome</dt><dd>{turnOutcome(watcherTurn)}</dd>
                  </dl>
                  {#if watcherTurn.result}
                    <pre>{watcherTurn.result}</pre>
                  {:else if watcherTurn.error}
                    <pre>{watcherTurn.error}</pre>
                  {:else}
                    <p class="help">No completed watcher result has been recorded.</p>
                  {/if}
                {:else}
                  <p class="help">No watcher turn has been recorded in this scope.</p>
                {/if}
              </article>
            {:else}
              <p class="disclosure-empty">No watchers are defined for this call.</p>
            {/each}
          {:else}
            <p class="disclosure-empty">No watcher definition is available for this recorded turn.</p>
          {/if}
        </details>
        <details><summary><ChevronRightIcon size={14} />Session <span>{session?.name ?? turn.session}</span></summary><dl><dt>Adapter</dt><dd>{session?.adapter}</dd><dt>Model</dt><dd>{session?.model}</dd></dl></details>
        <details><summary><ChevronRightIcon size={14} />Usage <span>{usageText(Object.values(snapshot.turn_usage[turn.id] ?? {})[0] ?? usageOf(undefined))}</span></summary></details>
        <details>
          <summary><ChevronRightIcon size={14} />Definition <span>{#if agentDefinition}<code>{agentDefinition.file}:{agentDefinition.line}</code>{:else}not available{/if}</span></summary>
          {#if agentDefinition}
            <dl>
              <dt>Role</dt><dd>{agentDefinition.role}</dd>
              <dt>Session</dt><dd>{agentDefinition.session}</dd>
              <dt>Source</dt><dd><code>{agentDefinition.file}:{agentDefinition.line}</code></dd>
            </dl>
            {#if agentDefinition.prompt}
              <pre>{agentDefinition.prompt}</pre>
            {:else}
              <p class="disclosure-empty">No declared prompt is present in the workflow definition.</p>
            {/if}
          {:else}
            <p class="disclosure-empty">No workflow definition is available for this recorded turn.</p>
          {/if}
        </details>
      </div>
    {:else if scope}
      {#if scope.task !== undefined}<section><div class="section-title">Assignment</div><p>{taskDescription(scope.task)}</p></section>{/if}
      <section>
        <div class="section-title">Values written</div>
        {#if Object.keys(scope.values).length === 0}<p class="help">No values were written in this scope.</p>{/if}
        {#each Object.entries(scope.values) as [key, value] (key)}
          <div class="value"><code>{key}</code><pre>{value.artifact?.preview ?? text(value.value)}</pre></div>
        {/each}
      </section>
      <section><div class="section-title">Sessions</div>{#each Object.values(snapshot.sessions).filter((row) => row.scope === scope.key) as row (row.id)}<div class="session-row"><Pip state={Object.values(snapshot.turns).some((turnRow) => turnRow.session === row.id && turnRow.ended === 0) ? "running" : "ended"} /><code>{row.id}</code><span>{row.model}</span></div>{/each}</section>
    {/if}

    {#if feedback}
      <p class="feedback" class:error={!feedback.ok} role="status">{feedback.message}</p>
    {/if}
  </div>

  {#if liveTurn && turn}
    <footer class="detail-foot">
      <Textarea rows={2} bind:value={steerMessage} placeholder="Steer this turn…" />
      <div class="footer-row"><span>Lands before the next model call. Dropped if the turn ends first.</span><Button size="sm" disabled={pending || !steerMessage.trim()} onclick={() => act(() => onsteer?.({ run: snapshot.run.id, session: turn.session, message: steerMessage }) ?? Promise.resolve({ ok: false, message: "Steering is unavailable." }), () => (steerMessage = ""))}>Steer</Button></div>
    </footer>
  {:else if liveLoop && scope}
    <footer class="detail-foot">
      <Textarea rows={2} bind:value={loopMessage} placeholder="Message the loop planner…" />
      <div class="footer-row"><Button variant="outline" size="sm" disabled={pending} onclick={() => act(() => onloop?.({ run: snapshot.run.id, scope: scope.key, message: "", wrap_up: true }) ?? Promise.resolve({ ok: false, message: "Loop control is unavailable." }))}>Wrap up</Button><span>Read at the planner’s next decision.</span><Button size="sm" disabled={pending || !loopMessage.trim()} onclick={() => act(() => onloop?.({ run: snapshot.run.id, scope: scope.key, message: loopMessage, wrap_up: false }) ?? Promise.resolve({ ok: false, message: "Loop control is unavailable." }), () => (loopMessage = ""))}>Send</Button></div>
    </footer>
  {:else if snapshot.run.status !== "running"}
    <footer class="recorded">This run has ended. Steering, answers, and stop controls are not offered on a record.</footer>
  {/if}
</aside>

<style>
  .detail { display: flex; width: 400px; min-width: 340px; flex-shrink: 0; flex-direction: column; overflow: hidden; color: var(--card-foreground); background: var(--card); border-left: 1px solid var(--map-line); }
  .detail-head { display: flex; flex-direction: column; gap: 8px; padding: 14px 16px 12px; background: color-mix(in oklch, var(--muted) 60%, var(--card)); border-bottom: 1px solid var(--border); }
  .title-row { display: flex; align-items: center; gap: 8px; }
  h2 { margin: 0; overflow: hidden; font-size: 16px; line-height: 24px; letter-spacing: -.01em; text-overflow: ellipsis; white-space: nowrap; }
  .spacer { flex: 1; }
  .placement, .help, .recorded { color: var(--status-muted); font-size: 13px; line-height: 18px; }
  code { font-family: var(--font-mono); }
  .detail-body { display: flex; min-height: 0; flex: 1; flex-direction: column; gap: 16px; padding: 14px 16px; overflow: auto; }
  section { display: flex; flex-direction: column; gap: 8px; }
  section p { margin: 0; font-size: 14px; line-height: 20px; white-space: pre-wrap; }
  .section-title { padding-bottom: 6px; font-size: 13px; font-weight: 600; line-height: 18px; border-bottom: 1px solid var(--border); }
  .section-title span { color: var(--status-muted); font-weight: 400; }
  dl { display: grid; grid-template-columns: 92px minmax(0, 1fr); gap: 6px 12px; margin: 0; font-size: 13px; line-height: 18px; }
  dt { color: var(--status-muted); }
  dd { min-width: 0; margin: 0; overflow-wrap: anywhere; }
  pre { max-height: 220px; margin: 0; padding: 8px 10px; overflow: auto; color: var(--foreground); font-family: var(--font-mono); font-size: 12px; line-height: 18px; white-space: pre-wrap; background: var(--muted); border: 1px solid var(--border); border-radius: 6px; }
  .activity, .session-row { display: flex; align-items: center; gap: 8px; min-width: 0; font-size: 13px; }
  .activity code { width: 64px; flex-shrink: 0; color: var(--status-muted); }
  .activity span:nth-child(2), .session-row code { min-width: 0; flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .activity span:last-child, .session-row span { color: var(--status-muted); }
  .disclosures details { border-top: 1px solid var(--border); }
  .disclosures details:last-child { border-bottom: 1px solid var(--border); }
  summary { display: flex; height: 40px; align-items: center; gap: 8px; font-size: 13px; font-weight: 500; cursor: pointer; list-style: none; }
  summary span { margin-left: auto; overflow: hidden; max-width: 230px; color: var(--status-muted); font-weight: 400; text-overflow: ellipsis; white-space: nowrap; }
  details[open] summary :global(svg) { transform: rotate(90deg); }
  details pre, details dl { margin-bottom: 10px; }
  .failure { padding: 10px 12px; color: var(--destructive); font-size: 13px; border: 1px solid color-mix(in oklch, var(--destructive) 45%, transparent); border-radius: 7px; }
  .not-observed { padding: 10px 12px; border: 1px dashed var(--map-line); border-radius: 7px; }
  .failure p { margin: 3px 0 0; color: var(--status-muted); }
  .exchanges article { padding: 8px 10px; font-size: 13px; border-left: 2px solid var(--map-line-strong); }
  .exchanges article.waiting { background: var(--map-live-soft); border-left-color: var(--status-live); border-radius: 0 6px 6px 0; }
  .exchanges article.answer { background: var(--muted); border: 1px solid var(--border); border-radius: 6px; }
  .exchanges article strong { font-size: 13px; }
  label { font-size: 13px; font-weight: 600; }
  .action-row, .footer-row { display: flex; align-items: center; gap: 8px; }
  .value { display: grid; gap: 5px; }
  .output-reference { margin: 0; color: var(--status-muted); font-size: 12px; line-height: 18px; overflow-wrap: anywhere; }
  .watcher-detail { display: grid; gap: 8px; padding: 0 0 10px 22px; }
  .watcher-detail + .watcher-detail { padding-top: 10px; border-top: 1px solid var(--border); }
  .watcher-detail > div { display: flex; align-items: baseline; gap: 8px; font-size: 13px; }
  .watcher-detail > div span { color: var(--status-muted); }
  .watcher-detail p, .disclosure-empty { margin: 0 0 10px 22px; font-size: 13px; line-height: 18px; white-space: pre-wrap; }
  .watcher-detail p { margin: 0; }
  .watcher-detail dl { margin-bottom: 0; }
  .feedback { margin: 0; padding: 7px 9px; color: var(--foreground); font-size: 13px; background: var(--map-live-soft); border-radius: 6px; }
  .feedback.error { color: var(--destructive); background: color-mix(in oklch, var(--destructive) 10%, transparent); }
  .detail-foot { display: flex; flex-direction: column; gap: 8px; padding: 12px 16px; background: color-mix(in oklch, var(--muted) 60%, var(--card)); border-top: 1px solid var(--border); }
  .footer-row span { flex: 1; color: var(--status-muted); font-size: 13px; line-height: 18px; }
  .recorded { padding: 12px 16px; color: var(--foreground); background: color-mix(in oklch, var(--muted) 60%, var(--card)); border-top: 1px solid var(--border); }
  @media (max-width: 960px) { .detail { width: 340px; } }
</style>
