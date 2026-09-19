<script lang="ts">
  type PipState = "ended" | "running" | "failed" | "waiting" | "not-yet";

  let { state }: { state: PipState } = $props();
</script>

<span class="pip" data-state={state} aria-hidden="true"></span>

<style>
  .pip {
    position: relative;
    display: inline-block;
    width: 10px;
    height: 10px;
    box-sizing: border-box;
    flex-shrink: 0;
    border-radius: 9999px;
  }

  .pip[data-state="ended"] {
    background: var(--foreground);
  }

  .pip[data-state="running"] {
    border: 2px solid var(--status-running);
    border-top-color: transparent;
    animation: spin 1s linear infinite;
  }

  .pip[data-state="failed"] {
    background: var(--destructive);
  }

  .pip[data-state="failed"]::before,
  .pip[data-state="failed"]::after {
    position: absolute;
    top: 4px;
    right: 2px;
    left: 2px;
    height: 1.5px;
    background: var(--background);
    content: "";
    transform: rotate(45deg);
  }

  .pip[data-state="failed"]::after {
    transform: rotate(-45deg);
  }

  .pip[data-state="waiting"] {
    background: var(--background);
    box-shadow: inset 0 0 0 2px var(--status-live);
  }

  .pip[data-state="waiting"]::after {
    position: absolute;
    top: 3px;
    left: 3px;
    width: 4px;
    height: 4px;
    border-radius: 9999px;
    background: var(--status-live);
    content: "";
  }

  .pip[data-state="not-yet"] {
    border: 1.5px dashed var(--status-muted);
  }

  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }
</style>
