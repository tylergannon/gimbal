<script module lang="ts">
  import type {
    CommandRow,
    InterviewRow,
    RunSnapshot,
    ScopeRow,
    SessionRow,
    TurnRow,
  } from "../observation/index.js";
  import type { AgentCall, Command, Interview, PromiseLoop } from "../workflow/types.js";

  export type TurnSelection = {
    kind: "turn";
    operation: AgentCall;
    turn: TurnRow;
    scope: ScopeRow;
    session: SessionRow;
  };

  export type CommandSelection = {
    kind: "command";
    operation: Command;
    command: CommandRow;
    scope: ScopeRow;
  };

  export type ScopeSelection = {
    kind: "scope";
    scope: ScopeRow;
  };

  export type LoopSelection = {
    kind: "loop";
    operation: PromiseLoop;
    scope: ScopeRow;
  };

  export type InterviewSelection = {
    kind: "interview";
    operation: Interview;
    interview: InterviewRow;
    scope: ScopeRow;
    session: SessionRow;
  };

  export type DetailSelection =
    | TurnSelection
    | CommandSelection
    | ScopeSelection
    | LoopSelection
    | InterviewSelection;

  export type DisclosureEvent = {
    name: string;
    open: boolean;
  };
</script>

<script lang="ts">
  import BotIcon from "@lucide/svelte/icons/bot";
  import BoxIcon from "@lucide/svelte/icons/box";
  import BracesIcon from "@lucide/svelte/icons/braces";
  import ChevronRightIcon from "@lucide/svelte/icons/chevron-right";
  import MessageCircleQuestionIcon from "@lucide/svelte/icons/message-circle-question-mark";
  import RepeatIcon from "@lucide/svelte/icons/repeat-2";
  import TerminalIcon from "@lucide/svelte/icons/terminal";
  import { Badge } from "#lib/components/ui/badge/index.js";
  import { Button } from "#lib/components/ui/button/index.js";
  import { ScrollArea } from "#lib/components/ui/scroll-area/index.js";
  import * as Tabs from "#lib/components/ui/tabs/index.js";
  import { Textarea } from "#lib/components/ui/textarea/index.js";
  import type { JSONObject } from "../sessionstate/index.js";
  import Pip from "./Pip.svelte";

  let {
    selection,
    snapshot,
    onsteer,
    onwrapup,
    onanswer,
    onendinterview,
    onopeninterview,
    onopentranscript,
    ondisclosurechange,
  }: {
    selection: DetailSelection;
    snapshot: RunSnapshot;
    onsteer?: (payload: { turn: TurnRow; message: string }) => void;
    onwrapup?: (payload: { scope: ScopeRow; message: string }) => void;
    onanswer?: (payload: { interview: InterviewRow; answer: string }) => void;
    onendinterview?: (interview: InterviewRow) => void;
    onopeninterview?: (interview: InterviewRow) => void;
    onopentranscript?: (turn: TurnRow) => void;
    ondisclosurechange?: (event: DisclosureEvent) => void;
  } = $props();

  let steerDraft = $state("");
  let wrapDraft = $state("Wrap this up: end dispatch at this decision.");
  let answerDraft = $state("");

  const title = $derived.by(() => {
    if (selection.kind === "turn") return selection.session.name;
    if (selection.kind === "command") return selection.command.name;
    if (selection.kind === "interview") return selection.interview.name;
    return selection.scope.name;
  });

  const source = $derived.by(() => {
    if (selection.kind === "scope") return undefined;
    return `${selection.operation.file}:${selection.operation.line}`;
  });

  const scopeSessions = $derived(
    Object.values(snapshot.sessions).filter((session) => session.scope === selection.scope.key),
  );

  const exchangeRows = $derived.by(() => {
    if (selection.kind !== "interview") return [];
    return Object.values(snapshot.interviews)
      .filter((row) => row.session === selection.interview.session)
      .sort((left, right) => left.asked - right.asked);
  });

  const otherPendingInterview = $derived.by(() => {
    if (selection.kind !== "interview") return undefined;
    return Object.values(snapshot.interviews).find(
      (row) => row.status === "pending" && row.question_id !== selection.interview.question_id,
    );
  });

  const activityRows = $derived.by(() => {
    if (selection.kind !== "turn") return [];
    const transcript = snapshot.transcripts[selection.turn.id];
    const messages = transcript ? Object.values(transcript.snapshot.state.message).flat() : [];
    const rows = messages.flatMap((message: JSONObject) => {
      if (message.type === "shell") {
        return [
          {
            time: clock(message.time?.created ?? selection.turn.started),
            action: "Bash",
            detail: message.command ?? "shell command",
            running: !message.time?.completed,
          },
        ];
      }
      if (message.type !== "assistant") return [];
      return (message.content ?? [])
        .filter((part: JSONObject) => part.type === "tool")
        .map((part: JSONObject) => ({
          time: clock(part.time?.created ?? message.time?.created ?? selection.turn.started),
          action: part.name ?? "Tool",
          detail: toolDetail(part),
          running: !["completed", "error"].includes(part.state?.status),
        }));
    });
    if (rows.length > 0) return rows;
    return (snapshot.model_calls[selection.turn.id] ?? []).map((call) => ({
      time: clock(call.started),
      action: "Model",
      detail: `${call.model} · ${call.input} in / ${call.output} out`,
      running: call.ended === 0,
    }));
  });

  function readable(value: unknown) {
    if (typeof value === "string") return value;
    return JSON.stringify(value, null, 2);
  }

  function toolDetail(part: JSONObject) {
    const input = part.state?.input;
    if (typeof input === "string") {
      const compact = input.replace(/\s+/g, " ").trim();
      return compact.length > 90 ? `${compact.slice(0, 87)}…` : compact || part.state?.status;
    }
    if (input !== undefined) {
      const compact = JSON.stringify(input);
      return compact.length > 90 ? `${compact.slice(0, 87)}…` : compact;
    }
    return part.state?.status ?? "pending";
  }

  function valueText(value: ScopeRow["values"][string]) {
    if (value.value !== undefined) return readable(value.value);
    if (value.artifact) return `${value.artifact.preview}\n${value.artifact.file}`;
    return "—";
  }

  function shortID(value: string) {
    return value.slice(value.lastIndexOf("/") + 1).replace(".", " ");
  }

  function duration(milliseconds: number) {
    const totalSeconds = Math.max(0, Math.floor(milliseconds / 1000));
    const minutes = Math.floor(totalSeconds / 60);
    const seconds = totalSeconds % 60;
    return minutes > 0 ? `${minutes}m ${seconds.toString().padStart(2, "0")}s` : `${seconds}s`;
  }

  function clock(milliseconds: number) {
    return new Date(milliseconds).toLocaleTimeString([], {
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
      hour12: false,
    });
  }

  function toggle(name: string, event: Event) {
    ondisclosurechange?.({ name, open: (event.currentTarget as HTMLDetailsElement).open });
  }

  function sendSteer() {
    if (selection.kind !== "turn" || !steerDraft.trim()) return;
    onsteer?.({ turn: selection.turn, message: steerDraft.trim() });
    steerDraft = "";
  }

  function sendWrapUp() {
    if (selection.kind !== "loop" || !wrapDraft.trim()) return;
    onwrapup?.({ scope: selection.scope, message: wrapDraft.trim() });
  }

  function sendAnswer() {
    if (selection.kind !== "interview" || !answerDraft.trim()) return;
    onanswer?.({ interview: selection.interview, answer: answerDraft.trim() });
    answerDraft = "";
  }

  function answerKeydown(event: KeyboardEvent) {
    if (event.key === "Enter" && (event.metaKey || event.ctrlKey)) {
      event.preventDefault();
      sendAnswer();
    }
  }
