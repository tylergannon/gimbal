# Jaeger screenshot evidence

- Screenshot sources: https://www.jaegertracing.io/img/traces-ss.png and
  https://www.jaegertracing.io/img/trace-detail-ss.png
- Source page: https://www.jaegertracing.io/docs/2.20/
- Retrieved: 2026-09-14
- Local files: `ui-timelines-jaeger-traces.png`,
  `ui-timelines-jaeger-trace-detail.png`

## What the images show

1. The trace search page combines query controls, a duration/time scatter plot,
   and a list of traces. Each result carries a duration, timestamp, span count,
   and compact service-count chips.
2. The trace detail page keeps a top miniature overview strip above a large
   waterfall. Rows are indented by span nesting; bars overlap on one shared
   time axis; labels and durations remain visible at row level.
3. Error markers are attached to individual rows. The whole trace remains in
   view while a nested or exceptional row can be inspected.

## Transferable UI pattern

Gimble can use a run overview strip (current status, elapsed time, active count)
above a vertically scrollable execution waterfall. Preserve overlapping sibling
work on one wall-clock axis; put per-turn state, errors, costs, and transcript
links on the row. A selected row should drive both the overview marker and the
detail pane.

## Limits

The screenshot's row indent is span nesting and its bar width is elapsed trace
time. Gimble must keep lexical scope, execution scope, session ownership, and
supervision as distinct relations. A supervisor row is not a dependency merely
because it overlaps or appears nearby.
