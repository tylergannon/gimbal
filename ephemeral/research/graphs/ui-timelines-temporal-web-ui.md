# Temporal Web UI source note

- Source: https://github.com/temporalio/documentation/blob/main/docs/web-ui.mdx
- Raw source used: https://raw.githubusercontent.com/temporalio/documentation/main/docs/web-ui.mdx
- Source type: official Temporal documentation repository
- Retrieved: 2026-09-14

## Relevant observations

1. The Workflows page is a searchable table of executions. The selected
   execution opens several related projections: History, Workers,
   Relationships, pending Activities/Nexus Operations, Queries, Metadata, and
   a call stack.
2. History has four modes: Timeline (chronological or reverse chronological
   events with a summary), All (every event), Compact (logical grouping of
   Activities, Signals, and Timers), and JSON (full history).
3. Clicking an event opens all details for that event; the entire event history
   can be downloaded as JSON.
4. Relationships shows parent and child executions as a hierarchy. This is a
   separate projection from the event timeline.
5. The page exposes top-level execution metadata (start/close/duration, run ID,
   workflow type, task queue, parent, and state transitions) before the detailed
   history.

## Transferable UI pattern

Start at a run list with useful filters, then open one run with a persistent
identity and switch projections without losing context. Offer a compact summary
for orientation, a chronological event list for audit, a grouped view for
human-scale debugging, and raw JSON for completeness. Keep relationship trees
and event order as separate views because they answer different questions.

## Limits for Gimble

Temporal's event history is an append-only execution record. Gimble's static
program graph is a possible-operation template and should not be presented as
if it were a complete history. A Compact view must remain reversible to the
underlying turns/events, and collapsed groups must retain counts and exceptional
children rather than silently dropping them.
