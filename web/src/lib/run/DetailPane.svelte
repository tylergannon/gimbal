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
  import EyeIcon from "@lucide/svelte/icons/eye";
  import InterviewIcon from "@lucide/svelte/icons/message-circle-question-mark";
  import MaximizeIcon from "@lucide/svelte/icons/maximize-2";
  import MinimizeIcon from "@lucide/svelte/icons/minimize-2";
  import RepeatIcon from "@lucide/svelte/icons/repeat-2";
  import ServerIcon from "@lucide/svelte/icons/server";
  import TerminalIcon from "@lucide/svelte/icons/terminal";
  import { Badge } from "#lib/components/ui/badge/index.js";
  import { Button } from "#lib/components/ui/button/index.js";
  import { Textarea } from "#lib/components/ui/textarea/index.js";
  import {
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
  import TabBar from "./detail/TabBar.svelte";
  import SessionDetail, { type WatcherRow } from "./detail/SessionDetail.svelte";
  import ScopeContext from "./detail/ScopeContext.svelte";
  import Payload from "./detail/Payload.svelte";

  let {
    snapshot,
    selection,
    observation,
    revision = 0,
    width = 480,
    maximized = false,
    onsteer,
    onloop,
    onanswer,
    onstop,
    onmaximize,
    onselectscope,
  }: {
    snapshot: RunSnapshot;
    selection?: RunSelection;
    observation?: RunObservation;
    revision?: number;
    width?: number;
    maximized?: boolean;
    onsteer?: (request: Steer) => Promise<ActionFeedback>;
    onloop?: (request: LoopMessage) => Promise<ActionFeedback>;
    onanswer?: (request: InterviewAnswer) => Promise<ActionFeedback>;
    onstop?: (turn: TurnRow) => Promise<ActionFeedback>;
    onmaximize?: () => void;
    onselectscope?: (scopeKey: string) => void;
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
    if (selection.kind === "service" && selection.runtime) return selection.runtime.id;
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
  const notObserved = $derived(
    selection?.kind === "node" &&
      (selection.operation.kind === "agent_call" ||
        selection.operation.kind === "command" ||
        selection.operation.kind === "interview") &&
      !turn &&
      !command &&
      !interview,
  );
  const liveLoop = $derived(snapshot.run.status === "running" && !!scope?.loop && scope.status === "running");
  const pendingInterview = $derived(
    snapshot.run.status === "running" && interview?.status === "pending",
  );

  const title = $derived.by(() => {
    if (!selection) return snapshot.run.name;
    if (selection.kind === "watcher") return selection.supervisor.session;
    if (selection.kind === "service") return selection.service.name;
    if (selection.kind === "sheet" || selection.kind === "instance")
      return selection.scope.name || snapshot.run.name;
    return selection.operation.kind === "agent_call"
      ? selection.operation.session
      : selection.operation.name;
  });
  const kind = $derived.by(() => {
    if (!selection) return "run";
    if (selection.kind === "watcher") return "watcher";
    if (selection.kind === "service" && !selection.runtime) return "service";
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

  // The last branch of the body: a scope selected on its own (a "sheet"),
  // loop or not. Tabbed the same way a turn is.
  const scopeSelected = $derived(kind === "scope" || kind === "loop");
  const live = $derived(snapshot.run.status === "running");

  function watcherPip(row: TurnRow): PipState {
    return !row.ended ? "running" : row.error ? "failed" : "ended";
  }

  const watcherRows = $derived<WatcherRow[]>(
    agentDefinition?.supervisors.map((watcher) => {
      const watcherTurn = latestWatcherTurn(scope, watcher.session);
      if (!watcherTurn) {
        return {
          session: watcher.session,
          role: watcher.role,
          pip: "not-yet",
          verdict: "not yet run",
          result: "No watcher turn has been recorded in this scope.",
        };
      }
      return {
        session: watcher.session,
        role: watcher.role,
        pip: watcherPip(watcherTurn),
        verdict: turnOutcome(watcherTurn),
        result: watcherTurn.result || watcherTurn.error || "No completed watcher result has been recorded.",
      };
    }) ?? [],
  );

  type ScopeTabID = "overview" | "context" | "decisions";
  const scopeTabs = $derived.by(() => {
    const tabs: { id: ScopeTabID; label: string }[] = [
      { id: "overview", label: "Overview" },
      { id: "context", label: "Context" },
    ];
    if (scope?.loop && (scope.decisions?.length ?? 0) > 0) tabs.push({ id: "decisions", label: "Decisions" });
    return tabs;
  });
  // A plain-string derived, not `scope?.key` read directly in the effect
  // below: `scope` is a fresh object every snapshot update even when its key
  // is unchanged, and an effect that reads it would reset the open tab on
  // every rerender rather than only when the selected scope actually changes.
  const scopeKey = $derived(scope?.key ?? "");
  let scopeActive = $state<ScopeTabID>("overview");
  $effect(() => {
    // A freshly selected scope re-opens on Overview, not whatever tab a
    // previous scope left showing.
    scopeKey;
    scopeActive = "overview";
  });

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

<aside class="detail" class:maximized style:width={maximized ? undefined : `${width}px`}>
  <header class="detail-head">
    <div class="title-row">
      {#if kind === "agent call"}<BotIcon size={16} />
      {:else if kind === "command"}<TerminalIcon size={16} />
      {:else if kind === "service"}<ServerIcon size={16} />
      {:else if kind === "interview"}<InterviewIcon size={16} />
      {:else if kind === "watcher"}<EyeIcon size={16} />
      {:else if kind === "loop"}<RepeatIcon size={16} />
      {:else}<BracesIcon size={16} />{/if}
      <h2>{title}</h2>
      <Badge variant="outline">{kind}</Badge>
      <span class="spacer"></span>
      {#if selection?.kind !== "service"}<Pip state={pipState} />{/if}
      {#if onmaximize}
        <button
          type="button"
          class="maximize"
          aria-label={maximized ? "Restore the detail pane" : "Maximize the detail pane"}
          title={`${maximized ? "Restore" : "Maximize"} (\\)`}
          onclick={onmaximize}
        >
          {#if maximized}<MinimizeIcon size={16} />{:else}<MaximizeIcon size={16} />{/if}
        </button>
      {/if}
    </div>
    {#if scope}
      <div class="placement" title={`${scope.key || "root"}${session ? ` · ${session.id}` : ""}`}>
        {scope.key || "root"}{session ? ` · ${session.id}` : ""}
      </div>
    {/if}
  </header>

  {#if scopeSelected}
    <TabBar tabs={scopeTabs} active={scopeActive} onselect={(id) => (scopeActive = id as ScopeTabID)} />
  {/if}

  {#if turn && (selection?.kind === "node" || selection?.kind === "watcher")}
    <SessionDetail
      {snapshot}
      {observation}
      {revision}
      {turn}
      definition={agentDefinition}
      watchers={selection?.kind === "node" ? watcherRows : []}
      {live}
      onsteer={(message) =>
        onsteer?.({ run: snapshot.run.id, session: turn.session, message }) ??
        Promise.resolve({ ok: false, message: "Steering is unavailable." })}
      onstop={onstop ? () => onstop(turn) : undefined}
      onscope={onselectscope}
    />
  {:else}
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
        <div class="section-title">Outcome</div>
        <p class="empty-selection">No watcher turn has been recorded in this scope.</p>
      </section>
      <section>
        <div class="section-title">Instruction</div>
        <p>{selection.supervisor.instruction}</p>
      </section>
      <section>
        <div class="section-title">Session</div>
        <dl><dt>Role</dt><dd>{selection.supervisor.role}</dd><dt>Source</dt><dd><code>{selection.supervisor.file}:{selection.supervisor.line}</code></dd></dl>
      </section>
    {:else if selection.kind === "service" && !command}
      <section>
        <div class="section-title">Declared service</div>
        <p>This service belongs to <code>{scope?.key || "root"}</code>. Its process state is runtime evidence, not part of this static declaration.</p>
        <dl>
          <dt>Source</dt><dd><code>{selection.service.file}:{selection.service.line}</code></dd>
        </dl>
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
      {@const failed = !command.interrupted && command.ended > 0 && command.exit_code !== 0}
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
      {#snippet stdout()}
        <section class="command-output">
          <Payload label="stdout" text={command.stdout || "No stdout was recorded."} anchor="bottom" />
          {#if command.stdout_file}
            <p class="output-reference">Complete output <code>{command.stdout_file}</code></p>
          {:else}
            <p class="help">No complete-output reference was recorded.</p>
          {/if}
        </section>
      {/snippet}
      {#snippet stderr()}
        <section class="command-output">
          <Payload
            label="stderr"
            text={command.stderr || "No stderr was recorded."}
            anchor="bottom"
            tone={command.exit_code !== 0 ? "error" : undefined}
          />
          {#if command.stderr_file}
            <p class="output-reference">Complete output <code>{command.stderr_file}</code></p>
          {:else}
            <p class="help">No complete-output reference was recorded.</p>
          {/if}
        </section>
      {/snippet}
      {#if failed}
        {@render stderr()}
        {@render stdout()}
      {:else}
        {@render stdout()}
        {@render stderr()}
      {/if}
    {:else if scope}
      {#if scopeActive === "overview"}
        {#if scope.task !== undefined}
          <section>
            <div class="section-title">Assignment</div>
            <Payload label="assignment" text={taskDescription(scope.task)} prose maxHeight={320} />
          </section>
        {/if}
        <section>
          <div class="section-title">Status</div>
          <dl>
            <dt>Status</dt><dd>{scope.status}</dd>
            <dt>Elapsed</dt><dd>{duration(scope.began, scope.ended)}</dd>
          </dl>
        </section>
        {#if scope.error}
          <div class="failure"><strong>Scope failed</strong><p>{scope.error}</p></div>
        {/if}
        <section>
          <div class="section-title">Sessions</div>
          {#each Object.values(snapshot.sessions).filter((row) => row.scope === scope.key) as row (row.id)}
            <div class="session-row"><Pip state={Object.values(snapshot.turns).some((turnRow) => turnRow.session === row.id && turnRow.ended === 0) ? "running" : "ended"} /><code>{row.id}</code><span>{row.model}</span></div>
          {/each}
        </section>
      {:else if scopeActive === "context"}
        <ScopeContext {snapshot} scopeKey={scope.key} onscope={onselectscope} />
      {:else if scopeActive === "decisions"}
        {#each [...scope.decisions].sort((left, right) => right.seq - left.seq) as decision (decision.seq)}
          <section class="decision-row">
            <div class="section-title">Decision {decision.seq}</div>
            <Payload label="decision" text={text(decision.body)} maxHeight={260} />
          </section>
        {/each}
      {/if}
    {/if}

    {#if feedback}
      <p class="feedback" class:error={!feedback.ok} role="status">{feedback.message}</p>
    {/if}
  </div>

  {#if liveLoop && scope}
    <footer class="detail-foot">
      <Textarea rows={2} bind:value={loopMessage} placeholder="Message the loop planner…" />
      <div class="footer-row"><Button variant="outline" size="sm" disabled={pending} onclick={() => act(() => onloop?.({ run: snapshot.run.id, scope: scope.key, message: "", wrap_up: true }) ?? Promise.resolve({ ok: false, message: "Loop control is unavailable." }))}>Wrap up</Button><span>Read at the planner’s next decision.</span><Button size="sm" disabled={pending || !loopMessage.trim()} onclick={() => act(() => onloop?.({ run: snapshot.run.id, scope: scope.key, message: loopMessage, wrap_up: false }) ?? Promise.resolve({ ok: false, message: "Loop control is unavailable." }), () => (loopMessage = ""))}>Send</Button></div>
    </footer>
  {:else if snapshot.run.status !== "running" && !turn}
    <footer class="recorded">This run has ended. Steering, answers, and stop controls are not offered on a record.</footer>
  {/if}
  {/if}
</aside>

<style>
  .detail { display: flex; min-width: 340px; flex-shrink: 0; flex-direction: column; overflow: hidden; color: var(--card-foreground); background: var(--card); border-left: 1px solid var(--map-line); }
  .detail.maximized { width: 100%; min-width: 0; border-left: none; }
  .detail-head { display: flex; flex-direction: column; gap: 8px; padding: 14px 16px 12px; background: color-mix(in oklch, var(--muted) 60%, var(--card)); border-bottom: 1px solid var(--border); }
  .title-row { display: flex; align-items: center; gap: 8px; }
  h2 { margin: 0; overflow: hidden; font-size: 16px; line-height: 24px; letter-spacing: -.01em; text-overflow: ellipsis; white-space: nowrap; }
  .spacer { flex: 1; }
  .placement, .help, .recorded { color: var(--status-muted); font-size: 13px; line-height: 18px; }
  .placement { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  code { font-family: var(--font-mono); }
  .detail-body { display: flex; min-height: 0; flex: 1; flex-direction: column; gap: 16px; padding: 14px 16px; overflow-y: auto; overflow-x: hidden; }
  section { display: flex; flex-direction: column; gap: 8px; }
  section p { margin: 0; font-size: 14px; line-height: 20px; white-space: pre-wrap; }
  .section-title { padding-bottom: 6px; font-size: 13px; font-weight: 600; line-height: 18px; border-bottom: 1px solid var(--border); }
  dl { display: grid; grid-template-columns: 92px minmax(0, 1fr); gap: 6px 12px; margin: 0; font-size: 13px; line-height: 18px; }
  dt { color: var(--status-muted); }
  dd { min-width: 0; margin: 0; overflow-wrap: anywhere; }
  .session-row { display: flex; align-items: center; gap: 8px; min-width: 0; font-size: 13px; }
  .session-row code { min-width: 0; flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .session-row span { color: var(--status-muted); }
  .failure { padding: 10px 12px; color: var(--destructive); font-size: 13px; border: 1px solid color-mix(in oklch, var(--destructive) 45%, transparent); border-radius: 7px; }
  .not-observed { padding: 10px 12px; border: 1px dashed var(--map-line); border-radius: 7px; }
  .failure p { margin: 3px 0 0; color: var(--status-muted); }
  .exchanges article { padding: 8px 10px; font-size: 13px; border-left: 2px solid var(--map-line-strong); }
  .exchanges article.waiting { background: var(--map-live-soft); border-left-color: var(--status-live); border-radius: 0 6px 6px 0; }
  .exchanges article.answer { background: var(--muted); border: 1px solid var(--border); border-radius: 6px; }
  .exchanges article strong { font-size: 13px; }
  label { font-size: 13px; font-weight: 600; }
  .action-row, .footer-row { display: flex; align-items: center; gap: 8px; }
  .output-reference { margin: 0; color: var(--status-muted); font-size: 12px; line-height: 18px; overflow-wrap: anywhere; }
  .decision-row { gap: 6px; }
  .feedback { margin: 0; padding: 7px 9px; color: var(--foreground); font-size: 13px; background: var(--map-live-soft); border-radius: 6px; }
  .feedback.error { color: var(--destructive); background: color-mix(in oklch, var(--destructive) 10%, transparent); }
  .detail-foot { display: flex; flex-direction: column; gap: 8px; padding: 12px 16px; background: color-mix(in oklch, var(--muted) 60%, var(--card)); border-top: 1px solid var(--border); }
  .footer-row span { flex: 1; color: var(--status-muted); font-size: 13px; line-height: 18px; }
  .recorded { padding: 12px 16px; color: var(--foreground); background: color-mix(in oklch, var(--muted) 60%, var(--card)); border-top: 1px solid var(--border); }
  .maximize { display: inline-flex; width: 28px; height: 28px; align-items: center; justify-content: center; color: var(--foreground); cursor: pointer; background: transparent; border: 0; border-radius: 6px; }
  .maximize:hover { background: var(--muted); }
  .maximize:focus-visible { outline: 2px solid var(--status-live); }
</style>
