# Interactive design study validation

2026-09-14. Conceptual UI with explicitly illustrative data, not a generated graph or a live Gimble run.

Source is the task-owned visualization fragment `gimble-workflow-view.html` in `/Users/tyler/.codex/visualizations/2026/09/14/01a0a08c-3dc0-7912-a88f-90b8650231b4/`. A temporary local preview wrapped it in a sandboxed iframe. Inspected with the Chrome CUA surface; no application build/test or live agent run was performed.

Observed interaction checks:

- Selecting task 3 and Run checks displays exit 1 / 45s and its recorded failure while the global run remains Running and identifies task 4 as the continuation.
- Selecting task 3's failed-check row in Timeline selects that instance in the inspector and retains it when switching to Map.
- Switching to Program removes observed instance counts/timing states, uses the task template in command details, and disables Timeline.
- Selecting the current coder and opening the transcript preserves map selection while showing an illustrative supervisor message.
- Desktop and 360px previews were inspected. Narrow connectors were moved out of headers/cards; timeline tick labels were corrected to the actual shared scale. A 320px timeline inspection showed legible 0/17/34 minute ticks and wrapped row labels.
- Browser error log was empty after the interaction checks. Temporary viewport override was reset.

Limits: scope focus/expansion, stable-layout behavior under real streaming events, large-loop navigation, exact source correlation, and actual runtime telemetry are described in the brief but not implemented in this study. The timeline is an illustrative selection of operation intervals, not a complete event history. Product claims require implementation and real run evidence.
