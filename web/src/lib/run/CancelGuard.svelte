<script lang="ts">
  import SquareIcon from "@lucide/svelte/icons/square";
  import XIcon from "@lucide/svelte/icons/x";
  import * as AlertDialog from "#lib/components/ui/alert-dialog/index.js";
  import type { RunRow, TurnRow } from "../observation/index.js";

  let {
    open = $bindable(false),
    run,
    turn,
    task = "task.3",
    openScopes = 3,
    runningTurns = 1,
    supervisors = 2,
    onstopturn,
    oncancelrun,
  }: {
    open?: boolean;
    run: RunRow;
    turn: TurnRow;
    task?: string;
    openScopes?: number;
    runningTurns?: number;
    supervisors?: number;
    onstopturn?: (turn: TurnRow) => void;
    oncancelrun?: (run: RunRow) => void;
  } = $props();

  function stopTurn() {
    onstopturn?.(turn);
    open = false;
  }

  function cancelRun() {
    oncancelrun?.(run);
    open = false;
  }
</script>

<AlertDialog.Root bind:open>
  <AlertDialog.Content class="guard-content">
    <AlertDialog.Header>
      <AlertDialog.Title>Cancel the whole run?</AlertDialog.Title>
      <AlertDialog.Description>
        {run.name} <span class="mono">{run.id}</span> stops now. That ends
        <span class="mono">{turn.session} / {turn.id.slice(turn.id.lastIndexOf("/") + 1)}</span>,
        the planner and supervisor sessions. The record so far is kept and marked cancelled.
      </AlertDialog.Description>
    </AlertDialog.Header>

    <div class="choices">
      <section>
        <h3><SquareIcon size={14} />Stop turn</h3>
        <p>
          Ends only <span class="mono">{turn.session} / {turn.id.slice(turn.id.lastIndexOf("/") + 1)}</span>
          in {task}. The run continues; the workflow decides what to do with the interrupted turn.
        </p>
      </section>
      <section class="danger">
        <h3><XIcon size={14} />Cancel run</h3>
        <p>
          Ends every scope, session and command in this run: {openScopes} scopes open,
          {runningTurns} turn running, {supervisors} supervisors watching. Nothing continues.
        </p>
      </section>
    </div>

    <AlertDialog.Footer>
      <AlertDialog.Cancel>Keep running</AlertDialog.Cancel>
      <AlertDialog.Action variant="outline" onclick={stopTurn}>
        <SquareIcon data-icon="inline-start" />
        Stop turn instead
      </AlertDialog.Action>
      <AlertDialog.Action variant="destructive" onclick={cancelRun}>Cancel run</AlertDialog.Action>
    </AlertDialog.Footer>
  </AlertDialog.Content>
</AlertDialog.Root>

<style>
  :global(.guard-content) {
    max-width: 540px;
    gap: 16px;
  }

  .mono {
    font-family: var(--font-mono);
  }

  .choices {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 12px;
  }

  section {
    box-sizing: border-box;
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding: 10px 12px;
    border: 1px solid var(--border);
    border-radius: calc(var(--radius) - 2px);
  }

  section.danger {
    border-color: color-mix(in oklch, var(--destructive) 45%, transparent);
  }

  h3 {
    display: flex;
    align-items: center;
    gap: 6px;
    margin: 0;
    font-size: 13px;
    font-weight: 500;
    line-height: 18px;
  }

  .danger h3 {
    color: var(--destructive);
  }

  p {
    margin: 0;
    color: var(--status-muted);
    font-size: 13px;
    line-height: 18px;
  }

  @media (max-width: 560px) {
    .choices {
      grid-template-columns: 1fr;
    }
  }
</style>