</script>

<aside class="pane" aria-label={`${title} details`}>
  <header class="pane-header">
    <div class="title-row">
      {#if selection.kind === "turn"}
        <BotIcon size={16} />
      {:else if selection.kind === "command"}
        <TerminalIcon size={16} />
      {:else if selection.kind === "interview"}
        <MessageCircleQuestionIcon size={16} />
      {:else if selection.kind === "loop"}
        <RepeatIcon size={16} />
      {:else}
        <BoxIcon size={16} />
      {/if}
      <h2>{title}</h2>
      <Badge variant="outline">{selection.kind === "turn" ? "agent call" : selection.kind}</Badge>
      {#if selection.kind === "interview" && selection.interview.status === "pending"}
        <Badge class="waiting-badge">Waiting for you</Badge>
      {/if}
    </div>

    {#if selection.kind === "turn"}
      <div class="meta-line">
        <span>{selection.scope.name} {selection.scope.key.split(".").at(-1)}</span>
        <span>·</span>
        <Pip state={selection.turn.ended ? "ended" : "running"} />
        <span>{selection.turn.ended ? "ended" : "running"}</span>
        <span>·</span>
        <span class="mono">{duration(selection.turn.duration)}</span>
      </div>
    {:else if selection.kind === "interview"}
      <div class="meta-line">
        <span>scope <span class="mono">{selection.scope.key}</span></span>
        <span>·</span>
        <span>session <span class="mono">{selection.session.name}.1</span></span>
        {#if source}<span>· <span class="source">{source}</span></span>{/if}
      </div>
      <Tabs.Root value="exchanges" class="interview-tabs">
        <Tabs.List variant="line" class="tab-list">
          <Tabs.Trigger value="exchanges">Exchanges</Tabs.Trigger>
          <Tabs.Trigger value="definition">Definition</Tabs.Trigger>
        </Tabs.List>
        <Tabs.Content value="definition" class="definition-tab">
          <code>{source}</code>
        </Tabs.Content>
      </Tabs.Root>
    {:else}
      <div class="meta-line">
        <span class="mono">{selection.scope.key || "root"}</span>
        <span>·</span>
        <Pip state={selection.scope.error ? "failed" : selection.scope.status === "running" ? "running" : "ended"} />
        <span>{selection.scope.error ? "failed" : selection.scope.status}</span>
        {#if source}<span>· <span class="source">{source}</span></span>{/if}
      </div>
    {/if}
  </header>

  <ScrollArea class="body">
    <div class="body-inner">
      {#if selection.kind === "turn"}
        <section class="section">
          <div class="section-title"><strong>Assignment</strong><span>from sprint-planning</span></div>
          <p>{selection.turn.prompt}</p>
        </section>

        <section class="section">
          <div class="section-title">
            <strong>Activity <span>· {activityRows.length} recorded events</span></strong>
            <button type="button" onclick={() => onopentranscript?.(selection.turn)}>
              open transcript
            </button>
          </div>
          {#each activityRows as item}
            <div class="activity">
              <time>{item.time}</time>
              <span class="activity-text"><span>{item.action}</span> {item.detail}</span>
              {#if item.running}<Pip state="running" />{/if}
            </div>
          {:else}
            <p class="muted">No activity is present in this turn's transcript.</p>
          {/each}
        </section>

        <div class="disclosures">
          <details ontoggle={(event) => toggle("Prompt sent", event)}>
            <summary><ChevronRightIcon />Prompt sent<span>{selection.turn.prompt.length} chars</span></summary>
            <pre>{selection.turn.prompt}</pre>
          </details>
          <details ontoggle={(event) => toggle("Watchers", event)}>
            <summary><ChevronRightIcon />Watchers<span>{selection.operation.supervisors.length}</span></summary>
            {#each selection.operation.supervisors as watcher}
              <div class="detail-row"><strong>{watcher.session}</strong><span>{watcher.instruction}</span></div>
            {:else}
              <p class="muted">No watchers.</p>
            {/each}
          </details>
          <details ontoggle={(event) => toggle("Session", event)}>
            <summary><ChevronRightIcon />Session<span>{selection.session.name} · {shortID(selection.turn.id)}</span></summary>
            <dl>
              <dt>Adapter</dt><dd>{selection.session.adapter}</dd>
              <dt>Model</dt><dd>{selection.session.model}</dd>
              <dt>Session</dt><dd class="mono">{selection.session.id}</dd>
            </dl>
          </details>
          <details ontoggle={(event) => toggle("Usage", event)}>
            <summary><ChevronRightIcon />Usage<span>{snapshot.turn_usage[selection.turn.id]?.[selection.session.model]?.input ?? 0} in</span></summary>
            <dl>
              {#each Object.entries(snapshot.turn_usage[selection.turn.id] ?? {}) as [model, usage]}
                <dt>{model}</dt><dd>{usage.input} in · {usage.output} out</dd>
              {/each}
            </dl>
          </details>
          <details ontoggle={(event) => toggle("Definition", event)}>
            <summary><ChevronRightIcon />Definition<span class="mono">{source}</span></summary>
            <p>{selection.operation.prompt}</p>
          </details>
        </div>
      {:else if selection.kind === "command"}
        <section class="section">
          <div class="section-title"><strong>Assignment</strong><span>{selection.command.workdir}</span></div>
          <pre>{selection.command.command} {selection.command.args.join(" ")}</pre>
        </section>
        <section class="section">
          <div class="section-title"><strong>Activity</strong><span>{duration(selection.command.duration)}</span></div>
          <div class="command-result" class:failed={selection.command.exit_code !== 0 || !!selection.command.error}>
            <Pip state={selection.command.exit_code !== 0 || selection.command.error ? "failed" : "ended"} />
            <strong>exit {selection.command.exit_code}</strong>
            <span>{selection.command.interrupted ? "interrupted" : "finished"}</span>
          </div>
        </section>
        <div class="disclosures">
          <details open ontoggle={(event) => toggle("stdout tail", event)}>
            <summary><ChevronRightIcon />stdout tail<span class="mono">{selection.command.stdout_file}</span></summary>
            <pre>{selection.command.stdout || "No stdout."}</pre>
          </details>
          <details open ontoggle={(event) => toggle("stderr tail", event)}>
            <summary><ChevronRightIcon />stderr tail<span class="mono">{selection.command.stderr_file}</span></summary>
            <pre>{selection.command.stderr || "No stderr."}</pre>
          </details>
          <details ontoggle={(event) => toggle("Definition", event)}>
            <summary><ChevronRightIcon />Definition<span class="mono">{source}</span></summary>
            <p>Command step <strong>{selection.operation.name}</strong>.</p>
          </details>
        </div>
      {:else if selection.kind === "scope"}
        <section class="section">
          <div class="section-title"><strong>Values written</strong><span>{Object.keys(selection.scope.values).length}</span></div>
          <div class="value-list">
            {#each Object.entries(selection.scope.values) as [key, value]}
              <div class="value-row">
                <div><BracesIcon size={13} /><strong>{key}</strong></div>
                <pre>{valueText(value)}</pre>
              </div>
            {:else}
              <p class="muted">No values written in this scope.</p>
            {/each}
          </div>
        </section>
        <section class="section">
          <div class="section-title"><strong>Sessions</strong><span>{scopeSessions.length}</span></div>
          {#each scopeSessions as session}
            <div class="session-row">
              <BotIcon size={14} />
              <strong>{session.name}</strong>
              <span>{session.adapter} · {session.model}</span>
            </div>
          {:else}
            <p class="muted">No sessions were declared in this scope.</p>
          {/each}
        </section>
      {:else if selection.kind === "loop"}
        <section class="section">
          <div class="section-title"><strong>Backlog</strong><span>{selection.scope.decisions.length} decisions</span></div>
          <div class="backlog">
            {#each selection.scope.decisions as decision, index}
              <div class="backlog-row">
                <span class="decision-number">{index + 1}</span>
                <div><strong>Decision {decision.seq}</strong><pre>{readable(decision.body)}</pre></div>
              </div>
            {/each}
          </div>
        </section>
        <section class="section">
          <div class="section-title"><strong>Definition</strong><span class="mono">{source}</span></div>
          <dl>
            <dt>Planner</dt><dd>{selection.operation.planner}</dd>
            <dt>Supervisors</dt><dd>{selection.operation.supervisors.length}</dd>
          </dl>
        </section>
      {:else}
        <div class="exchanges">
          {#each exchangeRows as row, index}
            <div class:live={row.status === "pending"} class="question">
              <div class="exchange-title">
                <strong>{row.status === "pending" ? `Question ${index + 1} · waiting` : `Question ${index + 1}`}</strong>
                <time>{clock(row.asked)}</time>
              </div>
              <p>{row.question}</p>
            </div>
            {#if row.status === "answered"}
              <div class="answer">
                <div class="exchange-title"><strong>You</strong><time>{clock(row.answered)}</time></div>
                <p>{row.answer}</p>
              </div>
            {/if}
          {/each}
        </div>

        {#if selection.interview.status === "pending"}
          <section class="answer-form">
            <label for={`answer-${selection.interview.question_id}`}>Your answer</label>
            <Textarea
              id={`answer-${selection.interview.question_id}`}
              rows={4}
              placeholder="Answer here. Send with ⌘⏎"
              bind:value={answerDraft}
              onkeydown={answerKeydown}
            />
            <div class="answer-actions">
              <Button size="sm" disabled={!answerDraft.trim()} onclick={sendAnswer}>Answer</Button>
              <Button size="sm" variant="outline" onclick={() => onendinterview?.(selection.interview)}>
                End interview
              </Button>
              <span>draft kept if the question changes</span>
            </div>
            <p class="muted">End interview sends no answer. The interviewer stops asking and the workflow continues with the exchanges so far.</p>
          </section>
        {/if}
      {/if}
    </div>
  </ScrollArea>

  {#if selection.kind === "turn"}
    <footer class="footer stacked">
      <Textarea rows={2} placeholder="Steer this turn…" bind:value={steerDraft} />
      <div>
        <span>Lands before the next tool call. Dropped if the turn ends first.</span>
        <Button size="sm" disabled={!steerDraft.trim()} onclick={sendSteer}>Steer</Button>
      </div>
    </footer>
  {:else if selection.kind === "loop"}
    <footer class="footer stacked">
      <label for={`wrap-${selection.scope.key}`}>Tell the planner to wrap up</label>
      <Textarea id={`wrap-${selection.scope.key}`} rows={2} bind:value={wrapDraft} />
      <div>
        <span>The planner records the final decision.</span>
        <Button size="sm" disabled={!wrapDraft.trim()} onclick={sendWrapUp}>Wrap up loop</Button>
      </div>
    </footer>
  {:else if selection.kind === "interview" && otherPendingInterview}
    <footer class="footer waiting-footer">
      <Pip state="waiting" />
      <span>Also waiting: <span class="mono">{otherPendingInterview.scope}</span> · “{otherPendingInterview.question}”</span>
      <Button size="xs" variant="outline" onclick={() => onopeninterview?.(otherPendingInterview)}>Open</Button>
    </footer>
  {/if}
</aside>

<style>
  .pane {
    display: flex;
    width: 400px;
    min-width: 0;
    height: 100%;
    min-height: 560px;
    flex-direction: column;
    overflow: hidden;
    color: var(--card-foreground);
    background: var(--card);
    border-left: 1px solid var(--map-line);
  }

  .pane-header {
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding: 14px 16px 12px;
    background: color-mix(in oklch, var(--muted) 60%, var(--card));
    border-bottom: 1px solid var(--border);
  }

  .title-row,
  .meta-line,
  .section-title,
  .activity,
  .command-result,
  .session-row,
  .exchange-title,
  .answer-actions,
  .footer > div,
  .waiting-footer {
    display: flex;
    align-items: center;
  }

  .title-row {
    gap: 8px;
  }

  h2 {
    margin: 0;
    font-size: 16px;
    font-weight: 600;
    line-height: 24px;
    letter-spacing: -0.01em;
  }

  :global(.waiting-badge) {
    margin-left: auto;
  }

  .meta-line {
    flex-wrap: wrap;
    gap: 6px;
    color: var(--status-muted);
    font-size: 13px;
    line-height: 18px;
  }

  .mono,
  pre,
  code,
  time {
    font-family: var(--font-mono);
  }

  .source,
  .section-title button,
  .title-row + .meta-line .source {
    color: var(--status-live);
  }

  :global(.interview-tabs) {
    gap: 0;
    margin-top: 4px;
  }

  :global(.tab-list) {
    width: 100%;
  }

  :global(.definition-tab) {
    position: absolute;
    z-index: 2;
    box-sizing: border-box;
    width: calc(100% - 32px);
    padding: 12px;
    margin-top: 6px;
    background: var(--card);
    border: 1px solid var(--border);
    border-radius: calc(var(--radius) - 2px);
    box-shadow: var(--shadow-md);
  }

  :global(.body) {
    min-height: 0;
    flex: 1;
  }

  .body-inner {
    display: flex;
    flex-direction: column;
    gap: 16px;
    padding: 14px 16px 16px;
  }

  .section {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .section-title {
    justify-content: space-between;
    gap: 8px;
    padding-bottom: 6px;
    font-size: 13px;
    line-height: 18px;
    border-bottom: 1px solid var(--border);
  }

  .section-title strong {
    font-weight: 600;
  }

  .section-title span,
  .section-title strong span,
  .section-title button {
    color: var(--status-muted);
    font-weight: 400;
  }

  .section-title button {
    padding: 0;
    font: inherit;
    cursor: pointer;
    background: none;
    border: 0;
  }

  p {
    margin: 0;
    font-size: 14px;
    line-height: 20px;
  }

  .activity {
    min-width: 0;
    gap: 8px;
    font-size: 13px;
    line-height: 18px;
  }

  .activity time {
    width: 60px;
    flex-shrink: 0;
    color: var(--status-muted);
  }

  .activity-text {
    overflow: hidden;
    flex: 1;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .activity-text > span {
    color: var(--status-muted);
  }

  .disclosures {
    display: flex;
    flex-direction: column;
  }

  details {
    border-top: 1px solid var(--border);
  }

  details:last-child {
    border-bottom: 1px solid var(--border);
  }

  summary {
    display: flex;
    height: 40px;
    align-items: center;
    gap: 8px;
    font-size: 13px;
    font-weight: 500;
    line-height: 18px;
    cursor: pointer;
    list-style: none;
  }

  summary::-webkit-details-marker {
    display: none;
  }

  summary :global(svg) {
    flex-shrink: 0;
    color: var(--muted-foreground);
    transition: transform 120ms ease;
  }

  details[open] summary :global(svg) {
    transform: rotate(90deg);
  }

  summary span {
    overflow: hidden;
    margin-left: auto;
    color: var(--status-muted);
    font-weight: 400;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  details > pre,
  details > p,
  details > dl,
  .detail-row {
    margin: 0 0 10px 22px;
  }

  pre {
    overflow: auto;
    padding: 8px 10px;
    margin: 0;
    color: var(--foreground);
    font-size: 12px;
    line-height: 18px;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
    background: var(--muted);
    border: 1px solid var(--border);
    border-radius: calc(var(--radius) - 4px);
  }

  .detail-row {
    display: flex;
    flex-direction: column;
    gap: 3px;
    font-size: 13px;
    line-height: 18px;
  }

  dl {
    display: grid;
    grid-template-columns: 84px minmax(0, 1fr);
    gap: 6px 12px;
    margin: 0;
    font-size: 13px;
    line-height: 18px;
  }

  dt {
    color: var(--status-muted);
  }

  dd {
    min-width: 0;
    margin: 0;
    overflow-wrap: anywhere;
  }

  .command-result {
    gap: 8px;
    padding: 9px 10px;
    font-size: 13px;
    border: 1px solid var(--border);
    border-radius: calc(var(--radius) - 2px);
  }

  .command-result span:last-child {
    margin-left: auto;
    color: var(--status-muted);
  }

  .command-result.failed {
    border-color: color-mix(in oklch, var(--destructive) 45%, transparent);
  }

  .value-list,
  .backlog,
  .exchanges {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  .value-row {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .value-row > div,
  .session-row {
    display: flex;
    align-items: center;
    gap: 7px;
    font-size: 13px;
  }

  .value-row :global(svg) {
    color: var(--status-muted);
  }

  .session-row {
    padding: 8px 10px;
    border: 1px solid var(--border);
    border-radius: calc(var(--radius) - 2px);
  }

  .session-row span {
    overflow: hidden;
    margin-left: auto;
    color: var(--status-muted);
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .backlog-row {
    display: grid;
    grid-template-columns: 24px minmax(0, 1fr);
    gap: 8px;
  }

  .decision-number {
    display: inline-flex;
    width: 22px;
    height: 22px;
    align-items: center;
    justify-content: center;
    font-family: var(--font-mono);
    font-size: 11px;
    background: var(--muted);
    border-radius: 999px;
  }

  .backlog-row > div {
    display: flex;
    flex-direction: column;
    gap: 5px;
  }

  .backlog-row strong {
    font-size: 13px;
  }

  .question,
  .answer {
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding: 8px 10px;
    font-size: 13px;
    line-height: 18px;
  }

  .question {
    border-left: 2px solid var(--map-line-strong);
  }

  .question.live {
    background: var(--map-live-soft);
    border-left-color: var(--status-live);
    border-radius: 0 calc(var(--radius) - 4px) calc(var(--radius) - 4px) 0;
  }

  .answer {
    background: var(--muted);
    border: 1px solid var(--border);
    border-radius: calc(var(--radius) - 2px);
  }

  .exchange-title {
    justify-content: space-between;
    gap: 8px;
  }

  .exchange-title time {
    font-weight: 400;
  }

  .question p,
  .answer p {
    font-size: 13px;
    line-height: 18px;
  }

  .answer-form {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .answer-form label,
  .footer label {
    font-size: 13px;
    font-weight: 600;
    line-height: 18px;
  }

  .answer-form :global(textarea),
  .footer :global(textarea) {
    box-sizing: border-box;
    width: 100%;
    resize: none;
  }

  .answer-actions {
    gap: 8px;
  }

  .answer-actions span {
    margin-left: auto;
    color: var(--status-muted);
    font-size: 13px;
  }

  .muted,
  .footer span {
    color: var(--status-muted);
  }

  .muted {
    font-size: 13px;
    line-height: 18px;
  }

  .footer {
    box-sizing: border-box;
    padding: 12px 16px;
    background: color-mix(in oklch, var(--muted) 60%, var(--card));
    border-top: 1px solid var(--border);
  }

  .stacked {
    display: flex;
    flex-direction: column;
    align-items: stretch;
    gap: 8px;
  }

  .stacked > div {
    gap: 10px;
  }

  .stacked > div span {
    flex: 1;
    font-size: 13px;
    line-height: 18px;
  }

  .waiting-footer {
    gap: 8px;
  }

  .waiting-footer > span {
    min-width: 0;
    flex: 1;
    color: var(--foreground);
    font-size: 13px;
    line-height: 18px;
  }
</style>
