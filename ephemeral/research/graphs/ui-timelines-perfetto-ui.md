# Perfetto UI source note

- Source: https://perfetto.dev/docs/visualization/perfetto-ui
- Source type: official Perfetto tracing documentation
- Retrieved: 2026-09-14
- Local evidence: this note records the relevant documented behavior; the
  source URL above is the canonical copy.

## Relevant observations

1. The UI is a browser trace viewer with a shared timeline. Several trace files
   can be merged onto one timeline.
2. Navigation is time based: pan and zoom change the visible wall-clock range.
3. A track event is selected in place and its details appear in a Current
   Selection drawer. The selected event can be centered or fitted in the
   viewport.
4. An area selection can span a start/end interval and a chosen list of tracks.
   Track checkboxes change which tracks participate in that selection.
5. Track filtering is non-destructive: filters hide rows temporarily and can be
   cleared to restore the full context.

## Transferable UI pattern

Keep a stable overview of all relevant lanes, let the operator narrow the time
window or rows, and keep the selected entity's detailed payload in a separate
drawer. A selection should be addressable from both an overview and a local
detail projection; focusing a detail should scroll the overview to the same
entity rather than creating a second identity.

## Limits for Gimble

Perfetto's horizontal coordinate is real trace time. A static workflow graph has
no honest wall-clock coordinate, so its source projection should not borrow the
timeline's visual semantics. An overlay can show a real runtime relation only
when an observed relation exists; do not draw a critical path from lexical order
or visual adjacency.
