# Ship the app

The promise: the web application serves the run interface design, built
from the components under `web/src/lib/run/`, and the old viewer is gone.

The claims are the two milestone files together, read as one list:

- `docs/design/milestone-1-storybook.md`: the Storybook components. Claims
  1 to 12 and 16 are met on this branch; build the rest only as far as the
  application needs them, a story beside each, and skip the vocabulary
  story if it costs a task of its own.
- `docs/design/milestone-2-app.md`: the application on those components.
  This is the goal; a component that the pages do not use is not worth a
  task.

Done is the definition in the workflow: the app runs, its pages look like
the specimens, the claims hold at 90 to 95 percent, and small gaps are
listed for later. Batch the work into as few tasks as the coder can carry,
and build the Go plumbing of milestone 2 claim 3 and claim 5 in the same
task as the components it serves, not after.
